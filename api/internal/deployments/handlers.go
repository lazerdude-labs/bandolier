package deployments

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// wsOriginPatterns parses the BANDOLIER_WS_ORIGIN_PATTERNS env var into the
// shape coder/websocket's AcceptOptions expects. Empty/unset means the library
// default applies: the request host is always authorized (strict same-origin),
// and no extra origins are allowed. That's the safe default for the standard
// loopback-bound deploy where the UI and API are served from the same host.
//
// Set the env var to a comma-separated list of host patterns
// (e.g. "localhost:5173,*.lab.internal") to allow additional origins —
// typically only needed when running the UI dev server on a different port,
// or when intentionally exposing the stack on a LAN with multiple hostnames.
//
// Patterns use path.Match semantics (case-insensitive). DO NOT set "*" — that
// disables origin enforcement entirely; use InsecureSkipVerify (which we never
// set) if you really need to.
func wsOriginPatterns() []string {
	v := os.Getenv("BANDOLIER_WS_ORIGIN_PATTERNS")
	if v == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(v, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

type Handler struct {
	Store    *store.Store
	Executor *Executor
	Hub      *Hub
}

func NewHandler(s *store.Store, e *Executor, h *Hub) *Handler {
	return &Handler{Store: s, Executor: e, Hub: h}
}

func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	depID, err := h.Executor.Deploy(r.Context(), id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"deployment_id": depID})
}

func (h *Handler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := h.Store.GetDeployment(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d)
}

// ListForCluster returns the most recent deployments for a cluster, newest first.
// Optional `?limit=N` query param caps the result count.
func (h *Handler) ListForCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	out, err := h.Store.ListDeploymentsForCluster(r.Context(), id, limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: wsOriginPatterns(),
	})
	if err != nil {
		return
	}
	defer func() { _ = conn.Close(websocket.StatusInternalError, "") }()

	ctx, cancel := context.WithTimeout(r.Context(), 24*time.Hour)
	defer cancel()

	snapshot, ch, unsub := h.Hub.Subscribe(id, 64)
	defer unsub()

	// Replay history so a late subscriber (eg. tab navigated back to the
	// live deploy) sees prior step_start/step_end transitions and any log
	// lines they missed. If the deployment already completed, the terminal
	// event is in the snapshot and we close right after replay.
	for _, ev := range snapshot {
		if err := wsjson.Write(ctx, conn, ev); err != nil {
			return
		}
		if ev.Type == EventDeploymentComplete {
			_ = conn.Close(websocket.StatusNormalClosure, "done")
			return
		}
	}

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if err := wsjson.Write(ctx, conn, ev); err != nil {
				return
			}
			if ev.Type == EventDeploymentComplete {
				_ = conn.Close(websocket.StatusNormalClosure, "done")
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
