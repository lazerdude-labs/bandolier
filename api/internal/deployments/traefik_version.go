package deployments

import (
	"log/slog"
	"os"
	"regexp"

	"github.com/lazerdude-labs/bandolier/api/internal/apps"
)

// defaultTraefikChartVersion is the pinned Traefik helm chart version
// installed during the cluster-deploy pipeline. Sourced from the apps
// package's curated catalog so the deploy executor and the UI catalog
// can't drift — a future bump to the chart version touches one place.
//
// History:
//   - v0.1.0-v0.1.8: pinned 34.2.1, which was never actually published —
//     Traefik went 34.2.0 → 34.3.0 directly. Every clean deploy died at
//     helm.install_traefik with "no chart version found for traefik-34.2.1".
//   - v0.1.9+: 34.5.0 — last patch in the 34.x series this code was
//     originally tested against. Defined in apps/catalog.go as
//     TraefikDefaultChartVersion.
const defaultTraefikChartVersion = apps.TraefikDefaultChartVersion

// traefikChartVersionRe constrains the env override to valid semver-like
// shapes. Reason: the value is interpolated into a `helm install --version`
// flag invocation; an unconstrained string could embed shell metacharacters
// or arguments. We don't shell-out via `sh -c` (helm.Install uses an
// argv slice via exec.Command), so injection is technically already
// prevented — but a strict allowlist is cheap defense in depth and
// catches typos (operator pasting "v34.5.0" with the v prefix, etc.).
var traefikChartVersionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$`)

// traefikChartVersion returns the Traefik helm chart version to install.
// Reads BANDOLIER_TRAEFIK_CHART_VERSION; falls back to the pinned default
// when unset/empty/malformed. Operators set this when:
//
//   - The default version gets yanked from the upstream index (history
//     repeats; the 34.2.1 typo of v0.1.0-v0.1.8 was the first instance).
//   - They want a newer version with a fix or feature.
//   - They're on a fork or internal mirror that publishes different tags.
//
// Malformed values (non-semver, embedded shell metachars) fall through to
// the pinned default rather than surfacing as a helm install error mid-
// deploy. Pre-release semver tags (e.g. "39.1.0-ea.1") are accepted.
func traefikChartVersion() string {
	v := os.Getenv("BANDOLIER_TRAEFIK_CHART_VERSION")
	if v == "" {
		return defaultTraefikChartVersion
	}
	if !traefikChartVersionRe.MatchString(v) {
		// Surface the rejection so operators can tell their override was
		// ignored. Silent fall-through (the previous behavior) made a
		// typo invisible — operator would see a deploy succeed against
		// the wrong version and have no idea why their pin didn't take.
		slog.Warn("BANDOLIER_TRAEFIK_CHART_VERSION rejected, falling back to default",
			"value", v, "default", defaultTraefikChartVersion,
			"allowlist", "MAJOR.MINOR.PATCH[-prerelease]")
		return defaultTraefikChartVersion
	}
	return v
}
