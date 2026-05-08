package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/lazerdude-labs/bandolier/api/internal/auth"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

func newTestHandler(t *testing.T) (*auth.Handler, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return auth.NewHandler(s), s
}

func TestSetupCreatesFirstUser(t *testing.T) {
	h, s := newTestHandler(t)
	body, _ := json.Marshal(map[string]string{"password": "correct horse battery staple"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Setup(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	n, _ := s.CountUsers(context.Background())
	if n != 1 {
		t.Fatalf("expected 1 user, got %d", n)
	}
}

func TestSetupRefusesSecondCall(t *testing.T) {
	h, _ := newTestHandler(t)
	body, _ := json.Marshal(map[string]string{"password": "p"})

	rr1 := httptest.NewRecorder()
	h.Setup(rr1, httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body)))

	rr2 := httptest.NewRecorder()
	h.Setup(rr2, httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body)))
	if rr2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr2.Code)
	}
}

func TestLoginSuccessSetsCookie(t *testing.T) {
	h, _ := newTestHandler(t)
	body, _ := json.Marshal(map[string]string{"password": "secret"})
	rr := httptest.NewRecorder()
	h.Setup(rr, httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body)))

	rr2 := httptest.NewRecorder()
	h.Login(rr2, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body)))
	if rr2.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", rr2.Code, rr2.Body.String())
	}
	cookies := rr2.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != "bandolier_session" {
		t.Fatalf("expected bandolier_session cookie, got %v", cookies)
	}
}

func TestLoginWrongPasswordReturns401(t *testing.T) {
	h, _ := newTestHandler(t)
	good, _ := json.Marshal(map[string]string{"password": "secret"})
	bad, _ := json.Marshal(map[string]string{"password": "guess"})

	rr := httptest.NewRecorder()
	h.Setup(rr, httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(good)))
	rr2 := httptest.NewRecorder()
	h.Login(rr2, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bad)))
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr2.Code)
	}
}

func TestLoginNoUserStillRunsBcrypt(t *testing.T) {
	h, _ := newTestHandler(t)
	body, _ := json.Marshal(map[string]string{"password": "anything"})
	start := time.Now()
	rr := httptest.NewRecorder()
	h.Login(rr, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body)))
	elapsed := time.Since(start)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
	// Bcrypt cost 12 takes ~100ms+ on most hardware; assert at least 50ms to
	// confirm the dummy comparison ran. This is a smoke test, not a strict bound.
	if elapsed < 50*time.Millisecond {
		t.Fatalf("login returned too fast (%v); dummy bcrypt did not run", elapsed)
	}
}

func TestChangePasswordSuccess(t *testing.T) {
	h, s := newTestHandler(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("originalpw123"), 12)
	_, _ = s.CreateUser(ctx, string(hash))

	body := strings.NewReader(`{"current_password":"originalpw123","new_password":"newpassword123"}`)
	req := httptest.NewRequest("POST", "/api/auth/change-password", body)
	req = req.WithContext(auth.WithUserID(req.Context(), 1))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ChangePassword(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	u, _ := s.GetUserByID(ctx, 1)
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("newpassword123")); err != nil {
		t.Fatalf("new hash does not match: %v", err)
	}
	count, err := s.CountAuditEntries(ctx, "change_password", "success")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("audit success count = %d, want 1", count)
	}
}

func TestChangePasswordWrongCurrentReturns401(t *testing.T) {
	h, s := newTestHandler(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("originalpw123"), 12)
	_, _ = s.CreateUser(ctx, string(hash))

	body := strings.NewReader(`{"current_password":"WRONG","new_password":"newpassword123"}`)
	req := httptest.NewRequest("POST", "/api/auth/change-password", body)
	req = req.WithContext(auth.WithUserID(req.Context(), 1))
	rr := httptest.NewRecorder()
	h.ChangePassword(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
	count, err := s.CountAuditEntries(ctx, "change_password", "failure")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("audit failure count = %d, want 1", count)
	}
}

func TestChangePasswordShortNewReturns400(t *testing.T) {
	h, s := newTestHandler(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("originalpw123"), 12)
	_, _ = s.CreateUser(ctx, string(hash))

	body := strings.NewReader(`{"current_password":"originalpw123","new_password":"short"}`)
	req := httptest.NewRequest("POST", "/api/auth/change-password", body)
	req = req.WithContext(auth.WithUserID(req.Context(), 1))
	rr := httptest.NewRecorder()
	h.ChangePassword(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestLoginAuditsSuccess(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("originalpw123"), 12)
	_, _ = s.CreateUser(ctx, string(hash))

	h := auth.NewHandler(s)
	body := strings.NewReader(`{"password":"originalpw123"}`)
	req := httptest.NewRequest("POST", "/api/auth/login", body)
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	if rr.Code != http.StatusNoContent && rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	count, _ := s.CountAuditEntries(ctx, "auth_login", "success")
	if count != 1 {
		t.Fatalf("audit count=%d, want 1", count)
	}
}

func TestLoginAuditsFailure(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("originalpw123"), 12)
	_, _ = s.CreateUser(ctx, string(hash))

	h := auth.NewHandler(s)
	body := strings.NewReader(`{"password":"WRONG"}`)
	req := httptest.NewRequest("POST", "/api/auth/login", body)
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
	count, _ := s.CountAuditEntries(ctx, "auth_login", "failure")
	if count != 1 {
		t.Fatalf("audit failure count=%d, want 1", count)
	}
}
