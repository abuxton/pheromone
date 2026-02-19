package benchmark

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/store"
	"github.com/abuxton/pheromone/internal/twin"
)

// createSyntheticTwin creates a twin with random data
func createSyntheticTwin(id string) *twin.Twin {
	return &twin.Twin{
		ID:             id,
		Name:           fmt.Sprintf("twin-%s", id),
		Type:           "os",
		State:          "active",
		ConfigVersion:  rand.Intn(100),
		LastHeartbeat:  time.Now(),
		LastConfigPush: time.Now().Add(-5 * time.Minute),
		Metadata: map[string]string{
			"hostname": fmt.Sprintf("host-%s", id),
			"zone":     fmt.Sprintf("zone-%d", rand.Intn(5)),
		},
		ConfigData: map[string]interface{}{
			"cpu_limit":    rand.Intn(16),
			"memory_limit": rand.Intn(64),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// BenchmarkMemoryStoreWrite tests in-memory write performance
func BenchmarkMemoryStoreWrite(b *testing.B) {
	mem := store.NewMemoryStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t := createSyntheticTwin(fmt.Sprintf("twin-%d", i))
		if err := mem.Set(t); err != nil {
			b.Fatalf("Failed to write twin: %v", err)
		}
	}
}

// BenchmarkMemoryStoreRead tests in-memory read performance
func BenchmarkMemoryStoreRead(b *testing.B) {
	mem := store.NewMemoryStore()

	// Pre-populate with 1000 twins
	for i := 0; i < 1000; i++ {
		t := createSyntheticTwin(fmt.Sprintf("twin-%d", i))
		mem.Set(t)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("twin-%d", rand.Intn(1000))
		_, err := mem.Get(id)
		if err != nil {
			// Cache miss is ok
		}
	}
}

// BenchmarkMemoryStoreConcurrentRead tests concurrent read performance
func BenchmarkMemoryStoreConcurrentRead(b *testing.B) {
	mem := store.NewMemoryStore()

	// Pre-populate with 1000 twins
	for i := 0; i < 1000; i++ {
		t := createSyntheticTwin(fmt.Sprintf("twin-%d", i))
		mem.Set(t)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := fmt.Sprintf("twin-%d", rand.Intn(1000))
			_, _ = mem.Get(id)
		}
	})
}

// TestMemoryCacheHitRatio validates cache hit ratio
func TestMemoryCacheHitRatio(t *testing.T) {
	mem := store.NewMemoryStore()
	mem.ResetStats()

	// Pre-populate with 100 twins
	for i := 0; i < 100; i++ {
		twin := createSyntheticTwin(fmt.Sprintf("twin-%d", i))
		mem.Set(twin)
	}

	// Perform 1000 reads (85% hit existing, 15% miss)
	// This ensures we exceed the 80% threshold reliably
	for i := 0; i < 1000; i++ {
		var id string
		if i < 850 {
			// First 850 reads hit existing twins
			id = fmt.Sprintf("twin-%d", i%100)
		} else {
			// Last 150 reads miss
			id = fmt.Sprintf("twin-missing-%d", i)
		}
		_, _ = mem.Get(id)
	}

	hitRatio := mem.CacheHitRatio()
	t.Logf("Cache hit ratio: %.2f%% (expected 85%%)", hitRatio*100)

	// Validate cache hit ratio is >80%
	if hitRatio < 0.80 {
		t.Errorf("Cache hit ratio %.2f%% is below required 80%%", hitRatio*100)
	}
}
