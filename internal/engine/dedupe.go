package engine

import (
	"sync"
	"time"
)

type DedupeCache struct {
	mu    sync.Mutex
	items map[string]time.Time
}

func NewDedupeCache() *DedupeCache {
	return &DedupeCache{items: make(map[string]time.Time)}
}

func (d *DedupeCache) Seen(key string, now time.Time, ttl time.Duration) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if ts, ok := d.items[key]; ok {
		if now.Sub(ts) <= ttl {
			return true
		}
	}
	d.items[key] = now
	if len(d.items) > 10000 {
		d.compact(now, ttl)
	}
	return false
}

func (d *DedupeCache) compact(now time.Time, ttl time.Duration) {
	for k, ts := range d.items {
		if now.Sub(ts) > ttl {
			delete(d.items, k)
		}
	}
}
