package profiles

import (
	"encoding/json"
	"net/http"
)

type listHandler struct {
	reg *Registry
}

func NewListHandler(reg *Registry) http.Handler {
	return &listHandler{reg: reg}
}

func (h *listHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	out := make([]Metadata, 0)
	for _, p := range h.reg.All() {
		out = append(out, p.Metadata())
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
