package example

import (
	"context"
	"fmt"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// NginxMonitorSkill monitors an Nginx web server workload twin.
// It collects requests/sec, active connections, and p99 latency.
// Available to workload-level agents (TwinLevelWorkload).
//
// Developer effort: ~50 lines of business logic; the framework handles
// the reasoning loop, gRPC transport, and observability automatically.
type NginxMonitorSkill struct {
	// stubCollector is injected in tests to replace real /nginx_status reads.
	stubCollector func() (float64, float64, float64, error)
}

// NewNginxMonitorSkill creates a production NginxMonitorSkill.
func NewNginxMonitorSkill() *NginxMonitorSkill {
	s := &NginxMonitorSkill{}
	s.stubCollector = s.collectFromStubServer
	return s
}

func (s *NginxMonitorSkill) Name() string                 { return "nginx-monitor" }
func (s *NginxMonitorSkill) Version() string              { return "1.0.0" }
func (s *NginxMonitorSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelWorkload }

// Execute collects Nginx metrics and returns them as a SkillResult.
// The only supported action type is "collect-metrics".
func (s *NginxMonitorSkill) Execute(_ context.Context, _ *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	if action.ActionType != "collect-metrics" {
		return nil, fmt.Errorf("nginx-monitor: unknown action type %q", action.ActionType)
	}

	reqPerSec, activeConns, latencyMs, err := s.stubCollector()
	if err != nil {
		return nil, fmt.Errorf("nginx-monitor: collection failed: %w", err)
	}

	agentID := action.Params["agent_id"]
	labels := map[string]string{
		"agent_id": agentID,
		"twin_id":  action.TwinID,
		"service":  "nginx",
	}
	now := time.Now()

	metrics := []skill.Metric{
		{Name: "pheromone_nginx_requests_per_sec", Value: reqPerSec, Labels: labels, Timestamp: now},
		{Name: "pheromone_nginx_active_connections", Value: activeConns, Labels: labels, Timestamp: now},
		{Name: "pheromone_nginx_latency_p99_ms", Value: latencyMs, Labels: labels, Timestamp: now},
	}

	return &skill.SkillResult{
		Success: true,
		Metrics: metrics,
		Detail:  fmt.Sprintf("nginx: %.1f req/s, %.0f conns, %.1f ms p99", reqPerSec, activeConns, latencyMs),
	}, nil
}

// collectFromStubServer returns synthetic values for Phase 1 (real impl reads /nginx_status).
func (s *NginxMonitorSkill) collectFromStubServer() (float64, float64, float64, error) {
	return 120.5, 42, 18.3, nil
}
