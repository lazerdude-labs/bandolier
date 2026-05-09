package apps

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/auth"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

// EventPublisher is the narrow surface the executor uses to push stream events
// to subscribers. Satisfied by an adapter around *deployments.Hub wired in the
// httpserver package. Defined here as an interface so apps doesn't import
// deployments — that would create a cycle (apps ← clusters ← deployments).
//
// Each method takes the streamID (== install id) and the bits the WS layer
// needs to render. Implementations are expected to publish to the same Hub
// the cluster deploy executor uses, so /ws/apps/installs/{id}/logs and
// /ws/deployments/{id}/logs share a backend.
type EventPublisher interface {
	PublishLog(streamID, text string)
	PublishStepStart(streamID, step string)
	PublishStepEnd(streamID, step string, exit int)
	PublishComplete(streamID, status, text string)
}

// ClusterLocker mirrors *deployments.ClusterMutex. Same import-cycle rationale
// as EventPublisher — apps cannot import deployments.
type ClusterLocker interface {
	TryLock(clusterID string) bool
	Unlock(clusterID string)
}

// ErrLocked is returned when another deployment-like operation already holds
// the per-cluster mutex (deploy/destroy/install all share the same lock so a
// helm install cannot race with a terraform apply on the same cluster).
var ErrLocked = errors.New("apps: cluster busy")

// ErrSystemRel is returned when an operator tries to uninstall a system
// release (currently just "traefik" — Bandolier owns its lifecycle and the
// dedicated Settings flow must be used to swap or remove it).
var ErrSystemRel = errors.New("apps: cannot uninstall system release")

// ErrNotCancellable is returned by Executor.Cancel when the named install id
// has no live cancel func registered (already finished or never started).
var ErrNotCancellable = errors.New("apps: operation not cancellable")

// Executor runs Helm install/upgrade/uninstall operations asynchronously,
// streaming output to the shared event Hub keyed by install id. The same Hub
// the cluster deploy executor uses — unifying the WS log surface.
type Executor struct {
	Apps    *Store
	Core    *store.Store
	Catalog *Catalog
	Hub     EventPublisher
	Mutex   ClusterLocker
	Helm    HelmFactory
	LogRoot string
	// Vault is the bundle install handler's source of cluster network
	// metadata (FQDN for hostname template substitution). Wired in main.go
	// in Phase 4G/Task 23.
	Vault *vault.Client

	// cancelMap tracks the cancel func for each in-flight install/upgrade/
	// uninstall/bundle so the HTTP cancel handler can signal the goroutine
	// to stop. Mirrors deployments.Executor; entries are added by spawn
	// sites and removed by the goroutine's defer.
	cancelMu  sync.Mutex
	cancelMap map[string]context.CancelFunc
}

// register stores the cancel func for a running install operation.
func (e *Executor) register(id string, cancel context.CancelFunc) {
	e.cancelMu.Lock()
	defer e.cancelMu.Unlock()
	if e.cancelMap == nil {
		e.cancelMap = make(map[string]context.CancelFunc)
	}
	e.cancelMap[id] = cancel
}

// deregister removes the cancel func once the goroutine exits.
func (e *Executor) deregister(id string) {
	e.cancelMu.Lock()
	defer e.cancelMu.Unlock()
	delete(e.cancelMap, id)
}

// Cancel signals the running goroutine for the named install id. Returns
// ErrNotCancellable when no live entry exists. The terminal status write
// happens inside the goroutine when its ctx is cancelled.
func (e *Executor) Cancel(_ context.Context, id string) error {
	e.cancelMu.Lock()
	cancel, ok := e.cancelMap[id]
	e.cancelMu.Unlock()
	if !ok {
		return ErrNotCancellable
	}
	cancel()
	return nil
}

// finishStatus mirrors the deployments package helper. apps cannot import
// deployments (cycle), so the small DRY violation is accepted per the Phase 7
// plan.
func finishStatus(ctxErr error, naturalFailure bool) (status string, outcome audit.Outcome) {
	if errors.Is(ctxErr, context.Canceled) {
		return "cancelled", audit.OutcomeCancelled
	}
	if naturalFailure {
		return "failed", audit.OutcomeFailed
	}
	return "succeeded", audit.OutcomeSucceeded
}

// Install starts an async helm install for the given cluster. Returns the
// install id on success; the caller subscribes to /ws/apps/installs/{id}/logs
// to follow progress.
func (e *Executor) Install(ctx context.Context, clusterID string, req InstallRequest) (string, error) {
	return e.run(ctx, clusterID, req, "install", audit.ActionAppInstall)
}

