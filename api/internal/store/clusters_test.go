package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

func TestClusterCRUD(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	c := &store.Cluster{ID: "abc", Name: "homelab", Profile: "homelab", Status: "pending"}
	if err := s.CreateCluster(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.GetCluster(ctx, "abc")
	if err != nil || got.Name != "homelab" {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	if err := s.UpdateClusterStatus(ctx, "abc", "initialized"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = s.GetCluster(ctx, "abc")
	if got.Status != "initialized" {
		t.Fatalf("status: %s", got.Status)
	}
	list, _ := s.ListClusters(ctx)
	if len(list) != 1 {
		t.Fatalf("list: %+v", list)
	}
}

func TestDeleteClusterCascadesDeployments(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	c := &store.Cluster{ID: "del-1", Name: "to-delete", Profile: "homelab", Status: "destroyed"}
	if err := s.CreateCluster(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}
	dep := &store.Deployment{ID: "dep-1", ClusterID: c.ID, Operation: "deploy", Status: "succeeded"}
	if err := s.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	if err := s.DeleteCluster(ctx, c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := s.GetCluster(ctx, c.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	// Deployment row should be gone via ON DELETE CASCADE.
	if _, err := s.GetDeployment(ctx, dep.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected deployment ErrNotFound after cascade, got %v", err)
	}
}

func TestDeleteClusterMissingReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if err := s.DeleteCluster(ctx, "does-not-exist"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateClusterDuplicateNameReturnsErrDuplicateName(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	c1 := &store.Cluster{ID: "id1", Name: "homelab", Profile: "homelab", Status: "pending"}
	if err := s.CreateCluster(ctx, c1); err != nil {
		t.Fatalf("first create: %v", err)
	}
	c2 := &store.Cluster{ID: "id2", Name: "homelab", Profile: "homelab", Status: "pending"}
	err := s.CreateCluster(ctx, c2)
	if !errors.Is(err, store.ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}
}
