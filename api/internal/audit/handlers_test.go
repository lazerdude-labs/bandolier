package audit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

func TestListHandlerReturnsRows(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_, _ = s.InsertAuditEntry(ctx, store.AuditEntry{ActorID: 1, Action: "auth_login", Outcome: "success"})
	_, _ = s.InsertAuditEntry(ctx, store.AuditEntry{ActorID: 1, Action: "cluster_deploy", Outcome: "started"})

	h := audit.NewListHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/audit-log?limit=10", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var got []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows, got %d", len(got))
	}
}

func TestListHandlerActionFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_, _ = s.InsertAuditEntry(ctx, store.AuditEntry{ActorID: 1, Action: "auth_login", Outcome: "success"})
	_, _ = s.InsertAuditEntry(ctx, store.AuditEntry{ActorID: 1, Action: "cluster_deploy", Outcome: "started"})

	h := audit.NewListHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/audit-log?action=auth_login", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var got []map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&got)
	if len(got) != 1 || got[0]["action"] != "auth_login" {
		t.Fatalf("want 1 auth_login row, got %v", got)
	}
}
