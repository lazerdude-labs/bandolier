package validate

import "testing"

func TestHostname(t *testing.T) {
	valid := []string{
		"redteam.example.com",
		"sysreptor.lab.rplab.lan",
		"k3s.rplab.lan",
		"a",
		"host1",
		"x-y.z-w.example",
	}
	for _, s := range valid {
		if !Hostname(s) {
			t.Errorf("expected %q valid", s)
		}
	}
	// Reject empty, helm --set injection vectors, and shell/format metacharacters.
	invalid := []string{
		"",
		"lab.local,admin.password=evil",      // helm --set comma injection
		"lab.local,ingress.enabled=false",
		"host=value",                          // equals
		"Lab.Local",                           // uppercase (k8s ingress is lowercase)
		"lab local",                           // space
		"-leading.com",
		"trailing-.com",
		".leadingdot",
		"trailingdot.",
		"$(whoami).com",
		"a;b.com",
		"a,b",
		"line\nbreak",
	}
	for _, s := range invalid {
		if Hostname(s) {
			t.Errorf("expected %q invalid", s)
		}
	}
}

// TestHostnameLength pins the 253-char DNS limit.
func TestHostnameLength(t *testing.T) {
	long := "aaaaaaaa"
	for len(long) <= 253 {
		long += ".aaaaaaaa"
	}
	if Hostname(long) {
		t.Errorf("expected over-length hostname (%d chars) invalid", len(long))
	}
}
