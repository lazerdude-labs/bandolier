package apps

import (
	"strings"
	"testing"
)

func TestParseListJSON(t *testing.T) {
	in := `[{"name":"traefik","namespace":"kube-system","chart":"traefik-34.2.1","app_version":"v3.2.1","revision":1,"status":"deployed","updated":"2026-05-01T12:00:00Z"}]`
	out, err := parseListJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Name != "traefik" || out[0].Namespace != "kube-system" {
		t.Fatalf("got %+v", out)
	}
	if out[0].Revision != 1 {
		t.Fatalf("revision: got %d want 1", out[0].Revision)
	}
}

// TestParseListJSONRevisionAsString pins the v0.1.16 fix for issue #45.
// helm CLI 3.16+ emits `revision` as a quoted JSON string ("1") instead of a
// bare integer (1). The old `Revision int` field rejected the string and
// failed every /apps/releases call on a cluster with any helm release,
// silently breaking the Installed tab. json.Number accepts both shapes.
func TestParseListJSONRevisionAsString(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"int revision (old helm)", `[{"name":"r","namespace":"n","chart":"c-1","app_version":"v1","revision":2,"status":"deployed","updated":"t"}]`, 2},
		{"string revision (helm 3.16+)", `[{"name":"r","namespace":"n","chart":"c-1","app_version":"v1","revision":"3","status":"deployed","updated":"t"}]`, 3},
		{"multi-digit string revision", `[{"name":"r","namespace":"n","chart":"c-1","app_version":"v1","revision":"42","status":"deployed","updated":"t"}]`, 42},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := parseListJSON([]byte(c.in))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out) != 1 {
				t.Fatalf("len: got %d want 1", len(out))
			}
			if out[0].Revision != c.want {
				t.Errorf("revision: got %d want %d", out[0].Revision, c.want)
			}
		})
	}
}

// TestParseListJSONRevisionMalformed verifies that a non-numeric revision
// surfaces as a parse error rather than silently zeroing. Pins behavior so
// future "revision: latest" or similar helm output changes get a loud
// signal rather than a confused UI showing revision 0.
func TestParseListJSONRevisionMalformed(t *testing.T) {
	in := `[{"name":"r","namespace":"n","chart":"c-1","app_version":"v1","revision":"latest","status":"deployed","updated":"t"}]`
	_, err := parseListJSON([]byte(in))
	if err == nil {
		t.Fatal("expected parse error on non-numeric revision, got nil")
	}
}

func TestParseSearchJSON(t *testing.T) {
	in := `[{"name":"bitnami/grafana","version":"8.7.0","app_version":"10.4.2","description":"Grafana"},
	         {"name":"bitnami/grafana","version":"8.6.4","app_version":"10.4.1","description":"Grafana"},
	         {"name":"bitnami/grafana","version":"8.6.3","app_version":"10.4.0","description":"Grafana"}]`
	out, err := parseSearchJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 grouped chart, got %d", len(out))
	}
	g := out[0]
	if g.Name != "grafana" || g.Chart != "bitnami/grafana" || g.LatestVersion != "8.7.0" || len(g.AvailableVersions) != 3 {
		t.Fatalf("grafana mismatch: %+v", g)
	}
}

func TestBuildInstallArgs(t *testing.T) {
	args := buildInstallArgs(InstallRequest{
		Chart: "bitnami/grafana", Version: "8.7.0", ReleaseName: "g1",
		Namespace: "default", Hostname: "g.lab.local",
		IngressValuePath: "ingress.hostname", Atomic: true,
	}, "/tmp/values.yaml")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "install g1 bitnami/grafana") ||
		!strings.Contains(joined, "--version 8.7.0") ||
		!strings.Contains(joined, "--namespace default") ||
		!strings.Contains(joined, "--create-namespace") ||
		!strings.Contains(joined, "--atomic") ||
		!strings.Contains(joined, "--set ingress.hostname=g.lab.local") ||
		!strings.Contains(joined, "--set ingress.enabled=true") ||
		!strings.Contains(joined, "--set ingress.className=traefik") ||
		!strings.Contains(joined, "-f /tmp/values.yaml") {
		t.Fatalf("args mismatch: %s", joined)
	}
}

func TestBuildUpgradeArgs(t *testing.T) {
	args := buildUpgradeArgs(InstallRequest{
		Chart: "bitnami/grafana", Version: "8.7.0", ReleaseName: "g1",
		Namespace: "default", Atomic: true,
	}, "")
	joined := strings.Join(args, " ")
	if !strings.HasPrefix(joined, "upgrade g1 bitnami/grafana") {
		t.Fatalf("upgrade prefix wrong: %s", joined)
	}
	if !strings.Contains(joined, "--install") {
		t.Fatalf("missing --install: %s", joined)
	}
}

// TestBuildInstallArgsIncludesTimeout pins the v0.1.14 timeout wiring.
// Without --timeout, helm uses its 5m default and Longhorn first-install
// times out before image pulls finish (see helm_timeout.go).
func TestBuildInstallArgsIncludesTimeout(t *testing.T) {
	t.Setenv("BANDOLIER_HELM_INSTALL_TIMEOUT", "")
	args := buildInstallArgs(InstallRequest{
		Chart: "longhorn/longhorn", Version: "1.11.2", ReleaseName: "longhorn",
		Namespace: "longhorn-system",
	}, "")
	// Exact-pair match so a future regression that emits the duration as
	// "15m0s" or some other shape is caught explicitly. Walk args to find
	// the --timeout flag and verify the next arg is exactly "15m".
	var timeoutIdx = -1
	for i, a := range args {
		if a == "--timeout" {
			timeoutIdx = i
			break
		}
	}
	if timeoutIdx == -1 || timeoutIdx+1 >= len(args) {
		t.Fatalf("--timeout flag not found in args: %v", args)
	}
	if got := args[timeoutIdx+1]; got != "15m" {
		t.Errorf("--timeout value: got %q want %q (full args: %v)", got, "15m", args)
	}
}

func TestHelmInstallTimeoutFlag(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"unset → default 15m", "", "15m"},
		{"valid 30m → 30m", "30m", "30m"},
		{"valid 1h → 1h", "1h", "1h"},
		{"valid 600s → 600s", "600s", "600s"},
		{"malformed → default", "30 min", "15m"},
		{"v-prefix rejected → default", "v30m", "15m"},
		{"shell injection rejected → default", "30m; rm -rf /", "15m"},
		{"decimal rejected → default", "1.5h", "15m"},
		{"unsupported unit rejected → default", "30d", "15m"},
		// Zero-guard: 0s / 0m / 0h pass the regex but mean "no timeout"
		// in helm, which would let a genuinely-stuck install hang
		// forever. Fall back to the default flag.
		{"zero-second timeout rejected → default", "0s", "15m"},
		{"zero-minute timeout rejected → default", "0m", "15m"},
		{"zero-hour timeout rejected → default", "0h", "15m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BANDOLIER_HELM_INSTALL_TIMEOUT", tc.env)
			got := helmInstallTimeoutFlag()
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

// TestHelmInstallTimeoutDurationZeroGuard verifies the duration helper
// also rejects zero values — the slog.Warn line in the production code
// catches operator misconfigurations that the flag helper might miss.
func TestHelmInstallTimeoutDurationZeroGuard(t *testing.T) {
	t.Setenv("BANDOLIER_HELM_INSTALL_TIMEOUT", "0s")
	got := helmInstallTimeout()
	if got != defaultHelmInstallTimeout {
		t.Errorf("zero env value should fall back to default; got %v want %v", got, defaultHelmInstallTimeout)
	}
}
