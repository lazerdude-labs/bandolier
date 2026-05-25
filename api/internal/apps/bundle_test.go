package apps

import "testing"

// TestCountInstallable pins the "N of M" denominator that step_progress
// events expose to the UI. Skips must NOT count toward total — the operator
// deselected them and the banner shouldn't blame them for charts they chose
// not to install. See issue #42.
func TestCountInstallable(t *testing.T) {
	cases := []struct {
		name    string
		choices []BundleChartChoice
		want    int
	}{
		{"empty", nil, 0},
		{"all skipped", []BundleChartChoice{{Skip: true}, {Skip: true}}, 0},
		{"none skipped", []BundleChartChoice{{Skip: false}, {Skip: false}, {Skip: false}}, 3},
		{"mixed skip and install", []BundleChartChoice{{Skip: false}, {Skip: true}, {Skip: false}}, 2},
		{"single install", []BundleChartChoice{{Skip: false}}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := countInstallable(c.choices); got != c.want {
				t.Errorf("countInstallable(%+v) = %d, want %d", c.choices, got, c.want)
			}
		})
	}
}

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
