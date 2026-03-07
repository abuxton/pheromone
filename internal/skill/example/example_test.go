package example_test

import (
	"context"
	"testing"

	"github.com/abuxton/pheromone/internal/skill"
	"github.com/abuxton/pheromone/internal/skill/example"
)

func TestNginxMonitorSkill_CollectsMetrics(t *testing.T) {
	s := example.NewNginxMonitorSkill()
	obs := &skill.Observations{}
	action := &skill.Action{
		ActionType: "collect-metrics",
		TwinID:     "workload-nginx-1",
		Params:     map[string]string{"agent_id": "agent-1"},
	}

	result, err := s.Execute(context.Background(), obs, action)
	if err != nil {
		t.Fatalf("NginxMonitorSkill.Execute: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got: %s", result.Detail)
	}
	if len(result.Metrics) != 3 {
		t.Errorf("expected 3 metrics (req/s, conns, latency), got %d", len(result.Metrics))
	}

	names := make(map[string]bool)
	for _, m := range result.Metrics {
		names[m.Name] = true
		if m.Labels["agent_id"] != "agent-1" {
			t.Errorf("metric %s: expected agent_id=agent-1, got %s", m.Name, m.Labels["agent_id"])
		}
	}
	for _, want := range []string{
		"pheromone_nginx_requests_per_sec",
		"pheromone_nginx_active_connections",
		"pheromone_nginx_latency_p99_ms",
	} {
		if !names[want] {
			t.Errorf("missing expected metric: %s", want)
		}
	}
}

func TestNginxMonitorSkill_UnknownActionReturnsError(t *testing.T) {
	s := example.NewNginxMonitorSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "unknown",
	})
	if err == nil {
		t.Error("expected error for unknown action type")
	}
}

func TestNginxMonitorSkill_MinTwinLevel(t *testing.T) {
	s := example.NewNginxMonitorSkill()
	if s.MinTwinLevel() != skill.TwinLevelWorkload {
		t.Errorf("expected TwinLevelWorkload, got %s", s.MinTwinLevel())
	}
}

func TestPostgreSQLMonitorSkill_CollectsMetrics(t *testing.T) {
	s := example.NewPostgreSQLMonitorSkill()
	obs := &skill.Observations{}
	action := &skill.Action{
		ActionType: "collect-metrics",
		TwinID:     "workload-postgres-1",
		Params:     map[string]string{"agent_id": "agent-2"},
	}

	result, err := s.Execute(context.Background(), obs, action)
	if err != nil {
		t.Fatalf("PostgreSQLMonitorSkill.Execute: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got: %s", result.Detail)
	}
	if len(result.Metrics) != 2 {
		t.Errorf("expected 2 metrics (conns, qps), got %d", len(result.Metrics))
	}

	names := make(map[string]bool)
	for _, m := range result.Metrics {
		names[m.Name] = true
	}
	for _, want := range []string{
		"pheromone_postgresql_active_connections",
		"pheromone_postgresql_queries_per_sec",
	} {
		if !names[want] {
			t.Errorf("missing expected metric: %s", want)
		}
	}
}

func TestPostgreSQLMonitorSkill_UnknownActionReturnsError(t *testing.T) {
	s := example.NewPostgreSQLMonitorSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "bad-action",
	})
	if err == nil {
		t.Error("expected error for unknown action type")
	}
}

func TestPostgreSQLMonitorSkill_MinTwinLevel(t *testing.T) {
	s := example.NewPostgreSQLMonitorSkill()
	if s.MinTwinLevel() != skill.TwinLevelWorkload {
		t.Errorf("expected TwinLevelWorkload, got %s", s.MinTwinLevel())
	}
}
