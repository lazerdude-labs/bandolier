package telemetry

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data    []NodeTelemetry
	expires time.Time
}

type cache struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]cacheEntry
}

func newCache(ttl time.Duration) *cache {
	return &cache{ttl: ttl, m: map[string]cacheEntry{}}
}

func (c *cache) get(key string) ([]NodeTelemetry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.data, true
}

func (c *cache) put(key string, v []NodeTelemetry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cacheEntry{data: v, expires: time.Now().Add(c.ttl)}
}

// putWithTTL stores entries with a custom TTL, overriding the cache's
// default. Used by Aggregator to give empty results a shorter window
// (so transient probe failures recover faster).
func (c *cache) putWithTTL(key string, v []NodeTelemetry, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cacheEntry{data: v, expires: time.Now().Add(ttl)}
}
