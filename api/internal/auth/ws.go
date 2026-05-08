package auth

import (
	"context"
	"net/http"
	"strings"
)

const wsProtocolPrefix = "bandolier.ws."

// WebSocketSession is a chi middleware that authenticates WebSocket upgrade
// requests via the Sec-WebSocket-Protocol header. The client must send a
// header of the form "bandolier.ws.<token>" where <token> is a signed blob
// minted by /api/auth/ws-token. On success, the user_id is set in the
// request context (same key as RequireSession) and the protocol is echoed
// back to the client per RFC 6455.
func WebSocketSession(key []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			proto := r.Header.Get("Sec-WebSocket-Protocol")
			if proto == "" {
				http.Error(w, "missing subprotocol", http.StatusUnauthorized)
				return
			}
			// Header value may contain comma-separated alternatives; pick the
			// one with our prefix.
			var match string
			for _, p := range strings.Split(proto, ",") {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, wsProtocolPrefix) {
					match = p
					break
				}
			}
			if match == "" {
				http.Error(w, "no bandolier subprotocol", http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(match, wsProtocolPrefix)
			uid, err := VerifyWSToken(key, token)
			if err != nil {
				http.Error(w, "invalid ws token", http.StatusUnauthorized)
				return
			}
			// Echo the negotiated subprotocol back per RFC 6455.
			w.Header().Set("Sec-WebSocket-Protocol", match)
			ctx := context.WithValue(r.Context(), userCtxKey, uid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
