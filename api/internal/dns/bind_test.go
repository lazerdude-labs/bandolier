// api/internal/dns/bind_test.go
package dns

import (
	"strings"
	"testing"
)

func TestNormalizeName(t *testing.T) {
	cases := []struct{ in, zone, out string }{
		{"*.homelab", "lab.local", "*.homelab.lab.local."},
		{"*.homelab.lab.local", "lab.local", "*.homelab.lab.local."},
		{"*.homelab.lab.local.", "lab.local", "*.homelab.lab.local."},
		{"grafana.homelab", "lab.local", "grafana.homelab.lab.local."},
	}
	for _, c := range cases {
		got := normalizeName(c.in, c.zone)
		if got != c.out {
			t.Errorf("normalizeName(%q, %q) = %q, want %q", c.in, c.zone, got, c.out)
		}
	}
}

func TestBuildUpsertScript(t *testing.T) {
	got := buildUpsertScript("192.0.2.5:53", "lab.local", Record{
		Name: "*.homelab.lab.local.", Type: "A", TTL: 300, Data: "192.0.2.21",
	})
	if !strings.Contains(got, "server 192.0.2.5 53") {
		t.Errorf("missing server line: %s", got)
	}
	if !strings.Contains(got, "zone lab.local") {
		t.Errorf("missing zone line: %s", got)
	}
	if !strings.Contains(got, "update delete *.homelab.lab.local. A") {
		t.Errorf("missing delete-then-add upsert pattern: %s", got)
	}
	if !strings.Contains(got, "update add *.homelab.lab.local. 300 A 192.0.2.21") {
		t.Errorf("missing add line: %s", got)
	}
	if !strings.Contains(got, "send") {
		t.Errorf("missing send: %s", got)
	}
}

func TestBuildDeleteScript(t *testing.T) {
	got := buildDeleteScript("192.0.2.5:53", "lab.local", "*.homelab.lab.local.", "A")
	if !strings.Contains(got, "update delete *.homelab.lab.local. A") {
		t.Errorf("missing delete: %s", got)
	}
}

func TestParseServer(t *testing.T) {
	host, port := parseServer("192.0.2.5:53")
	if host != "192.0.2.5" || port != "53" {
		t.Errorf("got %s/%s", host, port)
	}
	host2, port2 := parseServer("192.0.2.5")
	if host2 != "192.0.2.5" || port2 != "53" {
		t.Errorf("default port wrong: %s/%s", host2, port2)
	}
}
