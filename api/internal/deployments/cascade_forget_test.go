package deployments_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/clusters"
	"github.com/lazerdude-labs/bandolier/api/internal/deployments"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// newCascadeTestStore stands up a fresh on-disk SQLite store with the full
// migration set applied. We use a real DB (not a mock) because the cascade
// machinery touches deployments + clusters tables jointly and the latch
// behavior under SetPendingForget + RecoverOrphanedDeployments is the
// invariant under test.
func newCascadeTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestRecoverOrphanedDeploymentsClearsPendingForget pins the fix for the
// CR-1 race the v0.1.7 review caught: if the api restarts while a cascade-
// destroy is mid-flight, RecoverOrphanedDeployments must clear the
// pending_forget latch alongside the cluster status flip. Without this,
// a subsequent operator-initiated destroy on the now-`error` cluster
// would silently auto-forget — surprising the operator who only meant
// to retry.
func TestRecoverOrphanedDeploymentsClearsPendingForget(t *testing.T) {
	s := newCascadeTestStore(t)
	ctx := context.Background()

	// A cluster with the cascade latch set, as if a DELETE ?cascade=destroy
	// had just kicked off and the api process then died.
	c := &store.Cluster{ID: "cascade01", Name: "abc", Profile: "homelab", Status: "destroying"}
	if err := s.CreateCluster(ctx, c); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
	if err := s.SetPendingForget(ctx, c.ID, true); err != nil {
		t.Fatalf("SetPendingForget: %v", err)
	}

	// And an orphan destroy deployment from that crashed process.
	// ActorID is nullable on deployments (the column is a FK to users with
	// implicit ON DELETE SET NULL). Leaving it nil here avoids needing to
	// create a user row just to satisfy the FK — the recovery code under
	// test doesn't inspect actor_id at all.
	d := &store.Deployment{
		ID:        "d-orphan-01",
		ClusterID: c.ID,
		Operation: "destroy",
		Status:    "running",
	}
	if err := s.CreateDeployment(ctx, d); err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	// Build a minimal Executor; only Store is exercised by recovery.
	e := &deployments.Executor{Store: s}
	if err := e.RecoverOrphanedDeployments(ctx); err != nil {
		t.Fatalf("RecoverOrphanedDeployments: %v", err)
	}

	got, err := s.GetCluster(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if got.PendingForget {
		t.Fatal("pending_forget should be cleared on orphan recovery; was still true")
	}
	if got.Status != string(clusters.StatusError) {
		t.Errorf("status: got %q want %q", got.Status, clusters.StatusError)
	}
}

// TestForgetClusterOrchestratorRemovesRow exercises the pure forget
// orchestrator: with nil vault (the helper short-circuits), it should
// drop the cluster row and write an audit entry. Used by both the
// operator-initiated DELETE path and the executor's runDestroy cascade
// hook, so the contract here is load-bearing.
func TestForgetClusterOrchestratorRemovesRow(t *testing.T) {
	s := newCascadeTestStore(t)
	ctx := context.Background()

	c := &store.Cluster{ID: "forget01", Name: "to-forget", Profile: "homelab", Status: "destroyed"}
	if err := s.CreateCluster(ctx, c); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	if err := clusters.ForgetCluster(ctx, s, nil, c.ID, c.Name, c.Status, 42); err != nil {
		t.Fatalf("ForgetCluster: %v", err)
	}

	if _, err := s.GetCluster(ctx, c.ID); err == nil {
		t.Fatal("cluster row should be gone after ForgetCluster, still present")
	}
}
