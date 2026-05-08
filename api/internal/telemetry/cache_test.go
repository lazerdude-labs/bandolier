package telemetry

import (
	"testing"
	"time"
)

func TestCacheHitWithinTTL(t *testing.T) {
	c := newCache(50 * time.Millisecond)
	c.put("k", []NodeTelemetry{{Name: "x"}})
	got, ok := c.get("k")
	if !ok || len(got) != 1 {
		t.Fatalf("hit failed: ok=%v len=%d", ok, len(got))
	}
}

func TestCacheExpiresAfterTTL(t *testing.T) {
	c := newCache(20 * time.Millisecond)
	c.put("k", []NodeTelemetry{{Name: "x"}})
	time.Sleep(40 * time.Millisecond)
	if _, ok := c.get("k"); ok {
		t.Fatalf("expected miss after TTL")
	}
}

func TestCachePutWithTTLOverride(t *testing.T) {
	c := newCache(60 * time.Second) // default TTL
	c.putWithTTL("k", []NodeTelemetry{{Name: "x"}}, 50*time.Millisecond)
	if got, ok := c.get("k"); !ok || len(got) != 1 {
		t.Fatalf("expected hit immediately")
	}
	time.Sleep(80 * time.Millisecond)
	if _, ok := c.get("k"); ok {
		t.Fatalf("expected miss after override TTL expired")
	}
}
