package metrics_test

import (
"testing"

"github.com/abuxton/pheromone/internal/metrics"
dto "github.com/prometheus/client_model/go"
)

// TestNewRegistryNoDuplicatePanic verifies that calling NewRegistry multiple
// times does not panic, and each call produces an independent registry.
func TestNewRegistryNoDuplicatePanic(t *testing.T) {
t.Parallel()
defer func() {
if r := recover(); r != nil {
t.Fatalf("NewRegistry panicked: %v", r)
}
}()
// Two independent calls must not interfere.
reg1, _ := metrics.NewRegistry()
reg2, _ := metrics.NewRegistry()
if reg1 == reg2 {
t.Error("expected two different registry instances")
}
}

// TestAllMetricsRegistered verifies that all six required metrics appear in a
// freshly created registry once at least one observation is recorded.
// Vec metrics only appear in Gather output once labelled time-series have been
// observed, so we seed one observation per metric family.
func TestAllMetricsRegistered(t *testing.T) {
t.Parallel()
reg, m := metrics.NewRegistry()

m.AgentsTotal.WithLabelValues("online").Set(1)
m.TwinsTotal.Set(1)
m.GRPCRequestsTotal.WithLabelValues("svc", "method", "OK").Add(1)
m.GRPCDurationSeconds.WithLabelValues("svc", "method").Observe(0.001)
m.NATSMessagesTotal.WithLabelValues("TELEMETRY").Add(1)
m.ReasoningDecisionsTotal.WithLabelValues("rule-based", "apply").Add(1)

families, err := reg.Gather()
if err != nil {
t.Fatalf("Gather: %v", err)
}

want := map[string]bool{
"pheromone_agents_total":              false,
"pheromone_twins_total":               false,
"pheromone_grpc_requests_total":       false,
"pheromone_grpc_duration_seconds":     false,
"pheromone_nats_messages_total":       false,
"pheromone_reasoning_decisions_total": false,
}
for _, fam := range families {
if fam.Name != nil {
want[*fam.Name] = true
}
}
for name, found := range want {
if !found {
t.Errorf("metric %q not found in registry after observation", name)
}
}
}

// TestMetricTypes verifies that each metric has the correct Prometheus type.
func TestMetricTypes(t *testing.T) {
t.Parallel()
reg, m := metrics.NewRegistry()

m.AgentsTotal.WithLabelValues("online").Set(3)
m.TwinsTotal.Set(4)
m.GRPCRequestsTotal.WithLabelValues("AgentRegistry", "Register", "OK").Add(1)
m.GRPCDurationSeconds.WithLabelValues("AgentRegistry", "Register").Observe(0.01)
m.NATSMessagesTotal.WithLabelValues("TELEMETRY").Add(5)
m.ReasoningDecisionsTotal.WithLabelValues("rule-based", "apply").Add(2)

families, err := reg.Gather()
if err != nil {
t.Fatalf("Gather: %v", err)
}

typeMap := map[string]dto.MetricType{}
for _, fam := range families {
if fam.Name != nil && fam.Type != nil {
typeMap[*fam.Name] = *fam.Type
}
}

cases := []struct {
name     string
wantType dto.MetricType
}{
{"pheromone_agents_total", dto.MetricType_GAUGE},
{"pheromone_twins_total", dto.MetricType_GAUGE},
{"pheromone_grpc_requests_total", dto.MetricType_COUNTER},
{"pheromone_grpc_duration_seconds", dto.MetricType_HISTOGRAM},
{"pheromone_nats_messages_total", dto.MetricType_COUNTER},
{"pheromone_reasoning_decisions_total", dto.MetricType_COUNTER},
}
for _, tc := range cases {
got, ok := typeMap[tc.name]
if !ok {
t.Errorf("metric %q not found in gathered families", tc.name)
continue
}
if got != tc.wantType {
t.Errorf("metric %q: got type %v, want %v", tc.name, got, tc.wantType)
}
}
}

// TestPackageLevelRegistryInit verifies the package-level registry is
// pre-populated without panicking (covers the init() path).
func TestPackageLevelRegistryInit(t *testing.T) {
t.Parallel()
families, err := metrics.Registry.Gather()
if err != nil {
t.Fatalf("package Registry.Gather: %v", err)
}
if len(families) == 0 {
t.Error("expected package Registry to contain at least one metric family")
}
}
