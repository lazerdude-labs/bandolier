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
