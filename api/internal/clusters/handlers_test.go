package clusters_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/clusters"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/homelab"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newTestRegistry() *profiles.Registry {
	reg := profiles.NewRegistry()
	reg.Register(homelab.New("", ""))
	return reg
}

func TestCreateCluster(t *testing.T) {
	s := newTestStore(t)
	reg := newTestRegistry()
	h := clusters.NewHandler(s, reg, nil)

	body := strings.NewReader(`{"name":"test-cluster","profile":"homelab"}`)
	req := httptest.NewRequest("POST", "/api/clusters", body)
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestCreateClusterMissingFields(t *testing.T) {
	s := newTestStore(t)
	reg := newTestRegistry()
	h := clusters.NewHandler(s, reg, nil)

	body := strings.NewReader(`{"name":""}`)
	req := httptest.NewRequest("POST", "/api/clusters", body)
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestCreateRejectsDisabledProfile(t *testing.T) {
	s := newTestStore(t)
	reg := profiles.NewRegistry()
	// homelab stub with Enabled=false (NewStub forces Enabled=false)
	reg.Register(profiles.NewStub(profiles.Metadata{Name: "homelab"}))
	// red-team registered as a stub (disabled) to verify the
	// "reject disabled profile" path. Production red-team is now enabled
	// (delegates to homelab); we only need *some* disabled profile here.
	reg.Register(profiles.NewStub(profiles.Metadata{Name: "red-team"}))

	h := clusters.NewHandler(s, reg, nil)
	body := strings.NewReader(`{"name":"foo","profile":"red-team"}`)
	req := httptest.NewRequest("POST", "/api/clusters", body)
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
}

type fakeExecutor struct {
	destroyFn func(context.Context, string) (string, error)
}

func (f *fakeExecutor) Destroy(ctx context.Context, id string) (string, error) {
	return f.destroyFn(ctx, id)
}

func TestDestroyHandlerCallsExecutor(t *testing.T) {
	s := newTestStore(t)

	// Create a cluster directly in the store to have a valid ID.
	c := &store.Cluster{
		ID:      "abc",
		Name:    "foo",
		Profile: "homelab",
		Status:  "ready",
	}
	if err := s.CreateCluster(context.Background(), c); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	called := false
	exec := &fakeExecutor{destroyFn: func(ctx context.Context, cid string) (string, error) {
		called = true
		if cid != c.ID {
			t.Errorf("got cluster id %s want %s", cid, c.ID)
		}
		return "dep-1", nil
	}}
	h := clusters.NewDestroyHandler(s, exec)
	req := httptest.NewRequest("POST", "/api/clusters/"+c.ID+"/destroy", nil)
	req = withChiURLParam(req, "id", c.ID)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("executor.Destroy not invoked")
	}
}

func withChiURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestDeleteRemovesRowWhenStatusDeletable(t *testing.T) {
	s := newTestStore(t)
	reg := newTestRegistry()
	h := clusters.NewHandler(s, reg, nil)

	c := &store.Cluster{ID: "delcluster", Name: "del-foo", Profile: "homelab", Status: "destroyed"}
	if err := s.CreateCluster(context.Background(), c); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/clusters/"+c.ID, nil)
	req = withChiURLParam(req, "id", c.ID)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}

	if _, err := s.GetCluster(context.Background(), c.ID); err == nil {
		t.Fatal("expected cluster row gone, still present")
	}
}

func TestDeleteRejectsLiveStatusWith409(t *testing.T) {
	s := newTestStore(t)
	reg := newTestRegistry()
	h := clusters.NewHandler(s, reg, nil)

	c := &store.Cluster{ID: "live", Name: "live-foo", Profile: "homelab", Status: "ready"}
	if err := s.CreateCluster(context.Background(), c); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/clusters/"+c.ID, nil)
	req = withChiURLParam(req, "id", c.ID)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	// Row must still be there — guard rejected the delete.
	if _, err := s.GetCluster(context.Background(), c.ID); err != nil {
		t.Fatalf("cluster row should still exist after 409, got %v", err)
	}
}

func TestDeleteMissingReturns404(t *testing.T) {
	s := newTestStore(t)
	reg := newTestRegistry()
	h := clusters.NewHandler(s, reg, nil)

	req := httptest.NewRequest("DELETE", "/api/clusters/nope", nil)
	req = withChiURLParam(req, "id", "nope")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
}
