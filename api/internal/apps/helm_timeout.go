package apps

import (
	"log/slog"
	"os"
	"regexp"
	"time"
)

// defaultHelmInstallTimeout is the value passed to `helm install --timeout`
// and `helm upgrade --timeout` when the operator hasn't overridden it.
// 15m is sized for the heaviest chart in the curated catalog —
// longhorn/longhorn first-install pulls ~1-2 GB of images (manager,
// engine-image, instance-manager, csi sidecars) AND rolls out a DaemonSet
// across all cluster nodes. On a cold-cache homelab cluster that
// routinely takes 8-12 minutes. Helm's own default is 5m, which is too
// short for that workload and produces the
// "context deadline exceeded ... atomic rollback also failed" error
// class that operators saw in v0.1.13.
const defaultHelmInstallTimeout = 15 * time.Minute

// defaultHelmInstallTimeoutFlag is the string form of defaultHelmInstallTimeout
// that helm CLI accepts on the --timeout flag. Kept in sync with the
// duration above. Using a named constant rather than calling
// defaultHelmInstallTimeout.String() avoids the "15m0s" representation
// time.Duration.String() emits — helm accepts both shapes but "15m" is
// what operators write in env-var overrides, so the default flag value
// matches operator-visible conventions in audit logs / process listings.
const defaultHelmInstallTimeoutFlag = "15m"

// helmInstallTimeoutRe constrains the env override to valid Go duration
// strings. Reason: the value is interpolated into a `helm install
// --timeout <X>` flag invocation. Helm CLI uses exec.Command (argv slice,
// not sh -c) so shell-metachar injection is technically already
// prevented — but a strict allowlist is cheap defense in depth and
// catches typos (operator pasting "15 m" with the space, "15min" without
// the standard suffix, etc.). The pattern matches "<digits><unit>"
// where unit is one of s/m/h, which is what Helm expects.
//
// Examples that match:  "5m", "15m", "1h", "300s", "30m"
// Examples that don't:  "5", "15min", "1.5h", "15 m", "5m; rm -rf /"
var helmInstallTimeoutRe = regexp.MustCompile(`^[0-9]+(s|m|h)$`)

// helmInstallTimeout returns the duration to pass to `helm install
// --timeout`. Reads BANDOLIER_HELM_INSTALL_TIMEOUT; falls back to the
// pinned default when unset/empty/malformed.
//
// Malformed values emit a slog.Warn line so the operator can tell their
// override was ignored — silent fallback would hide typos and leave them
// wondering why the install still times out at 5m.
func helmInstallTimeout() time.Duration {
	v := os.Getenv("BANDOLIER_HELM_INSTALL_TIMEOUT")
	if v == "" {
		return defaultHelmInstallTimeout
	}
	if !helmInstallTimeoutRe.MatchString(v) {
		slog.Warn("BANDOLIER_HELM_INSTALL_TIMEOUT rejected, falling back to default",
			"value", v, "default", defaultHelmInstallTimeout.String(),
			"allowlist", "<digits>(s|m|h), e.g. 15m or 1h or 300s")
		return defaultHelmInstallTimeout
	}
	// Parse via time.ParseDuration to catch the rare edge case where the
	// regex accepts something the Go parser rejects (e.g. an absurdly long
	// integer that overflows). Should never trip in practice.
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("BANDOLIER_HELM_INSTALL_TIMEOUT parse failed, falling back to default",
			"value", v, "default", defaultHelmInstallTimeout.String(), "err", err.Error())
		return defaultHelmInstallTimeout
	}
	if d <= 0 {
		// "0s" / "0m" / "0h" pass the regex and parse cleanly, but helm
		// interprets --timeout 0s as "no timeout" — an actually-stuck
		// install would hang indefinitely instead of failing and rolling
		// back. Treat zero/negative as a misconfiguration and fall back
		// to the default with a warning. Operators who really want to
		// disable the timeout can patch the source; we won't ship a
		// foot-gun env var that disables the rollback safety net.
		slog.Warn("BANDOLIER_HELM_INSTALL_TIMEOUT must be positive, falling back to default",
			"value", v, "default", defaultHelmInstallTimeout.String())
		return defaultHelmInstallTimeout
	}
	return d
}

// helmInstallTimeoutFlag returns the timeout as the string Helm CLI
// expects: "15m" / "1h" / "300s". Splits the duration into its largest
// whole unit so the value passed to helm matches what the operator
// configured (avoids "900000000000ns" style noise from
// time.Duration.String()).
func helmInstallTimeoutFlag() string {
	v := os.Getenv("BANDOLIER_HELM_INSTALL_TIMEOUT")
	if v == "" || !helmInstallTimeoutRe.MatchString(v) {
		return defaultHelmInstallTimeoutFlag
	}
	// Mirror the zero-guard from helmInstallTimeout(): "0s"/"0m"/"0h"
	// passes the regex but means "no timeout" in helm. Fall back to the
	// default flag string. Parse failures (the time.ParseDuration check
	// helmInstallTimeout() does) are vanishingly unlikely here given the
	// regex shape, but if they ever happen the flag emitted is the
	// operator-visible default, which is the right outcome.
	if d, err := time.ParseDuration(v); err != nil || d <= 0 {
		return defaultHelmInstallTimeoutFlag
	}
	return v
}
