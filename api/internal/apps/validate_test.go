package apps

import "testing"

func TestValidHostname(t *testing.T) {
	valid := []string{
		"redteam.example.com",
		"sysreptor.lab.rplab.lan",
		"a",
		"host1",
		"a.b.c.d.e",
		"x-y.z-w.example",
	}
	for _, s := range valid {
		if !validHostname(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	// Reject empty, helm --set injection vectors, and shell/format metacharacters.
	invalid := []string{
		"",
		"app.example.com,admin.password=evil", // helm --set comma injection
		"app.example.com,ingress.enabled=false",
		"host=value",      // equals
		"Redteam.com",     // uppercase
		"red team.com",    // space
		"-leading.com",    // leading dash in label
		"trailing-.com",   // trailing dash in label
		".leadingdot.com", // empty leading label
		"trailingdot.com.",
		"$(whoami).com", // shell meta
		"a;b.com",       // semicolon
		"a,b",           // bare comma
		"line\nbreak",   // newline
	}
	for _, s := range invalid {
		if validHostname(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

// TestValidHostnameLength pins the 253-char DNS limit.
func TestValidHostnameLength(t *testing.T) {
	label := "aaaaaaaa" // 8 chars
	long := label
	for len(long) <= 253 {
		long += "." + label
	}
	if validHostname(long) {
		t.Errorf("expected over-length hostname (%d chars) to be invalid", len(long))
	}
}
