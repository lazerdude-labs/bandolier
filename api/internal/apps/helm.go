package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// HelmCLI is a thin shell-out wrapper around the helm CLI. KubeconfigPath is
// the path to a temp kubeconfig file (written from Vault per call). It
// implements the Helm interface defined in catalog.go.
type HelmCLI struct {
	Binary         string
	KubeconfigPath string
}

func (h HelmCLI) cmd(ctx context.Context, args ...string) *exec.Cmd {
	bin := h.Binary
	if bin == "" {
		bin = "helm"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+h.KubeconfigPath)
	return cmd
}

func (h HelmCLI) RepoAdd(ctx context.Context, name, url string) error {
	out, err := h.cmd(ctx, "repo", "add", "--force-update", name, url).CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm repo add %s: %w: %s", name, err, string(out))
	}
	return nil
}

func (h HelmCLI) RepoRemove(ctx context.Context, name string) error {
	out, err := h.cmd(ctx, "repo", "remove", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm repo remove %s: %w: %s", name, err, string(out))
	}
	return nil
}

func (h HelmCLI) RepoUpdate(ctx context.Context) error {
	out, err := h.cmd(ctx, "repo", "update").CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm repo update: %w: %s", err, string(out))
	}
	return nil
}

// SearchRepo returns aggregated CatalogEntry rows for the given repo. Charts
// with the same name across versions are grouped; the top-3 versions are kept
// in AvailableVersions (descending).
func (h HelmCLI) SearchRepo(ctx context.Context, repoName string) ([]CatalogEntry, error) {
	out, err := h.cmd(ctx, "search", "repo", repoName+"/", "--versions", "-o", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("helm search %s: %w", repoName, err)
	}
	entries, err := parseSearchJSON(out)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Source = repoName
	}
	return entries, nil
}

func (h HelmCLI) List(ctx context.Context) ([]Release, error) {
	out, err := h.cmd(ctx, "list", "-A", "-o", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("helm list: %w", err)
	}
	return parseListJSON(out)
}

