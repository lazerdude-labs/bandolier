package deployments

import "testing"

func TestTraefikChartVersion(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"unset → default", "", defaultTraefikChartVersion},
		{"valid 34.5.0 → 34.5.0", "34.5.0", "34.5.0"},
		{"valid 35.0.0 → 35.0.0", "35.0.0", "35.0.0"},
		{"valid pre-release 39.1.0-ea.1 → as-is", "39.1.0-ea.1", "39.1.0-ea.1"},
		{"v-prefix rejected → default (helm chart versions don't carry v)", "v34.5.0", defaultTraefikChartVersion},
		{"shell injection rejected → default", "34.5.0; rm -rf /", defaultTraefikChartVersion},
		{"semver-bare-major rejected → default", "34", defaultTraefikChartVersion},
		{"empty → default", "", defaultTraefikChartVersion},
		{"whitespace rejected → default", " 34.5.0 ", defaultTraefikChartVersion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BANDOLIER_TRAEFIK_CHART_VERSION", tc.env)
			got := traefikChartVersion()
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}
