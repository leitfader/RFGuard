package metrics

import (
	"sync"
	"time"

	"rfguard/internal/model"
)

type Store struct {
	mu        sync.RWMutex
	byReader  map[string]map[int]model.WindowMetrics
	updatedAt map[string]time.Time
	limit     int
}

func NewStore(limit int) *Store {
	if limit <= 0 {
		limit = 5000
	}
	return &Store{
		byReader:  make(map[string]map[int]model.WindowMetrics),
		updatedAt: make(map[string]time.Time),
		limit:     limit,
	}
}

func (s *Store) Update(readerID string, metrics []model.WindowMetrics) {
	if readerID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.byReader[readerID]
	if !ok {
		m = make(map[int]model.WindowMetrics)
		s.byReader[readerID] = m
	}
	for _, wm := range metrics {
		m[wm.WindowSec] = wm
	}
	s.updatedAt[readerID] = time.Now().UTC()
	if len(s.byReader) > s.limit {
		s.evictOldest()
	}
}

func (s *Store) Get(readerID string) ([]model.WindowMetrics, time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.byReader[readerID]
	if !ok {
		return nil, time.Time{}, false
	}
	out := make([]model.WindowMetrics, 0, len(m))
	for _, wm := range m {
		out = append(out, wm)
	}
	return out, s.updatedAt[readerID], true
}

func (s *Store) GetAll() map[string][]model.WindowMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]model.WindowMetrics, len(s.byReader))
	for readerID, m := range s.byReader {
		list := make([]model.WindowMetrics, 0, len(m))
		for _, wm := range m {
			list = append(list, wm)
		}
		out[readerID] = list
	}
	return out
}

func (s *Store) evictOldest() {
	var oldestReader string
	var oldest time.Time
	for reader, ts := range s.updatedAt {
		if oldestReader == "" || ts.Before(oldest) {
			oldestReader = reader
			oldest = ts
		}
	}
	if oldestReader != "" {
		delete(s.byReader, oldestReader)
		delete(s.updatedAt, oldestReader)
	}
}

func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byReader = make(map[string]map[int]model.WindowMetrics)
	s.updatedAt = make(map[string]time.Time)
}