// Upgrade starts an async helm upgrade for the given cluster. Same shape as
// Install — release_name + chart + version are required and uniquely identify
// the target.
func (e *Executor) Upgrade(ctx context.Context, clusterID string, req InstallRequest) (string, error) {
	return e.run(ctx, clusterID, req, "upgrade", audit.ActionAppUpgrade)
}

// Uninstall starts an async helm uninstall for a release. force=true bypasses
// the system-release guard — the UI gates this behind a type-the-release-name
// confirmation in InstalledTab so an operator who really wants to remove
// Traefik (or another Bandolier-owned component) can, while the default
// install/upgrade flow still 400s on system releases without the explicit ack.
func (e *Executor) Uninstall(ctx context.Context, clusterID, releaseName, namespace string, force bool) (string, error) {
	if isSystem(releaseName) && !force {
		return "", ErrSystemRel
	}
	if !e.Mutex.TryLock(clusterID) {
		return "", ErrLocked
	}

	id := newID()
	logFile, _, err := e.openLog(id)
	if err != nil {
		e.Mutex.Unlock(clusterID)
		return "", fmt.Errorf("open log: %w", err)
	}

	row := &Install{
		ID:          id,
		ClusterID:   clusterID,
		Chart:       "",
		Version:     "",
		ReleaseName: releaseName,
		Namespace:   namespace,
		Operation:   "uninstall",
		Status:      "running",
	}
	if err := e.Apps.CreateInstall(ctx, row); err != nil {
		_ = logFile.Close()
		e.Mutex.Unlock(clusterID)
		return "", fmt.Errorf("create install row: %w", err)
	}

	actorID := actorIDFromCtx(ctx)
	_, _ = audit.Write(ctx, e.Core, audit.Entry{
		ActorID: actorID,
		Action:  string(audit.ActionAppUninstall),
		Target:  clusterID,
		Outcome: audit.OutcomeStarted,
		Details: map[string]any{"install_id": id, "release_name": releaseName, "namespace": namespace},
	})

	runCtx, cancelRun := context.WithCancel(context.Background())
	e.register(id, cancelRun)
	go e.runUninstall(runCtx, id, clusterID, releaseName, namespace, logFile, actorID)
	return id, nil
}

// run is the shared front-half for install + upgrade. It validates the lock,
// opens the log file, creates the apps_installs row, writes the started audit,
// and spawns the goroutine that calls helm.
func (e *Executor) run(ctx context.Context, clusterID string, req InstallRequest, op string, action audit.Action) (string, error) {
	// Resolve ingress value path from the curated catalog if hostname set and
	// the caller did not specify one. Lets operators install Traefik with
	// "ingress.hostname" auto-resolved without UI knowledge.
	if req.Hostname != "" && req.IngressValuePath == "" && e.Catalog != nil {
		if entry, ok := e.Catalog.FindCurated(req.ReleaseName); ok && entry.IngressValuePath != "" {
			req.IngressValuePath = entry.IngressValuePath
		}
	}

	if !e.Mutex.TryLock(clusterID) {
		return "", ErrLocked
	}

	id := newID()
	logFile, _, err := e.openLog(id)
	if err != nil {
		e.Mutex.Unlock(clusterID)
		return "", fmt.Errorf("open log: %w", err)
	}

	hash := hashValues(req.Values)
	var hashPtr *string
	if hash != "" {
		hashPtr = &hash
	}
	var hostPtr *string
	if req.Hostname != "" {
		h := req.Hostname
		hostPtr = &h
	}
	row := &Install{
		ID:          id,
		ClusterID:   clusterID,
		Chart:       req.Chart,
		Version:     req.Version,
		ReleaseName: req.ReleaseName,
		Namespace:   req.Namespace,
		Hostname:    hostPtr,
		Operation:   op,
		Status:      "running",
		Atomic:      req.Atomic,
		ValuesHash:  hashPtr,
	}
	if err := e.Apps.CreateInstall(ctx, row); err != nil {
		_ = logFile.Close()
		e.Mutex.Unlock(clusterID)
		return "", fmt.Errorf("create install row: %w", err)
	}

	actorID := actorIDFromCtx(ctx)
	_, _ = audit.Write(ctx, e.Core, audit.Entry{
		ActorID: actorID,
		Action:  string(action),
		Target:  clusterID,
		Outcome: audit.OutcomeStarted,
		Details: map[string]any{
			"install_id":   id,
			"chart":        req.Chart,
			"version":      req.Version,
			"release_name": req.ReleaseName,
			"namespace":    req.Namespace,
		},
	})

	runCtx, cancelRun := context.WithCancel(context.Background())
	e.register(id, cancelRun)
	go e.runInstallOrUpgrade(runCtx, id, clusterID, req, op, action, logFile, actorID)
	return id, nil
}

