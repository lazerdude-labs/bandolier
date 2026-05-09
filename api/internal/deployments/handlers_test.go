package deployments

import (
	"reflect"
	"testing"
)

func TestWsOriginPatterns_Empty(t *testing.T) {
	t.Setenv("BANDOLIER_WS_ORIGIN_PATTERNS", "")
	got := wsOriginPatterns()
	if got != nil {
		t.Fatalf("expected nil for unset env, got %v", got)
	}
}

func TestWsOriginPatterns_Single(t *testing.T) {
	t.Setenv("BANDOLIER_WS_ORIGIN_PATTERNS", "localhost:5173")
	got := wsOriginPatterns()
	want := []string{"localhost:5173"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWsOriginPatterns_MultipleWithSpaces(t *testing.T) {
	t.Setenv("BANDOLIER_WS_ORIGIN_PATTERNS", "localhost:5173, 127.0.0.1, *.lab.internal")
	got := wsOriginPatterns()
	want := []string{"localhost:5173", "127.0.0.1", "*.lab.internal"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWsOriginPatterns_DropsEmptyEntries(t *testing.T) {
	// Trailing/leading commas should not introduce empty origin strings.
	t.Setenv("BANDOLIER_WS_ORIGIN_PATTERNS", ",localhost,,127.0.0.1,")
	got := wsOriginPatterns()
	want := []string{"localhost", "127.0.0.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWsOriginPatterns_OnlyWhitespaceIsEmpty(t *testing.T) {
	t.Setenv("BANDOLIER_WS_ORIGIN_PATTERNS", "   ")
	got := wsOriginPatterns()
	if got != nil {
		t.Fatalf("expected nil for whitespace-only env, got %v", got)
	}
}

func TestWsOriginPatterns_DropsBareStar(t *testing.T) {
	// A bare `*` would match every hostname under path.Match and re-enable
	// the original cross-origin replay vector. Verify it's silently dropped
	// while legitimate wildcard subdomains pass through unchanged.
	t.Setenv("BANDOLIER_WS_ORIGIN_PATTERNS", "*,localhost,*.lab.internal,*")
	got := wsOriginPatterns()
	want := []string{"localhost", "*.lab.internal"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bare '*' should be dropped: got %v, want %v", got, want)
	}
}
