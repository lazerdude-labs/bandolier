package clusters

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/auth"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

type joinTokenHandler struct {
	store *store.Store
	vault *vault.Client
}

func NewJoinTokenHandler(s *store.Store, v *vault.Client) http.Handler {
	return &joinTokenHandler{store: s, vault: v}
}

func (h *joinTokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	uid, _ := auth.UserIDFromContext(r.Context())

	if _, err := h.store.GetCluster(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			_, _ = audit.Write(r.Context(), h.store, audit.Entry{
				ActorID: uid,
				Action:  string(audit.ActionClusterJoinTokenRetrieve),
				Target:  id,
				Outcome: audit.OutcomeFailure,
				Details: map[string]any{"reason": "not_found"},
			})
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "cluster not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	paths := vault.Paths{}
	data, err := h.vault.Get(r.Context(), paths.JoinToken(id))
	if err != nil || data == nil {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterJoinTokenRetrieve),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "not_yet_retrieved"},
		})
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "join token not yet retrieved"})
		return
	}
	token, _ := data["token"].(string)
	if token == "" {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterJoinTokenRetrieve),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "not_yet_retrieved"},
		})
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "join token empty"})
		return
	}
	retrievedAt, _ := data["retrieved_at"].(string)

	_, _ = audit.Write(r.Context(), h.store, audit.Entry{
		ActorID: uid,
		Action:  string(audit.ActionClusterJoinTokenRetrieve),
		Target:  id,
		Outcome: audit.OutcomeSuccess,
	})
	writeJSON(w, http.StatusOK, map[string]string{
		"token":        token,
		"retrieved_at": retrievedAt,
	})
}

// JoinTokenRetriever decouples the handler from the deployments package so it
// can be unit-tested without spinning up the executor.
type JoinTokenRetriever interface {
	RetrieveJoinToken(ctx context.Context, clusterID string) error
}

type joinTokenRetrieveHandler struct {
	store *store.Store
	exec  JoinTokenRetriever
}

func NewJoinTokenRetrieveHandler(s *store.Store, e JoinTokenRetriever) http.Handler {
	return &joinTokenRetrieveHandler{store: s, exec: e}
}

func (h *joinTokenRetrieveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.exec.RetrieveJoinToken(r.Context(), id)
	switch {
	case errors.Is(err, ErrClusterNotReady):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
