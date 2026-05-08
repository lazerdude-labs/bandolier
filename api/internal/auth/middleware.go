package auth

import (
	"context"
	"net/http"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

type ctxKey string

const userCtxKey ctxKey = "user_id"

func RequireSession(s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(SessionCookieName)
			if err != nil || c.Value == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
				return
			}
			sess, err := s.GetSession(r.Context(), c.Value)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session expired"})
				return
			}
			ctx := context.WithValue(r.Context(), userCtxKey, sess.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(userCtxKey).(int64)
	return v, ok
}
