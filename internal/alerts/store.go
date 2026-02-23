package alerts

import (
	"sync"
	"time"

	"rfguard/internal/model"
)

type Store struct {
	mu    sync.RWMutex
	buf   []model.Alert
	limit int
}

func NewStore(limit int) *Store {
	if limit <= 0 {
		limit = 1000
	}
	return &Store{limit: limit}
}

func (s *Store) Add(alert model.Alert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buf) < s.limit {
		s.buf = append(s.buf, alert)
		return
	}
	copy(s.buf, s.buf[1:])
	s.buf[len(s.buf)-1] = alert
}

func (s *Store) List(limit int) []model.Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.buf) {
		limit = len(s.buf)
	}
	out := make([]model.Alert, 0, limit)
	start := len(s.buf) - limit
	if start < 0 {
		start = 0
	}
	for i := start; i < len(s.buf); i++ {
		out = append(out, s.buf[i])
	}
	return out
}

func (s *Store) Since(ts time.Time) []model.Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Alert, 0)
	for _, a := range s.buf {
		if !a.Timestamp.Before(ts) {
			out = append(out, a)
		}
	}
	return out
}

func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = nil
}