// runInstallOrUpgrade is the goroutine body for install + upgrade. It writes
// the values file (if any), gets a Helm instance for the cluster, executes the
// helm operation, and closes the loop with audit + hub events.
func (e *Executor) runInstallOrUpgrade(
	ctx context.Context,
	id, clusterID string,
	req InstallRequest,
	op string,
	action audit.Action,
	logFile *os.File,
	actorID int64,
) {
	defer func() { _ = logFile.Close() }()
	defer e.deregister(id)
	defer e.Mutex.Unlock(clusterID)

	tee := &teeWriter{file: logFile, hub: e.Hub, streamID: id}

	finish := func(opErr error) {
		// ctx may already be cancelled (operator clicked cancel). Use a
		// fresh background ctx for terminal writes; finishStatus picks the
		// status/outcome based on whether ctx was cancelled vs. natural
		// failure.
		bg := context.Background()
		status, outcome := finishStatus(ctx.Err(), opErr != nil)
		errMsg := ""
		if opErr != nil {
			errMsg = opErr.Error()
		}
		_ = e.Apps.FinishInstall(bg, id, status, errMsg)
		details := map[string]any{
			"install_id":   id,
			"chart":        req.Chart,
			"version":      req.Version,
			"release_name": req.ReleaseName,
			"namespace":    req.Namespace,
		}
		if errMsg != "" {
			details["error"] = errMsg
		}
		_, _ = audit.Write(bg, e.Core, audit.Entry{
			ActorID: actorID,
			Action:  string(action),
			Target:  clusterID,
			Outcome: outcome,
			Details: details,
		})
		e.Hub.PublishStepEnd(id, "helm."+op, boolToExit(opErr))
		e.Hub.PublishComplete(id, status, errMsg)
	}

	helm, cleanup, err := e.Helm.For(ctx, clusterID)
	if err != nil {
		_, _ = fmt.Fprintf(tee, "helm factory error: %s\n", err.Error())
		finish(fmt.Errorf("helm unavailable: %w", err))
		return
	}
	defer cleanup()

	// Each Helm invocation runs against a freshly-fetched kubeconfig with no
	// repo cache, so we must re-add the chart's source repo before install.
	// First check operator-added repos, then fall back to the factory-default
	// curated map for clusters created pre-Phase-3 or with a wiped repo list.
	if repoName, ok := splitChartRepo(req.Chart); ok {
		if repoURL := e.lookupRepoURL(ctx, clusterID, repoName); repoURL != "" {
			if err := helm.RepoAdd(ctx, repoName, repoURL); err != nil {
				_, _ = fmt.Fprintf(tee, "helm repo add %s: %s\n", repoName, err.Error())
			}
			_ = helm.RepoUpdate(ctx)
		}
	}

	// Optional values file. Written under the log root so the path is
	// predictable; removed after the helm run regardless of outcome.
	var valuesPath string
	if req.Values != "" {
		vp := filepath.Join(e.LogRoot, id+".values.yaml")
		if err := os.WriteFile(vp, []byte(req.Values), 0o600); err != nil {
			_, _ = fmt.Fprintf(tee, "write values file: %s\n", err.Error())
			finish(fmt.Errorf("write values: %w", err))
			return
		}
		valuesPath = vp
		defer func() { _ = os.Remove(vp) }()
	}

	e.Hub.PublishStepStart(id, "helm."+op)

	var opErr error
	switch op {
	case "install":
		opErr = helm.Install(ctx, req, valuesPath, tee, tee)
	case "upgrade":
		opErr = helm.Upgrade(ctx, req, valuesPath, tee, tee)
	default:
		opErr = fmt.Errorf("unknown op %q", op)
	}

	// Phase 4: ingress probe. Best-effort — failure to probe doesn't change
	// the install outcome, just leaves hostname_unclaimed at default false.
	if opErr == nil && req.Hostname != "" {
		claimed, perr := probeHostnameClaimed(ctx, helm.KubeconfigFile(), req.Namespace, req.Hostname)
		if perr == nil && !claimed {
			_ = e.Apps.MarkHostnameUnclaimed(ctx, id, true)
			_, _ = fmt.Fprintf(tee, "warning: hostname %q not claimed by any Ingress or IngressRoute in namespace %q. Chart may use a different value path. Open the install modal's Advanced ▸ Hostname value path to override.\n", req.Hostname, req.Namespace)
		}
	}

	finish(opErr)
}

