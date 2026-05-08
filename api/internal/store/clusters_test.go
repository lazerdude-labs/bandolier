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
