package clusters

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lazerdude-labs/bandolier/api/internal/proxmox"
)

// ProxmoxTestRequest mirrors the proxmox section of the initialize request,
// trimmed to the fields the tester needs. The wizard sends this when the
// operator clicks the "Test reachability" button — pre-save, so there's no
// cluster ID and no Vault round-trip.
type ProxmoxTestRequest struct {
	Endpoint        string `json:"endpoint"`
	TokenID         string `json:"token_id"`
	TokenSecret     string `json:"token_secret"`
	Node            string `json:"node"`
	Storage         string `json:"storage"`
	ImageStorage    string `json:"image_storage"`
	SnippetsStorage string `json:"snippets_storage"`
	CABundle        string `json:"ca_bundle"`
}

// HandleProxmoxTest is the HTTP entry point for POST /api/proxmox/test. It
// runs the validation suite and returns a structured result. The endpoint is
// idempotent (no state writes) and behind RequireSession at the router so
// only authenticated operators can probe arbitrary Proxmox hosts via this
// handler — limits the SSRF blast radius to operators who could already
// initialize a cluster.
func HandleProxmoxTest(w http.ResponseWriter, r *http.Request) {
	var req ProxmoxTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}

	// Minimal up-front shape validation — the Run* path returns its own
	// per-check failures for everything else, but we want a 400 (not a check
	// list with five "endpoint malformed" failures) when the operator hasn't
	// filled in the required fields yet.
	if missing := requiredFields(req); len(missing) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":          "missing required fields",
			"missing_fields": missing,
		})
		return
	}

	result := proxmox.RunTests(r.Context(), proxmox.TestRequest{
		Endpoint:        strings.TrimSpace(req.Endpoint),
		TokenID:         strings.TrimSpace(req.TokenID),
		TokenSecret:     req.TokenSecret, // don't trim — could be intentional whitespace
		Node:            strings.TrimSpace(req.Node),
		Storage:         strings.TrimSpace(req.Storage),
		ImageStorage:    strings.TrimSpace(req.ImageStorage),
		SnippetsStorage: strings.TrimSpace(req.SnippetsStorage),
		CABundle:        req.CABundle,
	})
	writeJSON(w, http.StatusOK, result)
}

func requiredFields(req ProxmoxTestRequest) []string {
	var missing []string
	if strings.TrimSpace(req.Endpoint) == "" {
		missing = append(missing, "endpoint")
	}
	if strings.TrimSpace(req.TokenID) == "" {
		missing = append(missing, "token_id")
	}
	if req.TokenSecret == "" {
		missing = append(missing, "token_secret")
	}
	if strings.TrimSpace(req.Node) == "" {
		missing = append(missing, "node")
	}
	if strings.TrimSpace(req.Storage) == "" {
		missing = append(missing, "storage")
	}
	if strings.TrimSpace(req.ImageStorage) == "" {
		missing = append(missing, "image_storage")
	}
	if strings.TrimSpace(req.SnippetsStorage) == "" {
		missing = append(missing, "snippets_storage")
	}
	return missing
}
