package homelab

import (
	"encoding/json"
	"net/http"
	"sort"
)

type distrosHandler struct{}

// NewDistrosHandler returns the GET /api/distros handler. The response is the
// curated distro catalog as a JSON array, sorted by ID for deterministic order.
func NewDistrosHandler() http.Handler {
	return &distrosHandler{}
}

func (h *distrosHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	out := make([]Distro, 0, len(Catalog))
	for _, d := range Catalog {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
