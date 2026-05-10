package homelab

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// scriptedProber returns a pre-canned response per URL, in call order. Used
// to drive PickReachableURL deterministically without touching the network.
type scriptedProber struct {
	responses []proberResponse
	calls     []string
}

type proberResponse struct {
	status int
	err    error
}

func (s *scriptedProber) Do(req *http.Request) (*http.Response, error) {
	s.calls = append(s.calls, req.URL.String())
	if len(s.calls) > len(s.responses) {
		return nil, errors.New("scriptedProber: more calls than responses")
	}
	r := s.responses[len(s.calls)-1]
	if r.err != nil {
		return nil, r.err
	}
	return &http.Response{StatusCode: r.status, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func TestPickReachableURLReturnsFirst200(t *testing.T) {
	urls := []string{"https://a/x", "https://b/x", "https://c/x"}
	p := &scriptedProber{responses: []proberResponse{{status: 200}}}
	got, attempts, err := PickReachableURL(context.Background(), urls, p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "https://a/x" {
		t.Errorf("got %q want %q", got, "https://a/x")
	}
	if len(attempts) != 0 {
		t.Errorf("attempts: %v want empty", attempts)
	}
	if len(p.calls) != 1 {
		t.Errorf("calls: %d want 1 (should short-circuit on first 2xx)", len(p.calls))
	}
}

func TestPickReachableURLFallsBackOn403(t *testing.T) {
	urls := []string{"https://a/x", "https://b/x", "https://c/x"}
	p := &scriptedProber{responses: []proberResponse{
		{status: 403},
		{status: 200},
	}}
	got, attempts, err := PickReachableURL(context.Background(), urls, p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "https://b/x" {
		t.Errorf("got %q want %q", got, "https://b/x")
	}
	if len(attempts) != 1 {
		t.Fatalf("attempts: got %d want 1", len(attempts))
	}
	if !strings.Contains(attempts[0].Error(), "HTTP 403") {
		t.Errorf("attempts[0]: %v want substring HTTP 403", attempts[0])
	}
}

func TestPickReachableURLAllFailReturnsError(t *testing.T) {
	urls := []string{"https://a/x", "https://b/x"}
	p := &scriptedProber{responses: []proberResponse{
		{status: 403},
		{err: errors.New("dial: connection refused")},
	}}
	got, attempts, err := PickReachableURL(context.Background(), urls, p)
	if err == nil {
		t.Fatal("expected error when all probes fail")
	}
	if got != "" {
		t.Errorf("got %q want empty on all-fail", got)
	}
	if len(attempts) != 2 {
		t.Errorf("attempts: got %d want 2", len(attempts))
	}
	if !strings.Contains(err.Error(), "no reachable mirror") {
		t.Errorf("err: %v want substring 'no reachable mirror'", err)
	}
}

func TestPickReachableURLEmptyListReturnsError(t *testing.T) {
	if _, _, err := PickReachableURL(context.Background(), nil, nil); err == nil {
		t.Error("expected error for empty URL list")
	}
}

func TestPickReachableURLContextCancellation(t *testing.T) {
	urls := []string{"https://a/x"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	// Use the default prober; the context carries through to Do which fails fast.
	_, _, err := PickReachableURL(ctx, urls, nil)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}
