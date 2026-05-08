package auth

import (
	"encoding/json"
	"net/http"
	"time"
)

// wsTokenTTL is the validity window for a freshly-minted WS token. Short
// enough that XSS exfil yields only a brief WS subscription window.
const wsTokenTTL = 60 * time.Second

// NewWSTokenHandler returns POST /api/auth/ws-token. Requires session auth
// (caller must wrap with RequireSession middleware).
func NewWSTokenHandler(key []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := UserIDFromContext(r.Context())
		if !ok || uid == 0 {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		token, err := MintWSToken(key, uid, wsTokenTTL)
		if err != nil {
			http.Error(w, "mint failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      token,
			"expires_at": time.Now().Add(wsTokenTTL).UTC().Format(time.RFC3339),
		})
	})
}
