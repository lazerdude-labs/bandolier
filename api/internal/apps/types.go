// Package apps provides Helm-based application lifecycle management for
// Bandolier-managed clusters: repos, catalog aggregation, and install/upgrade/
// uninstall execution.
package apps

import "time"

// CatalogEntry is one row in the operator-visible chart catalog. Curated
// entries set Source = "curated"; remote-repo entries set Source to the repo
// name (e.g. "bitnami").
type CatalogEntry struct {
	Source            string        `json:"source"`
	Name              string        `json:"name"`
	Chart             string        `json:"chart"` // "<repo>/<name>" for helm
	Description       string        `json:"description"`
	LatestVersion     string        `json:"latest_version"`
	AvailableVersions []string      `json:"available_versions"`
	System            bool          `json:"system,omitempty"`
	IngressValuePath  string        `json:"ingress_value_path,omitempty"`
	Icon              string        `json:"icon,omitempty"`
	Tag               string        `json:"tag,omitempty"`
	Type              string        `json:"type"`             // "chart" | "bundle"; default "chart" for older entries
	Charts            []BundleChart `json:"charts,omitempty"` // populated when Type == "bundle"
}

// Release is the live Helm release status as returned by `helm list -A -o json`.
type Release struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Chart      string `json:"chart"` // "<name>-<version>" form helm uses
	AppVersion string `json:"app_version"`
	Revision   int    `json:"revision"`
	Status     string `json:"status"` // deployed | pending-install | failed | etc.
	Updated    string `json:"updated"`
}

// Repo mirrors apps_repos.
type Repo struct {
	ID        int64     `json:"id"`
	ClusterID string    `json:"cluster_id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	AddedAt   time.Time `json:"added_at"`
	AddedBy   *int64    `json:"added_by,omitempty"`
}

// Install mirrors apps_installs.
type Install struct {
	ID           string     `json:"id"`
	ClusterID    string     `json:"cluster_id"`
	Chart        string     `json:"chart"`
	Version      string     `json:"version"`
	ReleaseName  string     `json:"release_name"`
	Namespace    string     `json:"namespace"`
	Hostname     *string    `json:"hostname,omitempty"`
	Operation    string     `json:"operation"`
	Status       string     `json:"status"`
	Atomic       bool       `json:"atomic"`
	ValuesHash   *string    `json:"values_hash,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	ActorID      *int64     `json:"actor_id,omitempty"`
	HostnameUnclaimed bool  `json:"hostname_unclaimed"`
}

// InstallRequest is the API + executor input shape.
type InstallRequest struct {
	Chart            string `json:"chart"` // "<repo>/<name>"
	Version          string `json:"version"`
	ReleaseName      string `json:"release_name"`
	Namespace        string `json:"namespace"`
	Hostname         string `json:"hostname,omitempty"`
	IngressValuePath string `json:"-"` // resolved server-side from catalog
	Values           string `json:"values,omitempty"` // raw YAML
	Atomic           bool   `json:"atomic"`
}

// InstallResponse is the API output for accepted requests.
type InstallResponse struct {
	InstallID string `json:"install_id"`
}

// BundleChart is one chart slot in a curated bundle. Hostname may use
// "{release}" and "{fqdn}" placeholders that are substituted at install time.
type BundleChart struct {
	Chart     string `json:"chart"`
	Version   string `json:"version"`
	Release   string `json:"release"`
	Namespace string `json:"namespace"`
	Hostname  string `json:"hostname,omitempty"`
	Required  bool   `json:"required"`
}

// BundleChartChoice is the per-chart operator selection at install time. When
// Skip == true the chart is omitted from the install (only valid for charts
// where Required == false in the catalog entry).
type BundleChartChoice struct {
	Chart     string `json:"chart"`
	Version   string `json:"version"`
	Release   string `json:"release"`
	Namespace string `json:"namespace"`
	Hostname  string `json:"hostname,omitempty"`
	Values    string `json:"values,omitempty"`
	Skip      bool   `json:"skip"`
}

// BundleInstallRequest is the API input for POST /api/clusters/{id}/apps/bundle.
type BundleInstallRequest struct {
	Bundle  string              `json:"bundle"`
	Version string              `json:"version"`
	Choices []BundleChartChoice `json:"choices"`
	Atomic  bool                `json:"atomic"`
}
