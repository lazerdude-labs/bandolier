package clusters

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/dns"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

// dnsTestHandler powers POST /api/clusters/{id}/dns/test. The wizard's "Test
// connection" button hits this to confirm Bandolier can reach the configured
// DNS authority (BIND9 in v1) before the operator commits the cluster init.
type dnsTestHandler struct {
	store *store.Store
	vault *vault.Client
}

// NewDNSTestHandler returns the http.Handler. Constructed inline at route-wire
// time — no Deps field required since the dependencies are already on Deps.
func NewDNSTestHandler(s *store.Store, v *vault.Client) http.Handler {
	return &dnsTestHandler{store: s, vault: v}
}

func (h *dnsTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if _, err := h.store.GetCluster(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "cluster not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	data, err := h.vault.Get(r.Context(), "clusters/"+id+"/dns")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": "vault read: " + err.Error(),
		})
		return
	}

	cfg := dns.Config{
		Kind:       dns.Kind(asString(data["kind"])),
		Server:     asString(data["server"]),
		Zone:       asString(data["zone"]),
		TSIGName:   asString(data["tsig_name"]),
		TSIGSecret: asString(data["tsig_secret"]),
		APIToken:   asString(data["api_token"]),
	}

	provider, err := dns.NewProvider(cfg)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	if err := provider.Healthy(r.Context()); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// asString narrows an untyped Vault KV value to string. Vault returns map[string]any
// where strings round-trip cleanly through JSON; non-string values become "".
func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
