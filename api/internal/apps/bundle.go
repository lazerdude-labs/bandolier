package apps

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lazerdude-labs/bandolier/api/internal/audit"
)

// chartOutcome captures per-chart status for the bundle terminal audit row.
// Outcome values: "succeeded" | "failed" | "skipped" | "rolled_back" |
// "not_attempted" (chart slot reached after a prior failure aborted the bundle).
type chartOutcome struct {
	Chart   string `json:"chart"`
	Version string `json:"version"`
	Release string `json:"release"`
	Outcome string `json:"outcome"`
}

// InstallBundle starts an async multi-chart bundle install. Returns the parent
// install id; subscribe to /ws/apps/installs/{id}/logs to follow progress. The
// per-chart helm operations stream into the same id so the UI can render them
// as a single timeline.
//
// Failure semantics: on any chart's failure, previously-installed charts are
// uninstalled in reverse order. The parent install row finishes "failed" and
// the audit terminal entry records each chart's outcome.
func (e *Executor) InstallBundle(ctx context.Context, clusterID string, req BundleInstallRequest, fqdn string) (string, error) {
	if !e.Mutex.TryLock(clusterID) {
		return "", ErrLocked
	}

	id := newID()
	logFile, _, err := e.openLog(id)
	if err != nil {
		e.Mutex.Unlock(clusterID)
		return "", fmt.Errorf("open log: %w", err)
	}

	// Parent install row. Chart prefixed with "bundle/" so the installed/list
	// view can distinguish a bundle parent from individual chart installs.
	row := &Install{
		ID:          id,
		ClusterID:   clusterID,
		Chart:       "bundle/" + req.Bundle,
		Version:     req.Version,
		ReleaseName: req.Bundle,
		Namespace:   "-",
		Operation:   "install",
		Status:      "running",
		Atomic:      req.Atomic,
	}
	if err := e.Apps.CreateInstall(ctx, row); err != nil {
		_ = logFile.Close()
		e.Mutex.Unlock(clusterID)
		return "", fmt.Errorf("create install row: %w", err)
	}

	actorID := actorIDFromCtx(ctx)
	_, _ = audit.Write(ctx, e.Core, audit.Entry{
		ActorID: actorID,
		Action:  string(audit.ActionAppBundleInstall),
		Target:  clusterID,
		Outcome: audit.OutcomeStarted,
		Details: map[string]any{
			"install_id":     id,
			"bundle":         req.Bundle,
			"bundle_version": req.Version,
			"chart_count":    len(req.Choices),
		},
	})

	runCtx, cancelRun := context.WithCancel(context.Background())
	e.register(id, cancelRun)
	go e.runBundleInstall(runCtx, id, clusterID, req, fqdn, logFile, actorID)
	return id, nil
}

