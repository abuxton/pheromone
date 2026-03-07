package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// MetricsCollectionSkill collects OS and workload metrics and formats them for
// the TelemetryStream. It is available to all twin levels (TwinLevelWorkload).
type MetricsCollectionSkill struct{}

// NewMetricsCollectionSkill creates a MetricsCollectionSkill.
func NewMetricsCollectionSkill() *MetricsCollectionSkill { return &MetricsCollectionSkill{} }

func (s *MetricsCollectionSkill) Name() string                 { return "metrics" }
func (s *MetricsCollectionSkill) Version() string              { return "1.0.0" }
func (s *MetricsCollectionSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelWorkload }

// Execute runs the metrics collection action.
// Supported action types: "collect-os", "collect-workload".
func (s *MetricsCollectionSkill) Execute(_ context.Context, _ *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	now := time.Now()
	agentID := action.Params["agent_id"]
	if agentID == "" {
		agentID = "unknown"
	}

	var metrics []skill.Metric
	switch action.ActionType {
	case "collect-os":
		metrics = collectOSMetrics(agentID, now)
	case "collect-workload":
		metrics = collectWorkloadMetrics(agentID, action.TwinID, now)
	default:
		return nil, fmt.Errorf("metrics: unknown action type %q", action.ActionType)
	}

	return &skill.SkillResult{
		Success: true,
		Metrics: metrics,
		Detail:  fmt.Sprintf("collected %d metrics", len(metrics)),
	}, nil
}

// collectOSMetrics returns synthetic OS-level metrics.
// TODO(Phase 2): replace stub values with real /proc, cgroups, and sysfs reads.
func collectOSMetrics(agentID string, ts time.Time) []skill.Metric {
	labels := map[string]string{"agent_id": agentID}
	return []skill.Metric{
		{Name: "pheromone_os_cpu_usage_percent", Value: 5.0, Labels: labels, Timestamp: ts},
		{Name: "pheromone_os_memory_used_bytes", Value: 512 * 1024 * 1024, Labels: labels, Timestamp: ts},
		{Name: "pheromone_os_disk_read_bytes_total", Value: 1024 * 1024, Labels: labels, Timestamp: ts},
		{Name: "pheromone_os_disk_write_bytes_total", Value: 256 * 1024, Labels: labels, Timestamp: ts},
	}
}

// collectWorkloadMetrics returns synthetic workload-level metrics.
// TODO(Phase 2): replace stub values with cgroup/container runtime reads.
func collectWorkloadMetrics(agentID, twinID string, ts time.Time) []skill.Metric {
	labels := map[string]string{"agent_id": agentID, "twin_id": twinID}
	return []skill.Metric{
		{Name: "pheromone_workload_cpu_usage_percent", Value: 2.5, Labels: labels, Timestamp: ts},
		{Name: "pheromone_workload_memory_used_bytes", Value: 128 * 1024 * 1024, Labels: labels, Timestamp: ts},
	}
}
