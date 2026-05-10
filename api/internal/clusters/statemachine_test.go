package clusters_test

import (
	"testing"

	c "github.com/lazerdude-labs/bandolier/api/internal/clusters"
)

func TestAllowedTransitions(t *testing.T) {
	cases := []struct {
		from, to c.Status
		ok       bool
	}{
		{c.StatusPending, c.StatusInitializing, true},
		{c.StatusInitializing, c.StatusInitialized, true},
		{c.StatusInitialized, c.StatusDeploying, true},
		{c.StatusDeploying, c.StatusReady, true},
		{c.StatusDeploying, c.StatusError, true},
		{c.StatusError, c.StatusDeploying, true},
		{c.StatusPending, c.StatusReady, false},
		{c.StatusReady, c.StatusInitialized, false},
	}
	for _, tc := range cases {
		err := c.CanTransition(tc.from, tc.to)
		if (err == nil) != tc.ok {
			t.Errorf("%s -> %s: ok=%v err=%v", tc.from, tc.to, tc.ok, err)
		}
	}
}

func TestEditConfigTransitions(t *testing.T) {
	// Editing initialize config is allowed from initialized / destroyed /
	// error (all "no live cluster to surprise"). Live states are not
	// editable in-place.
	cases := []struct {
		from c.Status
		to   c.Status
		ok   bool
	}{
		{c.StatusInitialized, c.StatusInitializing, true},
		{c.StatusDestroyed, c.StatusInitializing, true},
		{c.StatusError, c.StatusInitializing, true},
		{c.StatusReady, c.StatusInitializing, false},
		{c.StatusDeploying, c.StatusInitializing, false},
		{c.StatusUpgrading, c.StatusInitializing, false},
		{c.StatusDestroying, c.StatusInitializing, false},
		{c.StatusDegraded, c.StatusInitializing, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			err := c.CanTransition(tc.from, tc.to)
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("want error")
			}
		})
	}
}

func TestDestroyTransitions(t *testing.T) {
	cases := []struct {
		from c.Status
		to   c.Status
		ok   bool
	}{
		{c.StatusReady, c.StatusDestroying, true},
		{c.StatusDegraded, c.StatusDestroying, true},
		{c.StatusError, c.StatusDestroying, true},
		{c.StatusDestroying, c.StatusDestroyed, true},
		{c.StatusDestroying, c.StatusError, true},
		{c.StatusDestroyed, c.StatusDeploying, true},
		{c.StatusPending, c.StatusDestroying, false},
		{c.StatusDeploying, c.StatusDestroying, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			err := c.CanTransition(tc.from, tc.to)
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("want error")
			}
		})
	}
}
