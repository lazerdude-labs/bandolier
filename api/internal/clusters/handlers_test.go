package clusters_test

import (
	"context"
	"errors"
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

	c := &store.Cluster{ID: "00000000000000000000000000000001", Name: "del-foo", Profile: "homelab", Status: "destroyed"}
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

	c := &store.Cluster{ID: "00000000000000000000000000000002", Name: "live-foo", Profile: "homelab", Status: "ready"}
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

	missingID := "00000000000000000000000000000099"
	req := httptest.NewRequest("DELETE", "/api/clusters/"+missingID, nil)
	req = withChiURLParam(req, "id", missingID)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
}

// TestDeleteCascadeOnLiveClusterKicksOffDestroyAndSetsLatch confirms the
// happy path: DELETE ?cascade=destroy on a `ready` cluster (a) invokes the
// destroy executor, (b) sets pending_forget=1 on the row, (c) returns 202
// with the destroy deployment_id. The actual Forget happens in the
// executor's runDestroy success path, which is tested separately in the
// deployments package.
func TestDeleteCascadeOnLiveClusterKicksOffDestroyAndSetsLatch(t *testing.T) {
	s := newTestStore(t)
	reg := newTestRegistry()

	c := &store.Cluster{ID: "00000000000000000000000000000011", Name: "live-foo", Profile: "homelab", Status: "ready"}
	if err := s.CreateCluster(context.Background(), c); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	called := false
	exec := &fakeExecutor{destroyFn: func(_ context.Context, cid string) (string, error) {
		called = true
		if cid != c.ID {
			t.Errorf("got cluster id %s want %s", cid, c.ID)
		}
		return "dep-cascade-1", nil
	}}
	h := clusters.NewHandler(s, reg, nil).WithDestroyExecutor(exec)

	req := httptest.NewRequest("DELETE", "/api/clusters/"+c.ID+"?cascade=destroy", nil)
	req = withChiURLParam(req, "id", c.ID)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("executor.Destroy not invoked on cascade path")
	}
	got, err := s.GetCluster(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if !got.PendingForget {
		t.Fatal("pending_forget should be true after cascade delete request")
	}
}

// TestDeleteCascadeRollsBackLatchOnDestroyError confirms that if the
// executor fails to kick off destroy (e.g. terraform unavailable, lock
// contention), pending_forget is rolled back so a future operator-
// initiated destroy doesn't silently auto-forget.
func TestDeleteCascadeRollsBackLatchOnDestroyError(t *testing.T) {
	s := newTestStore(t)
	reg := newTestRegistry()

	c := &store.Cluster{ID: "00000000000000000000000000000012", Name: "live-bar", Profile: "homelab", Status: "ready"}
	if err := s.CreateCluster(context.Background(), c); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	exec := &fakeExecutor{destroyFn: func(_ context.Context, _ string) (string, error) {
		return "", errors.New("destroy executor blew up")
	}}
	h := clusters.NewHandler(s, reg, nil).WithDestroyExecutor(exec)

	req := httptest.NewRequest("DELETE", "/api/clusters/"+c.ID+"?cascade=destroy", nil)
	req = withChiURLParam(req, "id", c.ID)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	got, err := s.GetCluster(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if got.PendingForget {
		t.Fatal("pending_forget must be rolled back when destroy kickoff fails")
	}
}

// TestDeleteCascadeOnTransientStateStill409 confirms that ?cascade=destroy
// against `deploying` / `upgrading` / `destroying` (clusters with a live
// goroutine running against them) is rejected with 409. Cascade is for
// live but quiescent states, not in-flight ones.
func TestDeleteCascadeOnTransientStateStill409(t *testing.T) {
	s := newTestStore(t)
	reg := newTestRegistry()

	c := &store.Cluster{ID: "00000000000000000000000000000013", Name: "in-flight", Profile: "homelab", Status: "deploying"}
	if err := s.CreateCluster(context.Background(), c); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	exec := &fakeExecutor{destroyFn: func(context.Context, string) (string, error) {
		t.Fatal("destroy should NOT be invoked against a transient state")
		return "", nil
	}}
	h := clusters.NewHandler(s, reg, nil).WithDestroyExecutor(exec)

	req := httptest.NewRequest("DELETE", "/api/clusters/"+c.ID+"?cascade=destroy", nil)
	req = withChiURLParam(req, "id", c.ID)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
}