// runUninstall is the goroutine body for uninstall. Same shape as
// runInstallOrUpgrade but no values file and no chart/version metadata.
func (e *Executor) runUninstall(
	ctx context.Context,
	id, clusterID, releaseName, namespace string,
	logFile *os.File,
	actorID int64,
) {
	defer func() { _ = logFile.Close() }()
	defer e.deregister(id)
	defer e.Mutex.Unlock(clusterID)

	tee := &teeWriter{file: logFile, hub: e.Hub, streamID: id}

	finish := func(opErr error) {
		bg := context.Background()
		status, outcome := finishStatus(ctx.Err(), opErr != nil)
		errMsg := ""
		if opErr != nil {
			errMsg = opErr.Error()
		}
		_ = e.Apps.FinishInstall(bg, id, status, errMsg)
		details := map[string]any{
			"install_id":   id,
			"release_name": releaseName,
			"namespace":    namespace,
		}
		if errMsg != "" {
			details["error"] = errMsg
		}
		_, _ = audit.Write(bg, e.Core, audit.Entry{
			ActorID: actorID,
			Action:  string(audit.ActionAppUninstall),
			Target:  clusterID,
			Outcome: outcome,
			Details: details,
		})
		e.Hub.PublishStepEnd(id, "helm.uninstall", boolToExit(opErr))
		e.Hub.PublishComplete(id, status, errMsg)
	}

	helm, cleanup, err := e.Helm.For(ctx, clusterID)
	if err != nil {
		_, _ = fmt.Fprintf(tee, "helm factory error: %s\n", err.Error())
		finish(fmt.Errorf("helm unavailable: %w", err))
		return
	}
	defer cleanup()

	e.Hub.PublishStepStart(id, "helm.uninstall")
	finish(helm.Uninstall(ctx, releaseName, namespace, tee, tee))
}

// openLog creates the per-install log file under LogRoot and returns the open
// handle plus path. Caller is responsible for closing the file.
func (e *Executor) openLog(id string) (*os.File, string, error) {
	if err := os.MkdirAll(e.LogRoot, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(e.LogRoot, id+".log")
	f, err := os.Create(path)
	if err != nil {
		return nil, "", err
	}
	return f, path, nil
}

// ---------- helpers ----------

// teeWriter mirrors deployments.teeWriter — writes go to the on-disk log file
// AND publish to the Hub so live WS subscribers see them in real time.
type teeWriter struct {
	file     *os.File
	hub      EventPublisher
	streamID string
}

func (t *teeWriter) Write(p []byte) (int, error) {
	t.hub.PublishLog(t.streamID, string(p))
	return t.file.Write(p)
}

// actorIDFromCtx pulls the user id from the request context. Returns 0 when
// missing (eg. install kicked off by an internal pathway) — audit row gets
// actor_id=NULL.
func actorIDFromCtx(ctx context.Context) int64 {
	if uid, ok := auth.UserIDFromContext(ctx); ok {
		return uid
	}
	return 0
}

// newID returns a hex-encoded 16-byte random id (32 chars) used for both
// apps_installs.id and the Hub stream key.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// hashValues returns a hex SHA256 of the raw values YAML, or empty string for
// empty input. Stored on apps_installs for audit/repro without persisting the
// (potentially-secret) values themselves.
func hashValues(yaml string) string {
	if yaml == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(yaml))
	return hex.EncodeToString(sum[:])
}

// isSystem reports whether the named release is a Bandolier-owned system
// component. v1: just the ingress controller.
func isSystem(releaseName string) bool {
	return releaseName == "traefik"
}

// factoryDefaultRepos mirrors the seed loop in clusters.Create — used as the
// fallback when an operator-added repo isn't found for a given chart prefix
// (e.g. clusters created pre-Phase-3 with no apps_repos rows yet).
var factoryDefaultRepos = map[string]string{
	"traefik":              "https://traefik.github.io/charts",
	"bitnami":              "https://charts.bitnami.com/bitnami",
	"grafana":              "https://grafana.github.io/helm-charts",
	"prometheus-community": "https://prometheus-community.github.io/helm-charts",
}

// splitChartRepo separates "<repo>/<chart>" into its parts. Returns ok=false
// for unprefixed charts (e.g. local paths or OCI refs we don't handle yet).
func splitChartRepo(chart string) (repo string, ok bool) {
	idx := strings.Index(chart, "/")
	if idx <= 0 {
		return "", false
	}
	return chart[:idx], true
}

// lookupRepoURL returns the URL for a repo name on a cluster: operator-added
// repos win, factory-default curated repos are the fallback. Empty string when
// neither has it (caller skips RepoAdd in that case).
func (e *Executor) lookupRepoURL(ctx context.Context, clusterID, repoName string) string {
	if repos, err := e.Apps.ListRepos(ctx, clusterID); err == nil {
		for _, r := range repos {
			if r.Name == repoName {
				return r.URL
			}
		}
	}
	return factoryDefaultRepos[repoName]
}

// boolToExit maps an error to a helm-style exit code for step_end events.
// 0 on success, 1 on failure — Hub consumers display this as a status badge.
func boolToExit(err error) int {
	if err != nil {
		return 1
	}
	return 0
}

