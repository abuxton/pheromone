# ADR 006: Agent Lifecycle Interface - Agentic AI Agent Framework

## Status
Proposed

## Context

Pheromone agents are **agentic AI-capable agents** (see ADR-007) — autonomous processes running on each managed
instance. They must enable developers to write custom agents (Python, Go, Rust) that integrate with the platform
without deep framework knowledge (User Story 4, FR-020). The lifecycle interface defines what hooks agents must
implement, what the framework provides, and how the agent's AI reasoning loop interacts with its skills—
particularly the **Digital Twin Skill** for reading, updating, and applying the twin model to the system under
management.

**Requirements**:
- **Minimal Dependencies**: Custom agents should work with standard libraries (no forced frameworks)
- **Language-Agnostic**: Interface contracts defined via gRPC (ADR-003); any language can implement
- **Automatic Registration**: Agent framework handles gRPC setup and capability advertisement; developer writes business logic only
- **Built-in Observability**: Structured logs, metrics, and AI decision traces exported automatically (Principle II)
- **Clear Error Handling**: Graceful degradation on configuration mismatches or transient failures
- **Agentic AI Loop**: Framework provides a reasoning loop harness; developer provides skill implementations and optionally a custom reasoner

**Options Considered**:
- Full framework (like Telegraf plugins): Too opinionated; loses flexibility
- Minimal interface (pure gRPC): Requires developer to handle connection, retry, serialization logic
- Scaffolding + templating: Balanced approach; provides structure without rigidity
- **Agentic skill framework (selected)**: Extends scaffolding model with an explicit AI reasoning loop and skill interface; digital twin is a first-class skill

## Decision

**Define Agentic AI Agent Lifecycle Hooks + Provide Language-Specific Scaffolding**

The agent lifecycle is structured around an AI reasoning loop with discrete skills. The **Digital Twin Skill**
is the canonical way agents interact with their twin model—it is not a passive data sink but an active
capability the reasoning loop invokes.

### Agent Lifecycle Hooks (Pseudo-Code)

```
┌─────────────────────────────────────────────┐
│ 1. Initialize()                             │
│    - Load config                            │
│    - Set up local resources (files, db)     │
│    - Instantiate Skills (DigitalTwin,       │
│      MetricsCollector, ConfigEnforcer, ...) │
│    - Return: OS/Workload Twin objects       │
└──────────────┬──────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 2. Connect(server_address, agent_id)        │
│    - Establish gRPC connection to server    │
│    - Advertise capabilities & AI skills     │
│    - Register twin objects                  │
│    - Start heartbeat loop (FR-006)          │
│    - Return: gRPC client connection         │
└──────────────┬───────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 3. ReasoningLoop() [Continuous]             │
│    - Observe: CollectMetrics() +            │
│      DigitalTwinSkill.ReadTwin()            │
│    - Plan: compute drift & decide actions   │
│    - Act: invoke Skills (enforce, update)   │
│    - Reflect: update twin, emit traces      │
│    - ProposeAction() for uncertain cases    │
└──────────────┬───────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 4. CollectMetrics() [within Reasoning Loop] │
│    - Poll local system (CPU, memory, etc.)  │
│    - Format as gRPC Metric messages         │
│    - Stream to server via TelemetryStream   │
│    - Emit structured logs (trace-id tags)   │
└──────────────┬───────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 5. EnforceConfig(desired_config) [Async]    │
│    - Apply configuration from server        │
│    - Validate against actual state          │
│    - Report enforcement status back         │
│    - Called by reasoning loop via Skill     │
│    - Return: success/partial/failed         │
└──────────────┬───────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 6. HandleConfigUpdate(new_model_version)    │
│    - Called when server pushes new twin     │
│    - DigitalTwinSkill.UpdateTwin() invoked  │
│    - May trigger re-planning in loop        │
│    - Handle backward compatibility          │
└──────────────┬───────────────────────────────┘
               ↓
┌──────────────────────────────────────────────┐
│ 7. Shutdown() [On SIGTERM/disconnect]       │
│    - Flush pending logs/metrics/AI traces   │
│    - Clean up local resources               │
│    - Close gRPC connection gracefully       │
│    - Return: success or error               │
└──────────────────────────────────────────────┘
```

