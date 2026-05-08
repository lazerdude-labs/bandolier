package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebSocketSessionRejectsMissingProtocol(t *testing.T) {
	mw := WebSocketSession(testKey)
	srv := httptest.NewServer(mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})))
	defer srv.Close()
	resp, _ := http.Get(srv.URL)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestWebSocketSessionAcceptsValidToken(t *testing.T) {
	tok, _ := MintWSToken(testKey, 42, 60*time.Second)
	mw := WebSocketSession(testKey)
	called := false
	srv := httptest.NewServer(mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		uid, _ := UserIDFromContext(r.Context())
		if uid != 42 {
			t.Errorf("uid = %d, want 42", uid)
		}
	})))
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL, nil)
	req.Header.Set("Sec-WebSocket-Protocol", "bandolier.ws."+tok)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !called {
		t.Fatal("handler not called")
	}
	if !strings.Contains(resp.Header.Get("Sec-WebSocket-Protocol"), "bandolier.ws.") {
		t.Errorf("response missing echoed subprotocol: %s", resp.Header.Get("Sec-WebSocket-Protocol"))
	}
}

func TestWebSocketSessionRejectsBadToken(t *testing.T) {
	mw := WebSocketSession(testKey)
	srv := httptest.NewServer(mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})))
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL, nil)
	req.Header.Set("Sec-WebSocket-Protocol", "bandolier.ws.garbage")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestWebSocketSessionRejectsWrongProtocolPrefix(t *testing.T) {
	tok, _ := MintWSToken(testKey, 42, 60*time.Second)
	mw := WebSocketSession(testKey)
	srv := httptest.NewServer(mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})))
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL, nil)
	req.Header.Set("Sec-WebSocket-Protocol", "wrong.prefix."+tok)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
