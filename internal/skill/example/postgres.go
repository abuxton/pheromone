package example

import (
	"context"
	"fmt"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// PostgreSQLMonitorSkill monitors a PostgreSQL workload twin.
// It collects active connections and queries/sec.
// Available to workload-level agents (TwinLevelWorkload).
//
// Developer effort: ~50 lines of business logic; the framework handles
// the reasoning loop, gRPC transport, and observability automatically.
type PostgreSQLMonitorSkill struct {
	// stubCollector is injected in tests to replace real pg_stat queries.
	stubCollector func() (float64, float64, error)
}

// NewPostgreSQLMonitorSkill creates a production PostgreSQLMonitorSkill.
func NewPostgreSQLMonitorSkill() *PostgreSQLMonitorSkill {
	s := &PostgreSQLMonitorSkill{}
	s.stubCollector = s.collectFromStubDB
	return s
}

func (s *PostgreSQLMonitorSkill) Name() string                 { return "postgresql-monitor" }
func (s *PostgreSQLMonitorSkill) Version() string              { return "1.0.0" }
func (s *PostgreSQLMonitorSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelWorkload }

// Execute collects PostgreSQL metrics and returns them as a SkillResult.
// The only supported action type is "collect-metrics".
func (s *PostgreSQLMonitorSkill) Execute(_ context.Context, _ *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	if action.ActionType != "collect-metrics" {
		return nil, fmt.Errorf("postgresql-monitor: unknown action type %q", action.ActionType)
	}

	activeConns, queriesPerSec, err := s.stubCollector()
	if err != nil {
		return nil, fmt.Errorf("postgresql-monitor: collection failed: %w", err)
	}

	agentID := action.Params["agent_id"]
	labels := map[string]string{
		"agent_id": agentID,
		"twin_id":  action.TwinID,
		"service":  "postgresql",
	}
	now := time.Now()

	metrics := []skill.Metric{
		{Name: "pheromone_postgresql_active_connections", Value: activeConns, Labels: labels, Timestamp: now},
		{Name: "pheromone_postgresql_queries_per_sec", Value: queriesPerSec, Labels: labels, Timestamp: now},
	}

	return &skill.SkillResult{
		Success: true,
		Metrics: metrics,
		Detail:  fmt.Sprintf("postgresql: %.0f conns, %.1f qps", activeConns, queriesPerSec),
	}, nil
}

// collectFromStubDB returns synthetic values for Phase 1 (real impl queries pg_stat_activity).
func (s *PostgreSQLMonitorSkill) collectFromStubDB() (float64, float64, error) {
	return 15, 340.7, nil
}
