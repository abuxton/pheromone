package benchmark

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/abuxton/pheromone/internal/store"
)

// BenchmarkHybridStoreWrite tests hybrid store write performance
func BenchmarkHybridStoreWrite(b *testing.B) {
	hybridStore, err := store.NewHybridStore([]string{getEtcdEndpoint()})
	if err != nil {
		b.Skipf("Skipping hybrid benchmark: %v", err)
		return
	}
	defer hybridStore.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t := createSyntheticTwin(fmt.Sprintf("hybrid-twin-%d", i))
		if err := hybridStore.Set(ctx, t); err != nil {
			b.Fatalf("Failed to write twin: %v", err)
		}
	}
}

// BenchmarkHybridStoreRead tests hybrid store read performance (cache hits)
func BenchmarkHybridStoreRead(b *testing.B) {
	hybridStore, err := store.NewHybridStore([]string{getEtcdEndpoint()})
	if err != nil {
		b.Skipf("Skipping hybrid benchmark: %v", err)
		return
	}
	defer hybridStore.Close()

	ctx := context.Background()

	// Pre-populate with 1000 twins
	for i := 0; i < 1000; i++ {
		t := createSyntheticTwin(fmt.Sprintf("hybrid-twin-%d", i))
		hybridStore.Set(ctx, t)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("hybrid-twin-%d", rand.Intn(1000))
		_, err := hybridStore.Get(ctx, id)
		if err != nil {
			// Cache miss is ok
		}
	}
}

// TestHybridCacheHitRatio validates the hybrid store cache hit ratio
func TestHybridCacheHitRatio(t *testing.T) {
	hybridStore, err := store.NewHybridStore([]string{getEtcdEndpoint()})
	if err != nil {
		t.Skipf("Skipping hybrid test: %v", err)
		return
	}
	defer hybridStore.Close()

	ctx := context.Background()

	// Pre-populate with 100 twins
	for i := 0; i < 100; i++ {
		twin := createSyntheticTwin(fmt.Sprintf("hybrid-cache-twin-%d", i))
		if err := hybridStore.Set(ctx, twin); err != nil {
			t.Fatalf("Failed to write twin: %v", err)
		}
	}

	// Perform 1000 reads (targeting existing twins for cache hits)
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("hybrid-cache-twin-%d", rand.Intn(100))
		_, err := hybridStore.Get(ctx, id)
		if err != nil {
			t.Fatalf("Failed to read twin: %v", err)
		}
	}

	hitRatio := hybridStore.CacheHitRatio()
	t.Logf("Hybrid store cache hit ratio: %.2f%%", hitRatio*100)

	// After initial population, all reads should be cache hits (>80%)
	if hitRatio < 0.80 {
		t.Errorf("Cache hit ratio %.2f%% is below required 80%%", hitRatio*100)
	}
}

// TestHybridRecovery validates hybrid store recovery from etcd
func TestHybridRecovery(t *testing.T) {
	// Create first hybrid store and populate it
	hybridStore1, err := store.NewHybridStore([]string{getEtcdEndpoint()})
	if err != nil {
		t.Skipf("Skipping hybrid test: %v", err)
		return
	}

	ctx := context.Background()

	// Pre-populate with 500 twins
	t.Log("Populating hybrid store with 500 twins...")
	for i := 0; i < 500; i++ {
		twin := createSyntheticTwin(fmt.Sprintf("hybrid-recovery-twin-%d", i))
		if err := hybridStore1.Set(ctx, twin); err != nil {
			t.Fatalf("Failed to write twin: %v", err)
		}
	}

	hybridStore1.Close()

	// Create second hybrid store and recover from etcd (simulating server restart)
	hybridStore2, err := store.NewHybridStore([]string{getEtcdEndpoint()})
	if err != nil {
		t.Fatalf("Failed to create second hybrid store: %v", err)
	}
	defer hybridStore2.Close()

	t.Log("Recovering from etcd...")
	count, duration, err := hybridStore2.RecoverFromEtcd(ctx)
	if err != nil {
		t.Fatalf("Failed to recover from etcd: %v", err)
	}

	t.Logf("Recovered %d twins in %v", count, duration)

	// Validate all twins were recovered
	if count < 500 {
		t.Errorf("Expected at least 500 twins, got %d", count)
	}

	// Verify we can read them all from memory
	for i := 0; i < 500; i++ {
		id := fmt.Sprintf("hybrid-recovery-twin-%d", i)
		_, err := hybridStore2.Get(ctx, id)
		if err != nil {
			t.Errorf("Failed to read recovered twin %s: %v", id, err)
		}
	}
}
