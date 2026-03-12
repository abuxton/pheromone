// Package logging provides structured logging helpers for Pheromone,
// including agent context enrichment and a NATS slog.Handler for log streaming.
package logging

import (
	"log/slog"
)

// WithAgentAttrs returns a new logger enriched with agent-context slog attributes.
// Fields with empty values are omitted.
func WithAgentAttrs(logger *slog.Logger, agentID, skill, reasoner string) *slog.Logger {
	args := make([]any, 0, 6)
	if agentID != "" {
		args = append(args, "agent_id", agentID)
	}
	if skill != "" {
		args = append(args, "skill", skill)
	}
	if reasoner != "" {
		args = append(args, "reasoner", reasoner)
	}
	if len(args) == 0 {
		return logger
	}
	return logger.With(args...)
}
