package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/abuxton/pheromone/internal/twin"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	twinKeyPrefix = "/pheromone/twins/"
)

// EtcdStore provides etcd-backed persistent storage
type EtcdStore struct {
	client *clientv3.Client
}

// NewEtcdStore creates a new etcd store
func NewEtcdStore(endpoints []string) (*EtcdStore, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	return &EtcdStore{
		client: cli,
	}, nil
}

// Close closes the etcd connection
func (e *EtcdStore) Close() error {
	if err := e.client.Close(); err != nil {
		return fmt.Errorf("close etcd client: %w", err)
	}
	return nil
}

// Set stores a twin in etcd
func (e *EtcdStore) Set(ctx context.Context, t *twin.Twin) error {
	data, err := t.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize twin: %w", err)
	}

	key := twinKeyPrefix + t.ID
	_, err = e.client.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("failed to write to etcd: %w", err)
	}

	return nil
}

// Get retrieves a twin from etcd
func (e *EtcdStore) Get(ctx context.Context, id string) (*twin.Twin, error) {
	key := twinKeyPrefix + id
	resp, err := e.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to read from etcd: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, ErrTwinNotFound
	}

	t, err := twin.FromJSON(resp.Kvs[0].Value)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize twin: %w", err)
	}

	return t, nil
}

// Delete removes a twin from etcd
func (e *EtcdStore) Delete(ctx context.Context, id string) error {
	key := twinKeyPrefix + id
	_, err := e.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete from etcd: %w", err)
	}

	return nil
}

// List returns all twins from etcd
func (e *EtcdStore) List(ctx context.Context) ([]*twin.Twin, error) {
	resp, err := e.client.Get(ctx, twinKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list from etcd: %w", err)
	}

	twins := make([]*twin.Twin, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		t, err := twin.FromJSON(kv.Value)
		if err != nil {
			// Skip invalid entries
			continue
		}
		twins = append(twins, t)
	}

	return twins, nil
}

// Watch returns a channel for watching twin changes
func (e *EtcdStore) Watch(ctx context.Context) clientv3.WatchChan {
	return e.client.Watch(ctx, twinKeyPrefix, clientv3.WithPrefix())
}

// LoadAll loads all twins from etcd into memory store
func (e *EtcdStore) LoadAll(ctx context.Context, mem *MemoryStore) (int, error) {
	twins, err := e.List(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, t := range twins {
		if err := mem.Set(t); err != nil {
			return count, fmt.Errorf("failed to load twin %s: %w", t.ID, err)
		}
		count++
	}

	return count, nil
}

// DeleteAll removes all twins from etcd
func (e *EtcdStore) DeleteAll(ctx context.Context) error {
	_, err := e.client.Delete(ctx, twinKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("failed to delete all twins from etcd: %w", err)
	}
	return nil
}

// GetTwinIDFromKey extracts twin ID from etcd key
func GetTwinIDFromKey(key string) string {
	return strings.TrimPrefix(key, twinKeyPrefix)
}
