# ADR 006: Agent Lifecycle Interface - Custom Agent Framework

## Status
Proposed

## Context

Pheromone must enable developers to write custom agents (Python, Go, Rust) that integrate with the platform without deep framework knowledge (User Story 4, FR-020). The lifecycle interface defines what hooks agents must implement and what guarantees the framework provides.

**Requirements**:
- **Minimal Dependencies**: Custom agents should work with standard libraries (no forced frameworks)
- **Language-Agnostic**: Interface contracts defined via gRPC (ADR-003); any language can implement
- **Automatic Registration**: Agent framework handles gRPC setup; developer writes business logic only
- **Built-in Observability**: Structured logs and metrics exported automatically (Principle II)
- **Clear Error Handling**: Graceful degradation on configuration mismatches or transient failures

**Options Considered**:
- Full framework (like Telegraf plugins): Too opinionated; loses flexibility
- Minimal interface (pure gRPC): Requires developer to handle connection, retry, serialization logic
- Scaffolding + templating: Balanced approach; provides structure without rigidity

## Decision

**Define Agent Lifecycle Hooks + Provide Language-Specific Scaffolding**

### Agent Lifecycle Hooks (Pseudo-Code)

```
┌─────────────────────────────────────────────┐
│ 1. Initialize()                             │
│    - Load config                            │
│    - Set up local resources (files, db)     │
│    - Return: OS/Workload Twin objects       │
└──────────────┬──────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 2. Connect(server_address, agent_id)        │
│    - Establish gRPC connection to server    │
│    - Register twin objects                  │
│    - Start heartbeat loop (FR-006)          │
│    - Return: gRPC client connection         │
└──────────────┬───────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 3. CollectMetrics() [Continuous Loop]       │
│    - Poll local system (CPU, memory, etc.)  │
│    - Format as gRPC Metric messages         │
│    - Stream to server via TelemetryStream   │
│    - Emit structured logs (trace-id tags)   │
└──────────────┬───────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 4. EnforceConfig(desired_config) [Async]    │
│    - Apply configuration from server        │
│    - Validate against actual state          │
│    - Report enforcement status back         │
│    - Return: success/partial/failed         │
└──────────────┬───────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 5. HandleConfigUpdate(new_model_version)    │
│    - Called when server pushes new twin     │
│    - May trigger EnforceConfig()            │
│    - Handle backward compatibility          │
└──────────────┬───────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 6. Shutdown() [On SIGTERM/disconnect]       │
│    - Flush pending logs/metrics             │
│    - Clean up local resources               │
│    - Close gRPC connection gracefully       │
│    - Return: success or error               │
└──────────────────────────────────────────────┘
```

### Language-Specific Scaffolding

#### Go Agent Template

```go
// agent.go - User implements these methods
type Agent interface {
    Initialize() ([]Twin, error)
    CollectMetrics(ctx context.Context) ([]Metric, error)
    EnforceConfig(cfg Config) error
    Shutdown() error
}

// framework.go - Framework provides this
type AgentFramework struct {
    client  grpc.TelemetryStreamClient
    ticker  *time.Ticker
    logger  *logrus.Logger
}

func (f *AgentFramework) Run(agent Agent) {
    twins, _ := agent.Initialize()
    f.register(twins)

    for {
        select {
        case <-f.ticker.C:
            metrics, _ := agent.CollectMetrics(ctx)
            f.publish(metrics)
        case config := <-f.configChan:
            agent.EnforceConfig(config)
        case <-f.shutdownChan:
            agent.Shutdown()
            return
        }
    }
}

// User code: 50 lines
func main() {
    agent := &MyAgent{...}
    framework := NewAgentFramework("http://server:5050")
    framework.Run(agent)
}
```

#### Python Agent Template