### Language-Specific Scaffolding

#### Go Agent Template

```go
// skills.go - Digital Twin Skill interface
type DigitalTwinSkill interface {
    ReadTwin(ctx context.Context, twinID string) (*TwinModel, error)
    UpdateTwin(ctx context.Context, twinID string, delta *TwinDelta) error
    ApplyModel(ctx context.Context, model *TwinModel) (*ApplyResult, error)
    DiffModel(desired, actual *TwinModel) (*DriftReport, error)
}

// agent.go - User implements these methods (business logic only)
type Agent interface {
    Initialize() ([]Twin, error)
    CollectMetrics(ctx context.Context) ([]Metric, error)
    EnforceConfig(cfg Config) error
    Shutdown() error
    // Optional: override default rule-based reasoner
    Reason(ctx context.Context, obs Observations, skills AgentSkills) ([]Action, error)
}

// framework.go - Framework provides this (AI loop + skill wiring)
type AgentFramework struct {
    client    grpc.TelemetryStreamClient
    twinSkill DigitalTwinSkill
    ticker    *time.Ticker
    logger    *logrus.Logger
}

func (f *AgentFramework) Run(agent Agent) {
    twins, _ := agent.Initialize()
    f.register(twins, f.capabilities())  // advertises AI skills to server

    for {
        select {
        case <-f.ticker.C:
            obs := f.observe(agent)                   // collect + read twins
            actions, _ := agent.Reason(ctx, obs, f.skills)
            for _, action := range actions {
                f.executeAction(agent, action)        // invoke skill, emit trace
            }
        case config := <-f.configChan:
            f.twinSkill.UpdateTwin(ctx, config.TwinID, config.Delta)
            agent.EnforceConfig(config)
        case <-f.shutdownChan:
            agent.Shutdown()
            return
        }
    }
}

// User code: ~50 lines (implements business logic; framework handles AI loop)
func main() {
    agent := &MyAgent{...}
    framework := NewAgentFramework("http://server:5050")
    framework.Run(agent)
}
```

#### Python Agent Template

