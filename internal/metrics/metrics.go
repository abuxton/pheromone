// Package metrics provides the Prometheus metrics registry and all instrumented
// metrics for the Pheromone server. A single package-level registry is exposed
// so that server.go and any future instrumentors share the same namespace.
//
// All metrics are registered once on package initialisation via MustRegister
// on a custom (non-global) registry. This allows multiple test instances to
// create independent registries without collision.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Registry is the package-level Prometheus registry.
// Use NewRegistry to create an isolated registry (e.g. for tests).
var Registry = prometheus.NewRegistry()

// Package-level metric handles wired to the default Registry.
var (
	// AgentsTotal is a gauge tracking registered agents by status.
	AgentsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pheromone_agents_total",
		Help: "Total number of registered agents, partitioned by status.",
	}, []string{"status"})

	// TwinsTotal is a gauge tracking digital twins.
	TwinsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pheromone_twins_total",
		Help: "Total number of digital twins.",
	})

	// GRPCRequestsTotal is a counter of gRPC requests partitioned by service,
	// method, and gRPC status code.
	GRPCRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pheromone_grpc_requests_total",
		Help: "Total number of gRPC requests, partitioned by service, method, and status code.",
	}, []string{"service", "method", "code"})

	// GRPCDurationSeconds is a histogram of gRPC request latency.
	GRPCDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pheromone_grpc_duration_seconds",
		Help:    "gRPC request latency in seconds, partitioned by service and method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method"})

	// NATSMessagesTotal is a counter of NATS messages consumed by subject.
	NATSMessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pheromone_nats_messages_total",
		Help: "Total number of NATS messages processed, partitioned by subject.",
	}, []string{"subject"})

	// ReasoningDecisionsTotal is a counter of agentic reasoning outcomes.
	ReasoningDecisionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pheromone_reasoning_decisions_total",
		Help: "Total number of reasoning decisions made, partitioned by reasoner and outcome.",
	}, []string{"reasoner", "outcome"})
)

func init() {
	// Register all metrics against the package registry. MustRegister panics
	// on duplicate registration, which surfaces wiring errors at startup rather
	// than silently losing metrics.
	Registry.MustRegister(
		AgentsTotal,
		TwinsTotal,
		GRPCRequestsTotal,
		GRPCDurationSeconds,
		NATSMessagesTotal,
		ReasoningDecisionsTotal,
	)
}

// NewRegistry creates a fresh Prometheus registry pre-registered with all
// Pheromone metrics. Use this in tests to avoid cross-test metric collisions.
func NewRegistry() (*prometheus.Registry, *Metrics) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		AgentsTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pheromone_agents_total",
			Help: "Total number of registered agents, partitioned by status.",
		}, []string{"status"}),
		TwinsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pheromone_twins_total",
			Help: "Total number of digital twins.",
		}),
		GRPCRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pheromone_grpc_requests_total",
			Help: "Total number of gRPC requests, partitioned by service, method, and status code.",
		}, []string{"service", "method", "code"}),
		GRPCDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pheromone_grpc_duration_seconds",
			Help:    "gRPC request latency in seconds, partitioned by service and method.",
			Buckets: prometheus.DefBuckets,
		}, []string{"service", "method"}),
		NATSMessagesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pheromone_nats_messages_total",
			Help: "Total number of NATS messages processed, partitioned by subject.",
		}, []string{"subject"}),
		ReasoningDecisionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pheromone_reasoning_decisions_total",
			Help: "Total number of reasoning decisions made, partitioned by reasoner and outcome.",
		}, []string{"reasoner", "outcome"}),
	}
	reg.MustRegister(
		m.AgentsTotal,
		m.TwinsTotal,
		m.GRPCRequestsTotal,
		m.GRPCDurationSeconds,
		m.NATSMessagesTotal,
		m.ReasoningDecisionsTotal,
	)
	return reg, m
}

// Metrics holds isolated metric instances for a single registry.
// Use NewRegistry to obtain a *Metrics bound to a fresh registry.
type Metrics struct {
	AgentsTotal             *prometheus.GaugeVec
	TwinsTotal              prometheus.Gauge
	GRPCRequestsTotal       *prometheus.CounterVec
	GRPCDurationSeconds     *prometheus.HistogramVec
	NATSMessagesTotal       *prometheus.CounterVec
	ReasoningDecisionsTotal *prometheus.CounterVec
}
