package clusters

import (
	"testing"
	"time"
)

func TestShouldRenew(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		expires time.Time
		want    bool
		desc    string
	}{
		{now.Add(8 * 24 * time.Hour), false, "8 days out — leave alone"},
		{now.Add(7*24*time.Hour - time.Hour), true, "just under 7 days — renew"},
		{now.Add(-1 * time.Hour), true, "expired — renew"},
		{now.Add(30 * 24 * time.Hour), false, "30 days out — leave alone"},
	}
	for _, c := range cases {
		got := shouldRenew(c.expires, now)
		if got != c.want {
			t.Errorf("%s: shouldRenew(%v) = %v, want %v", c.desc, c.expires, got, c.want)
		}
	}
}
