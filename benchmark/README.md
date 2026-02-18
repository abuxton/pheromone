# ADR-002 Server Architecture Performance Validation

This directory contains benchmarks and tests to validate the performance characteristics of the hybrid in-memory + etcd persistence architecture proposed in ADR-002.

## Success Criteria

The benchmarks validate the following requirements:

1. **etcd write latency**: P99 < 100ms
2. **Cache hit ratio**: > 80% for twin state queries
3. **Server recovery time from etcd**: < 5 seconds
4. **No data loss on write failure**: Validated through rollback logic

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose (for etcd)
- etcd 3.5+

## Setup

1. Start etcd using Docker Compose:

```bash
docker-compose up -d
```

2. Verify etcd is running:

```bash
docker ps | grep etcd
```

3. Check etcd health:

```bash
curl http://localhost:2379/health
```

## Running Tests

### Memory-Only Tests

Test in-memory store performance (no etcd required):

```bash
go test -v ./benchmark -run TestMemoryCacheHitRatio
```

### Memory Benchmarks

Run memory store benchmarks:

```bash
# Write performance
go test -bench=BenchmarkMemoryStoreWrite ./benchmark -benchtime=10000x

# Read performance
go test -bench=BenchmarkMemoryStoreRead ./benchmark -benchtime=100000x

# Concurrent read performance
go test -bench=BenchmarkMemoryStoreConcurrentRead ./benchmark -benchtime=100000x
```

### etcd Tests

Test etcd integration (requires etcd running):

```bash
# Write latency validation (P99 < 100ms)
go test -v ./benchmark -run TestEtcdWriteLatency

# Server recovery time validation (< 5 seconds)
go test -v ./benchmark -run TestServerRecoveryTime

# Data loss prevention validation
go test -v ./benchmark -run TestNoDataLossOnWriteFailure
```

### etcd Benchmarks

Run etcd store benchmarks:

```bash
# Write performance
go test -bench=BenchmarkEtcdWrite ./benchmark -benchtime=1000x

# Read performance
go test -bench=BenchmarkEtcdRead ./benchmark -benchtime=1000x

# Range query performance
go test -bench=BenchmarkEtcdRangeQuery ./benchmark -benchtime=100x
```

### Hybrid Store Tests

Test hybrid store (in-memory + etcd):

```bash
# Cache hit ratio validation (> 80%)
go test -v ./benchmark -run TestHybridCacheHitRatio

# Recovery validation
go test -v ./benchmark -run TestHybridRecovery
```

### Hybrid Store Benchmarks

Run hybrid store benchmarks:

```bash
# Write performance (memory + etcd)
go test -bench=BenchmarkHybridStoreWrite ./benchmark -benchtime=1000x

# Read performance (cache hits)
go test -bench=BenchmarkHybridStoreRead ./benchmark -benchtime=100000x
```

## Run All Tests

Execute all tests and benchmarks:

```bash
# Run all tests
go test -v ./benchmark -run Test

# Run all benchmarks
go test -bench=. ./benchmark -benchmem
```

## Expected Results

### Memory Store
- **Write latency**: < 1µs per operation
- **Read latency**: < 1µs per operation
- **Concurrent read**: Scales linearly with CPU cores
- **Cache hit ratio**: > 80% for typical workloads

### etcd Store
- **Write latency P99**: < 100ms (typically 10-50ms for local etcd)
- **Read latency**: 1-10ms
- **Range query (1000 twins)**: 10-50ms
- **Recovery time (1000 twins)**: < 5 seconds (typically 100-500ms)

### Hybrid Store
- **Write latency**: Dominated by etcd write (10-100ms)
- **Read latency (cache hit)**: < 1µs
- **Read latency (cache miss)**: 1-10ms (etcd read)
- **Cache hit ratio**: > 80% for repeated queries

## Architecture Components

### Files

- `internal/twin/twin.go` - Digital twin model
- `internal/store/memory.go` - In-memory store implementation
- `internal/store/etcd.go` - etcd persistence layer
- `internal/store/hybrid.go` - Hybrid store combining both
- `benchmark/*_test.go` - Benchmark and validation tests

### Design

The hybrid architecture follows the CDC (Change Data Capture) pattern:

1. **Write Path**: Memory → etcd (with rollback on failure)
2. **Read Path**: Memory (cache hit) → etcd fallback (cache miss)
3. **Recovery Path**: etcd → Memory (on server restart)
4. **Sync Path**: etcd watch → Memory (for multi-server Phase 2)

## Cleanup

Stop and remove etcd:

```bash
docker-compose down -v
```

## Environment Variables

- `ETCD_ENDPOINT`: etcd endpoint (default: `localhost:2379`)

Example:

```bash
ETCD_ENDPOINT=localhost:2379 go test -v ./benchmark
```

## Results Documentation

After running the benchmarks, document the results in the ADR-002 validation section:

- Latency distributions (P50, P95, P99)
- Memory usage patterns
- Cache hit ratios
- Recovery times
- Any deviations from success criteria
