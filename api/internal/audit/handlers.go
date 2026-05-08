package audit

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

type listHandler struct {
	store *store.Store
}

func NewListHandler(s *store.Store) http.Handler {
	return &listHandler{store: s}
}

type wireRow struct {
	ID      int64   `json:"id"`
	ActorID *int64  `json:"actor_id"`
	Action  string  `json:"action"`
	Target  *string `json:"target"`
	Outcome string  `json:"outcome"`
	TS      string  `json:"ts"`
	Details *string `json:"details"`
}

func (h *listHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.AuditFilter{
		Action:  q.Get("action"),
		Outcome: q.Get("outcome"),
	}
	if v := q.Get("actor_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.ActorID = id
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}
	if v := q.Get("since"); v != "" {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			f.Since = &ts
		}
	}

	rows, err := h.store.ListAuditEntries(r.Context(), f)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	out := make([]wireRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, wireRow{
			ID: r.ID, ActorID: r.ActorID, Action: r.Action, Target: r.Target,
			Outcome: r.Outcome, TS: r.TS.UTC().Format(time.RFC3339), Details: r.Details,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
