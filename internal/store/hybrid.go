package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/abuxton/pheromone/internal/twin"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// HybridStore combines in-memory cache with etcd persistence
type HybridStore struct {
	memory *MemoryStore
	etcd   *EtcdStore
	mu     sync.RWMutex
}

// NewHybridStore creates a new hybrid store
func NewHybridStore(etcdEndpoints []string) (*HybridStore, error) {
	etcdStore, err := NewEtcdStore(etcdEndpoints)
	if err != nil {
		return nil, err
	}

	return &HybridStore{
		memory: NewMemoryStore(),
		etcd:   etcdStore,
	}, nil
}

// Close closes the etcd connection
func (h *HybridStore) Close() error {
	return h.etcd.Close()
}

// Set stores a twin in both memory and etcd
func (h *HybridStore) Set(ctx context.Context, t *twin.Twin) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Write to memory first (fast path)
	if err := h.memory.Set(t); err != nil {
		return err
	}

	// Write to etcd (durable path)
	if err := h.etcd.Set(ctx, t); err != nil {
		// Rollback memory write on etcd failure
		h.memory.Delete(t.ID)
		return fmt.Errorf("etcd write failed, rolled back: %w", err)
	}

	return nil
}

// Get retrieves a twin from memory (with etcd fallback)
func (h *HybridStore) Get(ctx context.Context, id string) (*twin.Twin, error) {
	// Try memory first (fast path)
	t, err := h.memory.Get(id)
	if err == nil {
		return t, nil
	}

	// Fallback to etcd if not in memory
	h.mu.Lock()
	defer h.mu.Unlock()

	t, err = h.etcd.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Populate cache (ignore error as read succeeded from etcd)
	// Memory population failure doesn't affect the read operation
	_ = h.memory.Set(t)
	return t, nil
}

// Delete removes a twin from both memory and etcd
func (h *HybridStore) Delete(ctx context.Context, id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Delete from memory
	h.memory.Delete(id)

	// Delete from etcd
	if err := h.etcd.Delete(ctx, id); err != nil {
		return err
	}

	return nil
}

// List returns all twins from memory
func (h *HybridStore) List() []*twin.Twin {
	return h.memory.List()
}

// Count returns the number of twins in memory
func (h *HybridStore) Count() int {
	return h.memory.Count()
}

// CacheHitRatio returns the cache hit ratio
func (h *HybridStore) CacheHitRatio() float64 {
	return h.memory.CacheHitRatio()
}

// RecoverFromEtcd loads all twins from etcd into memory
func (h *HybridStore) RecoverFromEtcd(ctx context.Context) (int, time.Duration, error) {
	start := time.Now()
	count, err := h.etcd.LoadAll(ctx, h.memory)
	duration := time.Since(start)
	return count, duration, err
}

// Watch returns a channel for watching twin changes in etcd
func (h *HybridStore) Watch(ctx context.Context) clientv3.WatchChan {
	return h.etcd.Watch(ctx)
}

// SyncFromWatch synchronizes memory state from etcd watch events
func (h *HybridStore) SyncFromWatch(ctx context.Context) error {
	watchChan := h.Watch(ctx)

	go func() {
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				twinID := GetTwinIDFromKey(string(event.Kv.Key))

				switch event.Type {
				case clientv3.EventTypePut:
					t, err := twin.FromJSON(event.Kv.Value)
					if err == nil {
						h.memory.Set(t)
					}
				case clientv3.EventTypeDelete:
					h.memory.Delete(twinID)
				}
			}
		}
	}()

	return nil
}
