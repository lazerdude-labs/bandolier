package vault_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	vapi "github.com/hashicorp/vault/api"

	v "github.com/lazerdude-labs/bandolier/api/internal/vault"
)

// fakeServer mocks the Vault KV v2 API surface our client uses.
// Sufficient for unit tests; Phase 7's compose stack runs a real Vault.
func fakeServer(t *testing.T, expectedToken string) (*httptest.Server, *map[string]map[string]any) {
	t.Helper()
	storage := map[string]map[string]any{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/bandolier/data/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != expectedToken {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/v1/bandolier/data/")
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			body := map[string]map[string]any{}
			_ = decodeJSON(r, &body)
			storage[key] = body["data"]
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"version":1}}`))
		case http.MethodGet:
			data, ok := storage[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			encodeJSON(w, map[string]any{"data": map[string]any{"data": data}})
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &storage
}

func TestPutAndGetSecret(t *testing.T) {
	srv, _ := fakeServer(t, "test-token")

	cfg := vapi.DefaultConfig()
	cfg.Address = srv.URL
	cli, err := vapi.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cli.SetToken("test-token")

	c := v.NewClient(cli, v.KVMount)
	ctx := context.Background()
	if err := c.Put(ctx, "clusters/abc/proxmox", map[string]any{"endpoint": "https://192.0.2.1"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := c.Get(ctx, "clusters/abc/proxmox")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got["endpoint"] != "https://192.0.2.1" {
		t.Fatalf("unexpected: %+v", got)
	}
}

// JSON helpers for the fake server.
func decodeJSON(r *http.Request, v any) error { return json.NewDecoder(r.Body).Decode(v) }
func encodeJSON(w http.ResponseWriter, v any) { _ = json.NewEncoder(w).Encode(v) }