```python
# skills.py - Digital Twin Skill interface
class DigitalTwinSkill(ABC):
    @abstractmethod
    def read_twin(self, twin_id: str) -> TwinModel: ...

    @abstractmethod
    def update_twin(self, twin_id: str, delta: TwinDelta) -> None: ...

    @abstractmethod
    def apply_model(self, model: TwinModel) -> ApplyResult: ...

    @abstractmethod
    def diff_model(self, desired: TwinModel, actual: TwinModel) -> DriftReport: ...

# agent.py - User implements these methods
class Agent(ABC):
    @abstractmethod
    def initialize(self) -> tuple[list[Twin], None]: ...

    @abstractmethod
    def collect_metrics(self) -> list[Metric]: ...

    @abstractmethod
    def enforce_config(self, cfg: Config) -> bool: ...

    # Optional: override default rule-based reasoner
    def reason(self, observations: Observations, skills: AgentSkills) -> list[Action]:
        return skills.twin.diff_model(observations.desired, observations.actual).to_actions()

# framework.py - Framework provides this (AI loop + skill wiring)
class AgentFramework:
    def __init__(self, server_address: str):
        self.stub = TelemetryStreamStub(server_address)
        self.twin_skill = DigitalTwinSkillImpl(self.stub)
        self.logger = logging.getLogger("pheromone-agent")

    def run(self, agent: Agent):
        twins, _ = agent.initialize()
        self.register(twins, self.capabilities())   # advertise AI skills

        while True:
            obs = self._observe(agent)              # collect + read twins
            actions = agent.reason(obs, self.skills)
            for action in actions:
                self._execute_action(agent, action)  # invoke skill, emit trace
            sleep(5)

# User code: ~30 lines
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

- **AI Decision Traces** (structured JSON per reasoning cycle)
  ```json
  {
    "timestamp": "2026-02-20T10:00:00Z",
    "trace_id": "xyz-789",
    "agent_id": "agent-1",
    "component": "reasoning-loop",
    "level": "info",
    "message": "reasoning cycle complete",
    "observations_count": 15,
    "drift_detected": true,
    "actions_planned": 2,
    "actions_executed": 2,
    "actions_proposed_to_server": 0,
    "twin_updates": ["os-twin-host-1"]
  }
  ```

Framework handles metrics aggregation and export (agents just call `AddMetric()`).
AI decision traces are auto-emitted by the framework reasoning loop harness.

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
- **Observability**: Structured logs, metrics, and AI decision traces automatic (no developer work)
- **Type Safety**: Framework handles gRPC serialization/deserialization (developer works with Go/Python native types)
- **Testability**: Agents can be unit-tested without server (mock Twin/Config/Skill objects)
- **Agentic AI Ready**: Skill interface enables plug-in of AI reasoning engines (LLM, rules, hybrid) without changing lifecycle hooks

### Negative
- **Framework Complexity**: Core team must maintain Go, Python, Rust scaffold versions plus DigitalTwinSkill implementations
- **Language Support**: Only actively maintained languages get scaffold (not JavaScript, PHP, etc.)
- **Debugging**: Framework abstraction makes troubleshooting harder if logic buried in framework
- **Performance**: Abstraction layers have overhead (acceptable for <10K agents; may need optimization for Phase 2)
- **AI Reasoning Overhead**: Agents hosting AI reasoning loops require more resources than pure daemons; edge devices may need rule-based fallback

### Implementation Effort
- **Go Scaffold**: ~300 lines (core framework + AI loop harness + DigitalTwinSkill + examples)
- **Python Scaffold**: ~350 lines (gRPC client boilerplate more verbose; skill interfaces)
- **Rust Scaffold**: ~400 lines (type system more verbose; safer AI trait boundaries)
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

  // Digital Twin Skill test
  func TestDigitalTwinSkillDiff(t *testing.T) {
      skill := NewDigitalTwinSkill(mockTwinStore)
      desired := &TwinModel{Packages: []string{"nginx=1.24"}}
      actual  := &TwinModel{Packages: []string{"nginx=1.20"}}
      drift   := skill.DiffModel(desired, actual)
      assert.Len(t, drift.PackageDrift, 1)
      assert.Equal(t, "nginx", drift.PackageDrift[0].Name)
  }

  // Reasoning loop test (mock reasoner)
  func TestAgentReasoningLoop(t *testing.T) {
      framework := NewAgentFramework(mockServer, mockReasoner)
      agent := &NginxAgent{}
      // Run one tick; verify actions planned and skills invoked
      actions := framework.RunOneTick(ctx, agent)
      assert.NotEmpty(t, actions)
  }
  ```

- **Integration Test**: Full agent lifecycle with mock server (test register → reason → collect → enforce → shutdown)
- **AI Decision Trace Test**: Verify trace emitted per reasoning cycle with correct structure

## Skill-Centric Revision (2026-03-05)

### Should this ADR address Skills, not Independent Agents?

> **Reviewed by @copilot on 2026-03-05 per agent instructions on this issue.**

**Yes.** Given ADR-007 (Accepted) establishes that agents are agentic AI-capable processes with
skill-based capabilities, ADR-006 should be re-framed as a **Skill Deployment and Distribution
Framework**, not an independent-agent scaffold. Key reasons:

1. **Skills are the unit of extension.** Developers write a `Skill` implementation (~50 lines),
   not a full agent. The framework handles the reasoning loop, gRPC transport, and observability.

2. **Server-distributed skill bundles.** The server holds a canonical `SkillRegistry` and
   distributes a filtered skill bundle to each agent at registration time — only the skills the
   agent is permitted to invoke based on its twin-level access policy.

3. **Hierarchical access control across twin layers.** The layered twin architecture (Principle I)
   requires that skills be gated by twin-level priority:

   ```
   TwinLevelOS (0) — highest priority; may invoke all skills, including OS-restricted ones
   TwinLevelWorkload (1) — lower priority; may only invoke skills with MinTwinLevel = Workload
   ```

   An OS-level actor may explicitly grant a workload agent access to a restricted skill via
   `AccessPolicy.Grant(agentID, skillName)`. This provides fine-grained capability segmentation:
   OS twins own the authoritative DigitalTwinSkill; workload twins get MetricsCollection and
   ConfigEnforce by default.

