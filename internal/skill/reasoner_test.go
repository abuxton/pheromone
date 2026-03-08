package skill_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// -----------------------------------------------------------------------------
// Unit tests for RuleBasedReasoner
// -----------------------------------------------------------------------------

func TestRuleBasedReasoner_NoDriftReturnsNoActions(t *testing.T) {
	r := skill.NewRuleBasedReasoner()
	obs := &skill.Observations{}
	goal := skill.GoalState{Twins: map[string]*skill.TwinModel{
		"twin-1": {TwinID: "twin-1", Version: "1.0.0", State: map[string]string{"nginx_version": "1.24"}},
	}}
	drift := skill.DriftReport{TwinID: "twin-1", HasDrift: false}

	actions, err := r.Plan(context.Background(), obs, goal, drift)
	if err != nil {
		t.Fatalf("Plan returned unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected no actions when there is no drift, got %d", len(actions))
	}
}

func TestRuleBasedReasoner_DriftProducesConfigEnforceAction(t *testing.T) {
	r := skill.NewRuleBasedReasoner()
	obs := &skill.Observations{}
	goal := skill.GoalState{Twins: map[string]*skill.TwinModel{
		"twin-1": {TwinID: "twin-1", Version: "1.0.0", State: map[string]string{"nginx_version": "1.24"}},
	}}
	drift := skill.DriftReport{
		TwinID:   "twin-1",
		HasDrift: true,
		DriftedFields: []skill.FieldDrift{
			{Field: "nginx_version", Desired: "1.24", Actual: "1.20"},
		},
	}

	actions, err := r.Plan(context.Background(), obs, goal, drift)
	if err != nil {
		t.Fatalf("Plan returned unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action on drift, got %d", len(actions))
	}
	a := actions[0]
	if a.SkillName != "config-enforce" {
		t.Errorf("expected skill 'config-enforce', got %q", a.SkillName)
	}
	if a.ActionType != "apply-config" {
		t.Errorf("expected action type 'apply-config', got %q", a.ActionType)
	}
	if a.TwinID != "twin-1" {
		t.Errorf("expected TwinID 'twin-1', got %q", a.TwinID)
	}
	if a.Params["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0' in params, got %q", a.Params["version"])
	}
	if a.Rationale == "" {
		t.Error("expected a non-empty rationale")
	}
}

