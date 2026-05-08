package apps

import "testing"

func TestSubstituteHostnameTemplate(t *testing.T) {
	cases := []struct {
		template, release, fqdn, want string
	}{
		{"{release}.{fqdn}", "grafana", "lab.local", "grafana.lab.local"},
		{"static.lab.local", "any", "lab.local", "static.lab.local"},
		{"", "any", "lab.local", ""},
	}
	for _, c := range cases {
		got := substituteHostnameTemplate(c.template, c.release, c.fqdn)
		if got != c.want {
			t.Errorf("substitute(%q, %q, %q) = %q, want %q",
				c.template, c.release, c.fqdn, got, c.want)
		}
	}
}
