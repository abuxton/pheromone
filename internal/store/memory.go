package store

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/abuxton/pheromone/internal/twin"
)

var (
	ErrTwinNotFound = errors.New("twin not found")
	ErrTwinExists   = errors.New("twin already exists")
)

// MemoryStore provides in-memory twin storage with cache statistics
type MemoryStore struct {
	mu          sync.RWMutex
	twins       map[string]*twin.Twin
	cacheHits   atomic.Int64
	cacheMisses atomic.Int64
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		twins: make(map[string]*twin.Twin),
	}
}

// Set stores a twin in memory
func (m *MemoryStore) Set(t *twin.Twin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t.UpdatedAt = time.Now()
	m.twins[t.ID] = t
	return nil
}

// Get retrieves a twin from memory
func (m *MemoryStore) Get(id string) (*twin.Twin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, exists := m.twins[id]
	if !exists {
		m.cacheMisses.Add(1)
		return nil, ErrTwinNotFound
	}

	m.cacheHits.Add(1)
	return t, nil
}

// Delete removes a twin from memory
func (m *MemoryStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.twins[id]; !exists {
		return ErrTwinNotFound
	}

	delete(m.twins, id)
	return nil
}

// List returns all twins
func (m *MemoryStore) List() []*twin.Twin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*twin.Twin, 0, len(m.twins))
	for _, t := range m.twins {
		result = append(result, t)
	}
	return result
}

// Count returns the number of twins
func (m *MemoryStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.twins)
}

// CacheHitRatio returns the cache hit ratio
func (m *MemoryStore) CacheHitRatio() float64 {
	hits := m.cacheHits.Load()
	misses := m.cacheMisses.Load()

	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total)
}

// ResetStats resets cache statistics
func (m *MemoryStore) ResetStats() {
	m.cacheHits.Store(0)
	m.cacheMisses.Store(0)
}
