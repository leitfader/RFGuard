package engine

import (
	"strconv"
	"sync"
	"time"
)

type Cooldown struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func NewCooldown() *Cooldown {
	return &Cooldown{last: make(map[string]time.Time)}
}

func (c *Cooldown) Allow(readerID string, windowSec int, cooldown time.Duration) bool {
	if cooldown <= 0 {
		return true
	}
	key := readerID + "|" + itoa(windowSec)
	return c.AllowKey(key, cooldown)
}

func (c *Cooldown) AllowKey(key string, cooldown time.Duration) bool {
	if cooldown <= 0 {
		return true
	}
	now := time.Now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	if ts, ok := c.last[key]; ok {
		if now.Sub(ts) < cooldown {
			return false
		}
	}
	c.last[key] = now
	return true
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
