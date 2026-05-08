package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// dummyBcryptHash is a valid cost-12 bcrypt hash used to equalize timing
// when the user lookup fails. The hash is unreachable (no preimage exists).
const dummyBcryptHash = "$2a$12$abcdefghijklmnopqrstuOoUhDghBpvw/RmCFTPgT.tEwS5mDDvwS"

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

type credentials struct {
	Password string `json:"password"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, err := h.store.CountUsers(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db"})
		return
	}
	if n > 0 {
		_, _ = audit.Write(ctx, h.store, audit.Entry{
			Action:  string(audit.ActionAuthSetup),
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "already_configured"},
		})
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already initialized"})
		return
	}
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil || c.Password == "" {
		_, _ = audit.Write(ctx, h.store, audit.Entry{
			Action:  string(audit.ActionAuthSetup),
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "invalid_json"},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password required"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), 12)
	if err != nil {
		_, _ = audit.Write(ctx, h.store, audit.Entry{
			Action:  string(audit.ActionAuthSetup),
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "hash_error"},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hash"})
		return
	}
	if _, err := h.store.CreateUser(ctx, string(hash)); err != nil {
		_, _ = audit.Write(ctx, h.store, audit.Entry{
			Action:  string(audit.ActionAuthSetup),
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "create_user"},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "create user"})
		return
	}
	_, _ = audit.Write(ctx, h.store, audit.Entry{
		ActorID: 1, // first (and only) user in v1
		Action:  string(audit.ActionAuthSetup),
		Outcome: audit.OutcomeSuccess,
	})
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil || c.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password required"})
		return
	}
	user, err := h.store.GetUserByID(ctx, 1)
	if err != nil {
		// Equalize timing with the password-mismatch branch so the existence of
		// a user record is not observable by response latency.
		_ = bcrypt.CompareHashAndPassword([]byte(dummyBcryptHash), []byte(c.Password))
		_, _ = audit.Write(ctx, h.store, audit.Entry{
			Action:  string(audit.ActionAuthLogin),
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "invalid_credentials"},
		})
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(c.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			_, _ = audit.Write(ctx, h.store, audit.Entry{
				Action:  string(audit.ActionAuthLogin),
				Outcome: audit.OutcomeFailure,
				Details: map[string]any{"reason": "invalid_credentials"},
			})
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "compare"})
		return
	}
	id, err := newSessionID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "session"})
		return
	}
	if err := h.store.CreateSession(ctx, id, user.ID, SessionTTLSeconds); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "session"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   SessionTTLSeconds,
	})
	_, _ = audit.Write(ctx, h.store, audit.Entry{
		ActorID: user.ID,
		Action:  string(audit.ActionAuthLogin),
		Outcome: audit.OutcomeSuccess,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(SessionCookieName)
	if err == nil && c.Value != "" {
		_ = h.store.DeleteSession(context.Background(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	uid, _ := UserIDFromContext(r.Context())
	_, _ = audit.Write(r.Context(), h.store, audit.Entry{
		ActorID: uid,
		Action:  string(audit.ActionAuthLogout),
		Outcome: audit.OutcomeSuccess,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type changePasswordReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Status returns whether the master password has been configured (i.e. at
// least one user record exists). This endpoint is public — no auth required —
// so the UI can decide whether to route new visitors to /setup or /login.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	n, err := h.store.CountUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"configured": n > 0})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var body changePasswordReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := ValidatePassword(body.NewPassword); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// Note: this endpoint is gated by RequireSession, which has already resolved
	// the user id. A user-not-found here implies a stale session, not an attack —
	// no constant-time dummy bcrypt is needed (unlike Login).
	user, err := h.store.GetUserByID(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.CurrentPassword)); err != nil {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  "change_password",
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "wrong_current_password"},
		})
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password incorrect"})
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), 12)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hash failed"})
		return
	}
	if err := h.store.UpdateUserPassword(r.Context(), uid, string(newHash)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "update failed"})
		return
	}
	_, _ = audit.Write(r.Context(), h.store, audit.Entry{
		ActorID: uid,
		Action:  "change_password",
		Outcome: audit.OutcomeSuccess,
	})
	w.WriteHeader(http.StatusNoContent)
}