func (h HelmCLI) Install(ctx context.Context, req InstallRequest, valuesPath string, stdout, stderr io.Writer) error {
	args := buildInstallArgs(req, valuesPath)
	cmd := h.cmd(ctx, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (h HelmCLI) Upgrade(ctx context.Context, req InstallRequest, valuesPath string, stdout, stderr io.Writer) error {
	args := buildUpgradeArgs(req, valuesPath)
	cmd := h.cmd(ctx, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (h HelmCLI) Uninstall(ctx context.Context, releaseName, namespace string, stdout, stderr io.Writer) error {
	cmd := h.cmd(ctx, "uninstall", releaseName, "--namespace", namespace, "--wait")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// KubeconfigFile exposes the temp kubeconfig path so deploy-time helpers can
// shell out to kubectl against the same cluster this Helm wrapper targets.
func (h HelmCLI) KubeconfigFile() string { return h.KubeconfigPath }

// ---------- internals ----------

// rawList mirrors the JSON shape `helm list -A -o json` emits. Revision is a
// raw JSON token because helm 3.16+ emits it as a quoted string ("1") instead
// of a bare number (1) on some clusters — the older `Revision int` field
// rejected the string shape and 500'd /apps/releases on every such cluster,
// blocking the Installed tab. json.RawMessage stores whatever the JSON layer
// produces (bare number or quoted string or anything else) so we can decide
// how to handle each shape in parseListJSON without the encoder failing
// upfront. See issue #45.
type rawList struct {
	Name       string          `json:"name"`
	Namespace  string          `json:"namespace"`
	Chart      string          `json:"chart"`
	AppVersion string          `json:"app_version"`
	Revision   json.RawMessage `json:"revision"`
	Status     string          `json:"status"`
	Updated    string          `json:"updated"`
}

func parseListJSON(b []byte) ([]Release, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var raw []rawList
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse helm list: %w", err)
	}
	out := make([]Release, 0, len(raw))
	for _, r := range raw {
		// Strip surrounding double-quotes if the raw JSON token is a
		// quoted string (helm 3.16+ shape), otherwise use the raw bytes
		// (older helm's bare-number shape). strconv.Atoi then parses
		// either form. Malformed revision = log + skip THIS release,
		// never fail the whole helm list call — reintroducing fail-all
		// on a single weird release would be the exact regression #45
		// set out to fix, just with a different trigger. The skipped
		// release stays invisible to the UI; one missing row beats
		// a blank tab.
		revStr := strings.Trim(string(r.Revision), `"`)
		rev, err := strconv.Atoi(revStr)
		if err != nil {
			slog.Warn("helm list: skipping release with unparseable revision",
				"namespace", r.Namespace, "release", r.Name,
				"revision", revStr, "err", err.Error())
			continue
		}
		out = append(out, Release{
			Name:       r.Name,
			Namespace:  r.Namespace,
			Chart:      r.Chart,
			AppVersion: r.AppVersion,
			Revision:   rev,
			Status:     r.Status,
			Updated:    r.Updated,
		})
	}
	return out, nil
}

type rawSearch struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
}

func parseSearchJSON(b []byte) ([]CatalogEntry, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var raw []rawSearch
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse helm search: %w", err)
	}
	groups := map[string]*CatalogEntry{}
	for _, r := range raw {
		key := r.Name // "<repo>/<chart>"
		short := r.Name
		if idx := strings.LastIndex(r.Name, "/"); idx >= 0 {
			short = r.Name[idx+1:]
		}
		if _, ok := groups[key]; !ok {
			groups[key] = &CatalogEntry{
				Name:        short,
				Chart:       r.Name,
				Description: r.Description,
			}
		}
		groups[key].AvailableVersions = append(groups[key].AvailableVersions, r.Version)
	}
	out := make([]CatalogEntry, 0, len(groups))
	for _, g := range groups {
		sort.Sort(sort.Reverse(sort.StringSlice(g.AvailableVersions)))
		if len(g.AvailableVersions) > 3 {
			g.AvailableVersions = g.AvailableVersions[:3]
		}
		if len(g.AvailableVersions) > 0 {
			g.LatestVersion = g.AvailableVersions[0]
		}
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func buildInstallArgs(req InstallRequest, valuesPath string) []string {
	a := []string{"install", req.ReleaseName, req.Chart,
		"--version", req.Version,
		"--namespace", req.Namespace,
		"--create-namespace",
		"--wait",
		// Override helm's 5m install/upgrade-wait default. Heavy charts
		// (Longhorn, kube-prometheus-stack, etc.) routinely exceed 5m on
		// homelab clusters with cold image caches and DaemonSet rollouts.
		// v0.1.14: default 15m, configurable via BANDOLIER_HELM_INSTALL_TIMEOUT.
		// See apps/helm_timeout.go for the rationale.
		"--timeout", helmInstallTimeoutFlag(),
	}
	if req.Atomic {
		a = append(a, "--atomic")
	}
	if req.Hostname != "" {
		path := req.IngressValuePath
		if path == "" {
			path = "ingress.hostname"
		}
		a = append(a,
			"--set", path+"="+req.Hostname,
			"--set", "ingress.enabled=true",
			"--set", "ingress.className=traefik")
	}
	if req.StorageClass != "" {
		a = append(a, "--set", "global.storageClass="+req.StorageClass)
	}
	if valuesPath != "" {
		a = append(a, "-f", valuesPath)
	}
	return a
}

func buildUpgradeArgs(req InstallRequest, valuesPath string) []string {
	a := buildInstallArgs(req, valuesPath)
	a[0] = "upgrade"
	// --install makes upgrade idempotent: creates the release if it doesn't
	// exist, upgrades if it does. Avoids confusing failures when a release was
	// uninstalled out-of-band before the operator clicked Upgrade.
	return append(a, "--install")
}