4. **Blocking lower-priority twins.** A workload twin cannot access skills or artifacts that
   require OS-level authority. The `SkillRegistry.BundleFor(agentID, twinLevels)` call enforces
   this at distribution time — workload agents simply never receive restricted skills.

### Revised Go Skill Framework (implemented in `internal/skill/`)

```go
// Skill is the interface every Pheromone skill must satisfy (~50 lines to implement).
type Skill interface {
    Name() string
    Version() string
    MinTwinLevel() TwinLevel  // access control boundary
    Execute(ctx context.Context, obs *Observations, action *Action) (*SkillResult, error)
}

// SkillRegistry holds server-side skills and distributes filtered bundles to agents.
type SkillRegistry struct { ... }
func (r *SkillRegistry) BundleFor(agentID string, twinLevels []TwinLevel) []Skill { ... }

// AccessPolicy enforces hierarchical access control.
type AccessPolicy struct { ... }
func (p *AccessPolicy) Check(agentID string, agentLevels []TwinLevel, s Skill) error { ... }
func (p *AccessPolicy) Grant(agentID, skillName string) { ... }  // OS-level override

// AgentFramework is the reasoning loop harness wired to a SkillRegistry.
func NewAgentFramework(agentID string, registry *SkillRegistry, opts ...FrameworkOption) *AgentFramework
func (f *AgentFramework) Run(ctx context.Context, agent Agent) error
func (f *AgentFramework) RunOneTick(ctx context.Context, agent Agent) ([]Action, error)
```

### Built-in Skills

| Skill | MinTwinLevel | Purpose |
|-------|-------------|---------|
| `digital-twin` | `TwinLevelOS` | ReadTwin, UpdateTwin, DiffModel, ApplyModel — OS agents only |
| `metrics` | `TwinLevelWorkload` | Collect OS and workload metrics |
| `config-enforce` | `TwinLevelWorkload` | Apply desired config, validate compliance, rollback |

### Custom Skill Example (developer effort: ~50 lines)

```go
// internal/skill/example/nginx.go — NginxMonitorSkill
type NginxMonitorSkill struct{}

func (s *NginxMonitorSkill) Name() string                 { return "nginx-monitor" }
func (s *NginxMonitorSkill) Version() string              { return "1.0.0" }
func (s *NginxMonitorSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelWorkload }

func (s *NginxMonitorSkill) Execute(ctx context.Context, obs *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
    // business logic only (~40 lines): collect /nginx_status, return metrics
}
```

### Skill Distribution Flow

```
Server SkillRegistry (canonical)
  ├─ digital-twin  (MinTwinLevel=OS)
  ├─ metrics       (MinTwinLevel=Workload)
  └─ config-enforce (MinTwinLevel=Workload)

Agent registration → BundleFor(agentID, twinLevels):
  OS agent    → [digital-twin, metrics, config-enforce]
  Workload agent → [metrics, config-enforce]
  Workload agent + explicit grant → [digital-twin, metrics, config-enforce]
```

## Follow-Up ADRs

- **ADR-007**: Agentic AI Agent Model (establishes the agent-as-AI-capable-agent canonical model; digital twin as skill) — **Accepted**
- **ADR-008** (Proposed): AI model selection and local vs. remote reasoning engine trade-offs
- **ADR-009** (Proposed): Action approval workflow and human-in-the-loop gate design

## References

- Spec-001, FR-020, User Story 4, SC-009
- Constitution Principle I (Layered Twin Architecture), Principle II (Observability), Principle IV (Smoke tests)
- ADR-007 (Agentic AI Agent Model — this ADR's context)
- `internal/skill/` — Go skill framework implementation
- `internal/skill/builtin/` — DigitalTwinSkill, MetricsCollectionSkill, ConfigEnforceSkill
- `internal/skill/example/` — NginxMonitorSkill, PostgreSQLMonitorSkill

---

**Decision Date**: 2026-02-18
**Status Update**: Proposed (pending scaffold implementation review)
**Updated**: 2026-02-20 — Revised to reflect agentic AI agent model (ADR-007); Digital Twin Skill added to lifecycle
**Updated**: 2026-03-05 — Skill-centric revision: framework re-framed as skill deployment and distribution with hierarchical twin-level access control; Go implementation in `internal/skill/`