```python
# agent.py - User implements these methods
class Agent(ABC):
    @abstractmethod
    def initialize(self) -> tuple[list[Twin], None]:
        pass

    @abstractmethod
    def collect_metrics(self) -> list[Metric]:
        pass

    @abstractmethod
    def enforce_config(self, cfg: Config) -> bool:
        pass

# framework.py - Framework provides this
class AgentFramework:
    def __init__(self, server_address: str):
        self.stub = TelemetryStreamStub(server_address)
        self.logger = logging.getLogger("pheromone-agent")

    def run(self, agent: Agent):
        twins, _ = agent.initialize()
        self.register(twins)

        while True:
            metrics = agent.collect_metrics()
            self.stream_metrics(metrics)
            sleep(5)  # collection interval

# User code: 30 lines
if __name__ == "__main__":
    agent = MyAgent()
    framework = AgentFramework("http://server:5050")
    framework.run(agent)
```

### Observability Guarantees

All agent implementations MUST emit:
- **Structured Logs** (JSON format with trace-id, component, level, message)
  ```json
  {
    "timestamp": "2026-02-18T20:10:39Z",
    "trace_id": "abc-123",
    "agent_id": "agent-1",
    "component": "metrics-collector",
    "level": "info",
    "message": "collected CPU metrics",
    "metrics_count": 42
  }
  ```

- **Metrics** (OpenMetrics format, auto-prefixed)
  ```
  pheromone_agent_metrics_collected_total{agent_id="agent-1"} 1000
  pheromone_agent_config_enforcements_total{agent_id="agent-1",status="success"} 95
  pheromone_agent_connection_errors_total{agent_id="agent-1"} 3
  ```

Framework handles metrics aggregation and export (agents just call `AddMetric()`).

### Configuration Schema Evolution

Agents declare supported Twin model versions:

```go
type MyAgent struct {
    SupportedVersions []string  // ["1.0.0", "1.1.0"]
}

func (a *MyAgent) EnforceConfig(cfg Config) error {
    if !contains(a.SupportedVersions, cfg.Version) {
        return fmt.Errorf("unsupported config version %s", cfg.Version)
    }
    // Apply config...
}
```

Server respects version constraints (doesn't push config to agents that don't support it).

## Consequences

### Positive
- **Developer Experience**: Scaffold generates 80% of boilerplate code; developers write 50-100 lines for new agent
- **Consistency**: All agents follow same lifecycle (regardless of language)
- **Observability**: Structured logs/metrics automatic (no developer work)
- **Type Safety**: Framework handles gRPC serialization/deserialization (developer works with Go/Python native types)
- **Testability**: Agents can be unit-tested without server (mock Twin/Config objects)

### Negative
- **Framework Complexity**: Core team must maintain Go, Python, Rust scaffold versions
- **Language Support**: Only actively maintained languages get scaffold (not JavaScript, PHP, etc.)
- **Debugging**: Framework abstraction makes troubleshooting harder if logic buried in framework
- **Performance**: Abstraction layers have overhead (acceptable for <10K agents; may need optimization for Phase 2)

### Implementation Effort
- **Go Scaffold**: ~200 lines (core framework + examples)
- **Python Scaffold**: ~250 lines (gRPC client boilerplate more verbose)
- **Rust Scaffold**: ~300 lines (Type system more verbose but safer)
- **Documentation + Examples**: 1 full example per language (nginx service, PostgreSQL service)

## Testing Strategy

- **Unit Test Examples**:
  ```go
  // No server needed; mock gRPC connection
  func TestAgentMetricCollection(t *testing.T) {
      agent := &NginxAgent{}
      metrics, err := agent.CollectMetrics(ctx)
      assert.NoError(t, err)
      assert.Equal(t, 5, len(metrics))  // CPU, memory, connections, requests/sec, latency
  }

  // Configuration enforcement test
  func TestAgentEnforceConfig(t *testing.T) {
      config := &Config{Version: "1.0.0", ...}
      err := agent.EnforceConfig(config)
      assert.NoError(t, err)
  }
  ```

- **Integration Test**: Full agent lifecycle with mock server (test register → collect → enforce → shutdown)

## Follow-Up ADRs

None (completes core engine framework decisions).

## References

- Spec-001, FR-020, User Story 4, SC-009
- Constitution Principle IV (Smoke tests), Principle II (Observability)
- gRPC Go/Python/Rust Client Libraries
- Similar frameworks (Telegraf plugin system, Fluent Bit plugins)

---

**Decision Date**: 2026-02-18
**Status Update**: Proposed (pending scaffold implementation review)