// runBundleInstall is the goroutine body for InstallBundle. Sequentially
// installs each non-skipped chart; on any failure rolls back installed charts
// in reverse order, then writes a terminal audit row and closes the Hub stream.
func (e *Executor) runBundleInstall(
	ctx context.Context,
	id, clusterID string,
	req BundleInstallRequest,
	fqdn string,
	logFile *os.File,
	actorID int64,
) {
	defer func() { _ = logFile.Close() }()
	defer e.deregister(id)
	defer e.Mutex.Unlock(clusterID)

	tee := &teeWriter{file: logFile, hub: e.Hub, streamID: id}

	// Pre-seed outcomes so a failure path can mark untouched charts as
	// not_attempted in the terminal audit row.
	outcomes := make([]chartOutcome, len(req.Choices))
	for i, c := range req.Choices {
		out := "not_attempted"
		if c.Skip {
			out = "skipped"
		}
		outcomes[i] = chartOutcome{
			Chart:   c.Chart,
			Version: c.Version,
			Release: c.Release,
			Outcome: out,
		}
	}

	finish := func(ok bool, errMsg string) {
		bg := context.Background()
		status, outcome := finishStatus(ctx.Err(), !ok)
		_ = e.Apps.FinishInstall(bg, id, status, errMsg)
		details := map[string]any{
			"install_id":     id,
			"bundle":         req.Bundle,
			"bundle_version": req.Version,
			"charts":         outcomes,
		}
		if errMsg != "" {
			details["error"] = errMsg
		}
		_, _ = audit.Write(bg, e.Core, audit.Entry{
			ActorID: actorID,
			Action:  string(audit.ActionAppBundleInstall),
			Target:  clusterID,
			Outcome: outcome,
			Details: details,
		})
		exit := 0
		if !ok {
			exit = 1
		}
		e.Hub.PublishStepEnd(id, "bundle.install", exit)
		e.Hub.PublishComplete(id, status, errMsg)
	}

	helm, cleanup, err := e.Helm.For(ctx, clusterID)
	if err != nil {
		_, _ = fmt.Fprintf(tee, "helm factory error: %s\n", err.Error())
		finish(false, fmt.Sprintf("helm unavailable: %s", err.Error()))
		return
	}
	defer cleanup()

	e.Hub.PublishStepStart(id, "bundle.install")

	// Track the installed-and-succeeded charts for reverse-order rollback on
	// any subsequent failure. We snapshot the choice (with substituted
	// hostname) so rollback can target the right release/namespace.
	var installed []BundleChartChoice

	for i, choice := range req.Choices {
		if choice.Skip {
			_, _ = fmt.Fprintf(tee, "bundle: skip %s (release=%s)\n", choice.Chart, choice.Release)
			continue
		}

		// Hostname template substitution — apply before any helm work so
		// the values file path (if hostname is rendered into custom values
		// downstream) and audit details show the resolved value.
		choice.Hostname = substituteHostnameTemplate(choice.Hostname, choice.Release, fqdn)

		_, _ = fmt.Fprintf(tee, "bundle: install %s (release=%s, ns=%s)\n",
			choice.Chart, choice.Release, choice.Namespace)

		// Re-add the chart's source repo before install. Same dance as the
		// single-chart executor — fresh kubeconfig means fresh helm cache.
		if repoName, ok := splitChartRepo(choice.Chart); ok {
			if repoURL := e.lookupRepoURL(ctx, clusterID, repoName); repoURL != "" {
				if err := helm.RepoAdd(ctx, repoName, repoURL); err != nil {
					_, _ = fmt.Fprintf(tee, "helm repo add %s: %s\n", repoName, err.Error())
				}
				_ = helm.RepoUpdate(ctx)
			}
		}

		// Optional values file. One per chart — written under LogRoot with
		// the bundle install id + chart index for uniqueness; removed at the
		// end of this iteration regardless of outcome.
		var valuesPath string
		if choice.Values != "" {
			vp := filepath.Join(e.LogRoot, fmt.Sprintf("%s.bundle-%d.values.yaml", id, i))
			if err := os.WriteFile(vp, []byte(choice.Values), 0o600); err != nil {
				_, _ = fmt.Fprintf(tee, "write values file: %s\n", err.Error())
				outcomes[i].Outcome = "failed"
				rollbackInstalled(ctx, helm, installed, tee)
				markRolledBack(outcomes, installed)
				finish(false, fmt.Sprintf("write values: %s", err.Error()))
				return
			}
			valuesPath = vp
		}

		instReq := InstallRequest{
			Chart:       choice.Chart,
			Version:     choice.Version,
			ReleaseName: choice.Release,
			Namespace:   choice.Namespace,
			Hostname:    choice.Hostname,
			Values:      choice.Values,
			Atomic:      req.Atomic,
		}

		opErr := helm.Install(ctx, instReq, valuesPath, tee, tee)
		if valuesPath != "" {
			_ = os.Remove(valuesPath)
		}

		if opErr != nil {
			_, _ = fmt.Fprintf(tee, "bundle: chart %s failed: %s\n", choice.Chart, opErr.Error())
			outcomes[i].Outcome = "failed"
			rollbackInstalled(ctx, helm, installed, tee)
			markRolledBack(outcomes, installed)
			finish(false, opErr.Error())
			return
		}

		outcomes[i].Outcome = "succeeded"
		installed = append(installed, choice)
	}

	finish(true, "")
}

// substituteHostnameTemplate swaps "{release}" and "{fqdn}" placeholders in
// the bundle catalog's hostname template. Returns input unchanged when no
// placeholders are present (or when input is empty).
func substituteHostnameTemplate(tmpl, release, fqdn string) string {
	if tmpl == "" {
		return ""
	}
	out := strings.ReplaceAll(tmpl, "{release}", release)
	out = strings.ReplaceAll(out, "{fqdn}", fqdn)
	return out
}

// rollbackInstalled uninstalls already-installed bundle charts in reverse
// order. Failures are logged but do not abort the rollback — best effort.
func rollbackInstalled(ctx context.Context, helm Helm, installed []BundleChartChoice, tee io.Writer) {
	if len(installed) == 0 {
		return
	}
	_, _ = fmt.Fprintf(tee, "bundle: rolling back %d previously-installed chart(s) in reverse order\n", len(installed))
	for i := len(installed) - 1; i >= 0; i-- {
		c := installed[i]
		_, _ = fmt.Fprintf(tee, "bundle: rollback uninstall %s (release=%s, ns=%s)\n", c.Chart, c.Release, c.Namespace)
		if err := helm.Uninstall(ctx, c.Release, c.Namespace, tee, tee); err != nil {
			_, _ = fmt.Fprintf(tee, "bundle: rollback uninstall %s failed: %s (continuing)\n", c.Release, err.Error())
		}
	}
}

// markRolledBack updates the outcomes slice — any chart whose release matches
// an entry in installed (i.e. it succeeded earlier) is now marked rolled_back.
func markRolledBack(outcomes []chartOutcome, installed []BundleChartChoice) {
	rb := make(map[string]struct{}, len(installed))
	for _, c := range installed {
		rb[c.Release] = struct{}{}
	}
	for i := range outcomes {
		if _, ok := rb[outcomes[i].Release]; ok && outcomes[i].Outcome == "succeeded" {
			outcomes[i].Outcome = "rolled_back"
		}
	}
}