func TestRuleBasedReasoner_DriftWithMissingGoalVersionUsesEmpty(t *testing.T) {
	r := skill.NewRuleBasedReasoner()
	obs := &skill.Observations{}
	// GoalState has no entry for the drifted twin — version should default to ""
	goal := skill.GoalState{Twins: map[string]*skill.TwinModel{}}
	drift := skill.DriftReport{
		TwinID:   "twin-unknown",
		HasDrift: true,
		DriftedFields: []skill.FieldDrift{
			{Field: "sshd_version", Desired: "9.0", Actual: "8.9"},
		},
	}

	actions, err := r.Plan(context.Background(), obs, goal, drift)
	if err != nil {
		t.Fatalf("Plan returned unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Params["version"] != "" {
		t.Errorf("expected empty version when goal unknown, got %q", actions[0].Params["version"])
	}
}

func TestRuleBasedReasoner_MultipleDriftedFieldsProducesSingleAction(t *testing.T) {
	r := skill.NewRuleBasedReasoner()
	obs := &skill.Observations{}
	goal := skill.GoalState{Twins: map[string]*skill.TwinModel{
		"twin-2": {TwinID: "twin-2", Version: "2.0.0"},
	}}
	drift := skill.DriftReport{
		TwinID:   "twin-2",
		HasDrift: true,
		DriftedFields: []skill.FieldDrift{
			{Field: "cpu_governor", Desired: "performance", Actual: "powersave"},
			{Field: "swap_enabled", Desired: "false", Actual: "true"},
			{Field: "kernel_version", Desired: "6.8", Actual: "6.5"},
		},
	}

	actions, err := r.Plan(context.Background(), obs, goal, drift)
	if err != nil {
		t.Fatalf("Plan returned unexpected error: %v", err)
	}
	// Rule-based reasoner plans one consolidated config-enforce action per drift report
	if len(actions) != 1 {
		t.Errorf("expected 1 action for multi-field drift, got %d", len(actions))
	}
}

func TestRuleBasedReasoner_ContextCancellationRespected(t *testing.T) {
	r := skill.NewRuleBasedReasoner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	obs := &skill.Observations{}
	goal := skill.GoalState{Twins: map[string]*skill.TwinModel{}}
	drift := skill.DriftReport{TwinID: "twin-1", HasDrift: false}

	// RuleBasedReasoner is synchronous and completes before checking ctx; it must not error.
	_, err := r.Plan(ctx, obs, goal, drift)
	if err != nil {
		t.Errorf("RuleBasedReasoner should complete even with cancelled context: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Tech Spike: ADR-007 — Resource validation harness (100 reasoning cycles)
// -----------------------------------------------------------------------------

// TestRuleBasedReasoner_ResourceUsage_100Cycles is the ADR-007 tech-spike harness.
//
// It runs 100 reasoning cycles (simulating ~5 minutes at 3-second intervals)
// and validates that the RuleBasedReasoner stays within the resource targets
// defined by the spike:
//
//   - RAM overhead: < 50 MB (on a 1 GB instance)
//   - Average cycle duration: < 1 ms (implying << 5% CPU on a 2-core instance)
//
// Results are reported via t.Logf so they appear in verbose test output and
// in CI artefacts.
func TestRuleBasedReasoner_ResourceUsage_100Cycles(t *testing.T) {
	const (
		cycles         = 100
		maxRAMDeltaMiB = 50                    // success criterion from ADR-007 spike
		maxCycleMs     = 1                     // well within 5% CPU on a 2-core instance
		twinCount      = 10                    // 10 managed twins per agent
		metricCount    = 20                    // 20 metrics per observation
		fieldCount     = 5                     // 5 state fields per twin
	)

	r := skill.NewRuleBasedReasoner()
	ctx := context.Background()

	// Build a representative goal state with twinCount twins.
	goal := skill.GoalState{Twins: make(map[string]*skill.TwinModel, twinCount)}
	for i := 0; i < twinCount; i++ {
		id := twinID(i)
		goal.Twins[id] = &skill.TwinModel{
			TwinID:  id,
			Level:   skill.TwinLevelOS,
			Version: "1.0.0",
			State:   desiredState(fieldCount),
		}
	}

	// Build representative observations.
	obs := buildObservations(twinCount, metricCount, fieldCount)

	// Build a representative drift report (alternate between drift / no drift).
	driftReports := make([]skill.DriftReport, twinCount)
	for i := 0; i < twinCount; i++ {
		id := twinID(i)
		if i%2 == 0 {
			driftReports[i] = skill.DriftReport{
				TwinID:   id,
				HasDrift: true,
				DriftedFields: []skill.FieldDrift{
					{Field: "nginx_version", Desired: "1.24", Actual: "1.20"},
				},
			}
		} else {
			driftReports[i] = skill.DriftReport{TwinID: id, HasDrift: false}
		}
	}

	// --- Force a GC before sampling baseline memory. ---
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	totalDuration := time.Duration(0)

	// Run 100 cycles; each cycle reasons over all twinCount twins.
	for cycle := 0; cycle < cycles; cycle++ {
		cycleStart := time.Now()
		for i := 0; i < twinCount; i++ {
			_, err := r.Plan(ctx, obs, goal, driftReports[i])
			if err != nil {
				t.Fatalf("cycle %d twin %d: Plan returned error: %v", cycle, i, err)
			}
		}
		totalDuration += time.Since(cycleStart)
	}

	// --- Sample post-run memory. ---
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Compute metrics.
	heapDeltaBytes := int64(memAfter.HeapInuse) - int64(memBefore.HeapInuse)
	heapDeltaMiB := float64(heapDeltaBytes) / (1024 * 1024)
	if heapDeltaBytes < 0 {
		heapDeltaMiB = 0 // GC may have freed more than allocated — treat as 0 growth
	}

	avgCycleMs := totalDuration.Milliseconds() / int64(cycles)
	avgCycleUs := totalDuration.Microseconds() / int64(cycles)
	totalPlans := int64(cycles * twinCount)
	avgPlanNs := totalDuration.Nanoseconds() / totalPlans

	t.Logf("ADR-007 Tech Spike — RuleBasedReasoner resource validation")
	t.Logf("  cycles          : %d (× %d twins = %d Plan() calls)", cycles, twinCount, totalPlans)
	t.Logf("  total duration  : %v", totalDuration)
	t.Logf("  avg cycle       : %d ms (%d µs per-twin)", avgCycleMs, avgCycleUs/int64(twinCount))
	t.Logf("  avg per Plan()  : %d ns", avgPlanNs)
	t.Logf("  heap before     : %d KiB", memBefore.HeapInuse/1024)
	t.Logf("  heap after      : %d KiB", memAfter.HeapInuse/1024)
	t.Logf("  heap Δ          : %.3f MiB", heapDeltaMiB)
	t.Logf("  total alloc     : %d KiB", (memAfter.TotalAlloc-memBefore.TotalAlloc)/1024)

	// --- Validate success criteria (ADR-007 spike). ---
	if heapDeltaMiB >= maxRAMDeltaMiB {
		t.Errorf("FAIL: heap growth %.3f MiB >= limit %d MiB (ADR-007 RAM target violated)",
			heapDeltaMiB, maxRAMDeltaMiB)
	} else {
		t.Logf("PASS: heap growth %.3f MiB < %d MiB RAM target", heapDeltaMiB, maxRAMDeltaMiB)
	}

	perPlanMs := totalDuration.Milliseconds() / totalPlans
	if perPlanMs >= maxCycleMs {
		t.Errorf("FAIL: avg Plan() duration %d ms >= %d ms (ADR-007 CPU target violated)",
			perPlanMs, maxCycleMs)
	} else {
		t.Logf("PASS: avg Plan() duration %d ms < %d ms CPU target", perPlanMs, maxCycleMs)
	}
}

// TestRuleBasedReasoner_MetricThroughput confirms the reasoner does not degrade
// metric collection throughput (ADR-007 spike criterion 3).
//
// It verifies that reasoning cycles interleaved with metric collection calls complete
// in a time consistent with the baseline (no significant overhead added by Plan()).
func TestRuleBasedReasoner_MetricThroughput(t *testing.T) {
	const (
		cycles     = 100
		metricCount = 50
		twinCount  = 5
	)

	r := skill.NewRuleBasedReasoner()
	ctx := context.Background()

	goal := skill.GoalState{Twins: make(map[string]*skill.TwinModel, twinCount)}
	for i := 0; i < twinCount; i++ {
		id := twinID(i)
		goal.Twins[id] = &skill.TwinModel{TwinID: id, Version: "1.0.0"}
	}
	obs := buildObservations(twinCount, metricCount, 3)
	drift := skill.DriftReport{TwinID: twinID(0), HasDrift: true,
		DriftedFields: []skill.FieldDrift{{Field: "f", Desired: "d", Actual: "a"}},
	}

	start := time.Now()
	for cycle := 0; cycle < cycles; cycle++ {
		// Simulate metric collection by iterating observations (no I/O, CPU-only).
		for _, m := range obs.Metrics {
			_ = m.Value // access value to prevent optimisation away
		}
		// Run Plan for one twin per cycle.
		if _, err := r.Plan(ctx, obs, goal, drift); err != nil {
			t.Fatalf("Plan error at cycle %d: %v", cycle, err)
		}
	}
	elapsed := time.Since(start)

	t.Logf("MetricThroughput: %d cycles (metrics=%d, twins=%d) in %v (avg %v/cycle)",
		cycles, metricCount, twinCount, elapsed, elapsed/cycles)

	// Throughput criterion: all 100 cycles including metric iteration must complete in < 100 ms
	// (consistent with the 1 ms per-cycle target from ADR-007 plus reasonable headroom for
	// metric iteration overhead).
	if elapsed > 100*time.Millisecond {
		t.Errorf("metric+reasoning cycles took %v, expected < 100ms (throughput degradation detected)", elapsed)
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func twinID(i int) string {
	return fmt.Sprintf("twin-%d", i)
}

func desiredState(fields int) map[string]string {
	m := make(map[string]string, fields)
	for i := 0; i < fields; i++ {
		key := fmt.Sprintf("field_%d", i)
		m[key] = "desired_value"
	}
	return m
}

func buildObservations(twinCount, metricCount, fieldCount int) *skill.Observations {
	metrics := make([]skill.Metric, metricCount)
	for i := range metrics {
		metrics[i] = skill.Metric{
			Name:  "cpu_usage",
			Value: float64(i) * 0.5,
		}
	}

	twins := make([]*skill.TwinModel, twinCount)
	desired := make([]*skill.TwinModel, twinCount)
	for i := 0; i < twinCount; i++ {
		id := twinID(i)
		state := make(map[string]string, fieldCount)
		for j := 0; j < fieldCount; j++ {
			state[fmt.Sprintf("field_%d", j)] = "actual_value"
		}
		twins[i] = &skill.TwinModel{TwinID: id, Level: skill.TwinLevelOS, State: state}
		desired[i] = &skill.TwinModel{TwinID: id, Level: skill.TwinLevelOS, Version: "1.0.0", State: desiredState(fieldCount)}
	}

	return &skill.Observations{
		Metrics:      metrics,
		Twins:        twins,
		DesiredTwins: desired,
	}
}
