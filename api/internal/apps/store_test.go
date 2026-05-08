package apps_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/apps"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustCluster(t *testing.T, s *store.Store, id, name string) {
	t.Helper()
	if err := s.CreateCluster(context.Background(), &store.Cluster{ID: id, Name: name, Profile: "homelab", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
}

func TestRepoCRUD(t *testing.T) {
	s := newStore(t)
	mustCluster(t, s, "c1", "n1")
	as := apps.NewStore(s)
	ctx := context.Background()

	id, err := as.CreateRepo(ctx, "c1", "bitnami", "https://charts.bitnami.com/bitnami", nil)
	if err != nil || id == 0 {
		t.Fatalf("create: %v %d", err, id)
	}
	rs, err := as.ListRepos(ctx, "c1")
	if err != nil || len(rs) != 1 || rs[0].Name != "bitnami" {
		t.Fatalf("list: %v %+v", err, rs)
	}
	if err := as.DeleteRepo(ctx, "c1", "bitnami"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	rs, _ = as.ListRepos(ctx, "c1")
	if len(rs) != 0 {
		t.Fatalf("expected empty: %+v", rs)
	}
}

func TestInstallCRUD(t *testing.T) {
	s := newStore(t)
	mustCluster(t, s, "c1", "n1")
	as := apps.NewStore(s)
	ctx := context.Background()

	in := &apps.Install{
		ID: "abc", ClusterID: "c1", Chart: "bitnami/grafana", Version: "v8.7.0",
		ReleaseName: "grafana", Namespace: "default", Operation: "install",
		Status: "running", Atomic: true,
	}
	if err := as.CreateInstall(ctx, in); err != nil {
		t.Fatal(err)
	}
	got, err := as.GetInstall(ctx, "abc")
	if err != nil || got.ReleaseName != "grafana" {
		t.Fatalf("get: %v %+v", err, got)
	}
	if err := as.FinishInstall(ctx, "abc", "succeeded", ""); err != nil {
		t.Fatal(err)
	}
	got, _ = as.GetInstall(ctx, "abc")
	if got.Status != "succeeded" {
		t.Fatalf("status: %s", got.Status)
	}
}
