package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/abuxton/pheromone/internal/config"
	uiassets "github.com/abuxton/pheromone/ui"
)

// Server is the Pheromone HTTP management UI and REST API server.
type Server struct {
	cfg       config.UIConfig
	log       *slog.Logger
	httpSrv   *http.Server
	startTime time.Time
	ready     bool // true once the server has finished seeding/startup

	mu          sync.RWMutex
	users       map[string]*config.UIUser // keyed by lowercase username
	agents      map[string]*Agent
	twins       map[string]*Twin
	skills      map[string]*Skill
	groups      map[string]*Group
	changesets  map[string]*Changeset
	connections map[string]*Connection

	// SSE subscribers: each connected /api/v1/events client has a buffered channel.
	eventMu      sync.RWMutex
	eventClients map[chan []byte]struct{}

	// Per-IP rate limit state for this server instance.
	rateLimiters sync.Map // map[string]*ipRateLimiter
}

// New creates a new API Server with the given UIConfig and logger.
// Call Seed to populate demo data and ListenAndServe to start accepting requests.
// If log is nil, slog.Default() is used.
func New(cfg config.UIConfig, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		cfg:          cfg,
		log:          log,
		startTime:    time.Now(),
		users:        make(map[string]*config.UIUser),
		agents:       make(map[string]*Agent),
		twins:        make(map[string]*Twin),
		skills:       make(map[string]*Skill),
		groups:       make(map[string]*Group),
		changesets:   make(map[string]*Changeset),
		connections:  make(map[string]*Connection),
		eventClients: make(map[chan []byte]struct{}),
	}

	// Index configured users by lowercase username.
	for i := range cfg.Users {
		u := cfg.Users[i]
		s.users[strings.ToLower(u.Username)] = &u
	}

	return s
}

// Seed populates the server with example data so the UI is immediately useful.
// In production this data would come from the agent registry and twin store.
func (s *Server) Seed() {
	now := time.Now().UTC()

	// Skills
	skills := []*Skill{
		{Name: "digital-twin", Version: "1.0.0", MinTwinLevel: "os", Description: "Read, update, and apply twin state to the system", AgentCount: 3},
		{Name: "metrics", Version: "1.0.0", MinTwinLevel: "workload", Description: "Collect and emit observability metrics", AgentCount: 4},
		{Name: "config-enforce", Version: "1.0.0", MinTwinLevel: "os", Description: "Apply configuration changes and validate compliance", AgentCount: 2},
		{Name: "nginx-monitor", Version: "1.0.0", MinTwinLevel: "workload", Description: "Monitor Nginx service state", AgentCount: 1},
		{Name: "postgresql-monitor", Version: "1.0.0", MinTwinLevel: "workload", Description: "PostgreSQL health checks", AgentCount: 1},
	}
	for _, sk := range skills {
		s.skills[sk.Name] = sk
	}

	// Agents
	agents := []*Agent{
		{
			ID: "agent-os-01", Name: "web-server-01", Type: "os", Status: "online",
			Hostname: "web-server-01.internal", TwinIDs: []string{"twin-os-01"},
			Skills: []string{"digital-twin", "metrics", "config-enforce"},
			Identities: []AgentIdentity{
				{Type: "tls-cert", Value: "CN=web-server-01", Label: "mTLS"},
			},
			LastSeen: now.Add(-5 * time.Second), RegisteredAt: now.Add(-72 * time.Hour),
			Metadata: map[string]string{"env": "production", "region": "us-east-1"},
		},
		{
			ID: "agent-os-02", Name: "db-server-01", Type: "os", Status: "online",
			Hostname: "db-server-01.internal", TwinIDs: []string{"twin-os-02"},
			Skills: []string{"digital-twin", "metrics", "config-enforce"},
			Identities: []AgentIdentity{
				{Type: "tls-cert", Value: "CN=db-server-01", Label: "mTLS"},
			},
			LastSeen: now.Add(-12 * time.Second), RegisteredAt: now.Add(-72 * time.Hour),
			Metadata: map[string]string{"env": "production", "region": "us-east-1"},
		},
		{
			ID: "agent-wl-01", Name: "nginx-agent-01", Type: "workload", Status: "online",
			Hostname: "web-server-01.internal", TwinIDs: []string{"twin-wl-01"},
			Skills: []string{"nginx-monitor", "metrics"},
			Identities: []AgentIdentity{
				{Type: "api-key", Value: "ak_•••••••1a2b", Label: "Nginx Agent Key"},
			},
			LastSeen: now.Add(-2 * time.Second), RegisteredAt: now.Add(-48 * time.Hour),
			Metadata: map[string]string{"env": "production", "service": "nginx"},
		},
		{
			ID: "agent-wl-02", Name: "pg-agent-01", Type: "workload", Status: "stale",
			Hostname: "db-server-01.internal", TwinIDs: []string{"twin-wl-02"},
			Skills: []string{"postgresql-monitor", "metrics"},
			Identities: []AgentIdentity{
				{Type: "api-key", Value: "ak_•••••••3c4d", Label: "PG Agent Key"},
			},
			LastSeen: now.Add(-5 * time.Minute), RegisteredAt: now.Add(-48 * time.Hour),
			Metadata: map[string]string{"env": "production", "service": "postgresql"},
		},
	}
	for _, a := range agents {
		s.agents[a.ID] = a
	}

	// Twins
	twins := []*Twin{
		{
			ID: "twin-os-01", Name: "web-server-01 OS", Type: "os", State: "active",
			AgentID: "agent-os-01", ConfigVersion: 5,
			LastHeartbeat: now.Add(-5 * time.Second),
			Metadata: map[string]string{"env": "production", "region": "us-east-1"},
			ActualState:  map[string]interface{}{"os": "Ubuntu 24.04", "kernel": "6.8.0-50-generic", "uptime_hours": 720, "cpu_cores": 8, "memory_gb": 32},
			DesiredState: map[string]interface{}{"os": "Ubuntu 24.04", "kernel": "6.8.0-50-generic", "cpu_cores": 8, "memory_gb": 32},
			CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now.Add(-5 * time.Second),
		},
		{
			ID: "twin-os-02", Name: "db-server-01 OS", Type: "os", State: "active",
			AgentID: "agent-os-02", ConfigVersion: 3,
			LastHeartbeat: now.Add(-12 * time.Second),
			Metadata: map[string]string{"env": "production", "region": "us-east-1"},
			ActualState:  map[string]interface{}{"os": "Debian 12", "kernel": "6.1.0-27-amd64", "uptime_hours": 500, "cpu_cores": 16, "memory_gb": 64},
			DesiredState: map[string]interface{}{"os": "Debian 12", "kernel": "6.1.0-28-amd64", "cpu_cores": 16, "memory_gb": 64},
			CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now.Add(-12 * time.Second),
		},
		{
			ID: "twin-wl-01", Name: "nginx-service", Type: "workload", State: "active",
			AgentID: "agent-wl-01", ConfigVersion: 12,
			LastHeartbeat: now.Add(-2 * time.Second),
			Metadata: map[string]string{"env": "production", "service": "nginx"},
			ActualState:  map[string]interface{}{"nginx_version": "1.24.0", "worker_processes": "4", "connections_active": 128, "status": "running"},
			DesiredState: map[string]interface{}{"nginx_version": "1.24.0", "worker_processes": "4", "status": "running"},
			CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-2 * time.Second),
		},
		{
			ID: "twin-wl-02", Name: "postgresql-service", Type: "workload", State: "stale",
			AgentID: "agent-wl-02", ConfigVersion: 7,
			LastHeartbeat: now.Add(-5 * time.Minute),
			Metadata: map[string]string{"env": "production", "service": "postgresql"},
			ActualState:  map[string]interface{}{"pg_version": "16.4", "max_connections": "200", "status": "degraded"},
			DesiredState: map[string]interface{}{"pg_version": "16.4", "max_connections": "200", "status": "running"},
			CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-5 * time.Minute),
		},
	}
	for _, t := range twins {
		s.twins[t.ID] = t
	}

	// Changesets
	applied := now.Add(-1 * time.Hour)
	changesets := []*Changeset{
		{
			ID: "cs-001", TwinID: "twin-os-02", AgentID: "agent-os-02",
			Type: "config", Status: "pending",
			Changes: []FieldChange{
				{Field: "kernel", OldValue: "6.1.0-27-amd64", NewValue: "6.1.0-28-amd64"},
			},
			CreatedAt: now.Add(-10 * time.Minute),
		},
		{
			ID: "cs-002", TwinID: "twin-wl-02", AgentID: "agent-wl-02",
			Type: "state", Status: "pending",
			Changes: []FieldChange{
				{Field: "status", OldValue: "degraded", NewValue: "running"},
				{Field: "max_connections", OldValue: "200", NewValue: "300"},
			},
			CreatedAt: now.Add(-4 * time.Minute),
		},
		{
			ID: "cs-003", TwinID: "twin-os-01", AgentID: "agent-os-01",
			Type: "config", Status: "applied",
			Changes: []FieldChange{
				{Field: "nginx_version", OldValue: "1.22.0", NewValue: "1.24.0"},
			},
			ApprovedBy: "admin",
			AppliedAt:  &applied,
			CreatedAt:  now.Add(-2 * time.Hour),
		},
	}
	for _, c := range changesets {
		s.changesets[c.ID] = c
	}

	// Groups
	groups := []*Group{
		{
			ID: "grp-001", Name: "Production Servers", Type: "infrastructure",
			Description: "All production infrastructure nodes",
			Members:     []string{"web-server-01.internal", "db-server-01.internal"},
			CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now.Add(-72 * time.Hour),
		},
		{
			ID: "grp-002", Name: "OS Agents", Type: "agents",
			Description: "All OS-level agents",
			Members:     []string{"agent-os-01", "agent-os-02"},
			CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now.Add(-72 * time.Hour),
		},
		{
			ID: "grp-003", Name: "Workload Twins", Type: "twins",
			Description: "All workload-level digital twins",
			Members:     []string{"twin-wl-01", "twin-wl-02"},
			CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour),
		},
	}
	for _, g := range groups {
		s.groups[g.ID] = g
	}

	// Connections
	connections := []*Connection{
		{ID: "conn-001", AgentID: "agent-os-01", TwinID: "twin-os-01", Status: "connected", Since: now.Add(-72 * time.Hour), LastSync: now.Add(-5 * time.Second)},
		{ID: "conn-002", AgentID: "agent-os-02", TwinID: "twin-os-02", Status: "connected", Since: now.Add(-72 * time.Hour), LastSync: now.Add(-12 * time.Second)},
		{ID: "conn-003", AgentID: "agent-wl-01", TwinID: "twin-wl-01", Status: "connected", Since: now.Add(-48 * time.Hour), LastSync: now.Add(-2 * time.Second)},
		{ID: "conn-004", AgentID: "agent-wl-02", TwinID: "twin-wl-02", Status: "disconnected", Since: now.Add(-48 * time.Hour), LastSync: now.Add(-5 * time.Minute)},
	}
	for _, c := range connections {
		s.connections[c.ID] = c
	}
	s.ready = true
}

// broadcast sends an Event to all active SSE subscribers.
// It is safe to call concurrently.
func (s *Server) broadcast(evt Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		s.log.Warn("broadcast: marshal event failed", "error", err)
		return
	}
	// SSE frame: "data: <json>\n\n"
	frame := append([]byte("data: "), data...)
	frame = append(frame, '\n', '\n')

	s.eventMu.RLock()
	defer s.eventMu.RUnlock()
	for ch := range s.eventClients {
		// Non-blocking send; drop the frame if the client is slow.
		select {
		case ch <- frame:
		default:
		}
	}
}

// buildTLSConfig constructs a *tls.Config from the UI TLS configuration.
// Returns nil, nil when TLS is disabled.
func (s *Server) buildTLSConfig() (*tls.Config, error) {
	t := s.cfg.TLS
	if !t.Enabled {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS cert/key: %w", err)
	}

	tc := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if t.UseOSCertStore {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system cert pool: %w", err)
		}
		tc.ClientCAs = pool
		tc.ClientAuth = tls.VerifyClientCertIfGiven
	} else if t.CABundleFile != "" {
		pem, err := os.ReadFile(t.CABundleFile)
		if err != nil {
			return nil, fmt.Errorf("read CA bundle %s: %w", t.CABundleFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no valid certificates found in CA bundle %s", t.CABundleFile)
		}
		tc.ClientCAs = pool
		tc.ClientAuth = tls.VerifyClientCertIfGiven
	}

	return tc, nil
}

// ListenAndServe starts the HTTP (or HTTPS) server on the configured address and port.
// It blocks until the context is cancelled or a fatal error occurs.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Address, s.cfg.Port)
	if s.cfg.Address == "" {
		addr = fmt.Sprintf(":%d", s.cfg.Port)
	}

	// Start the rate-limit eviction goroutine once per server lifetime.
	s.startRateLimitEviction(ctx)

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	handler := s.corsMiddleware(
		s.rateLimitMiddleware(
			requestSizeLimitMiddleware(
				loggingMiddleware(s.log,
					s.authMiddleware(mux),
				),
			),
		),
	)

	tlsConfig, err := s.buildTLSConfig()
	if err != nil {
		return fmt.Errorf("TLS config: %w", err)
	}

	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig:    tlsConfig,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	scheme := "http"
	if tlsConfig != nil {
		ln = tls.NewListener(ln, tlsConfig)
		scheme = "https"
	}

	s.log.Info("pheromone UI server listening", "addr", addr, "scheme", scheme)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.httpSrv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

// registerRoutes wires all API and UI routes to the mux.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// k8s-compatible health probes (no authentication required).
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)

	// API routes
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	mux.HandleFunc("/api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("/api/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("/api/v1/auth/whoami", s.handleWhoami)
	mux.HandleFunc("/api/v1/agents", s.handleAgents)
	mux.HandleFunc("/api/v1/agents/", s.handleAgent)
	mux.HandleFunc("/api/v1/twins", s.handleTwins)
	mux.HandleFunc("/api/v1/twins/", s.routeTwin)
	mux.HandleFunc("/api/v1/skills", s.handleSkills)
	mux.HandleFunc("/api/v1/groups", s.handleGroups)
	mux.HandleFunc("/api/v1/groups/", s.handleGroup)
	mux.HandleFunc("/api/v1/changesets", s.handleChangesets)
	mux.HandleFunc("/api/v1/changesets/", s.handleChangeset)
	mux.HandleFunc("/api/v1/connections", s.handleConnections)
	mux.HandleFunc("/api/v1/users", adminMiddleware(http.HandlerFunc(s.handleUsers)).ServeHTTP)

	// Embedded UI: serve index.html for all non-API paths.
	mux.Handle("/", http.FileServer(http.FS(uiassets.FS)))
}

// routeTwin dispatches /api/v1/twins/{id} and /api/v1/twins/{id}/changesets.
func (s *Server) routeTwin(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/twins/")
	if strings.HasSuffix(path, "/changesets") {
		s.handleTwinChangesets(w, r)
		return
	}
	s.handleTwin(w, r)
}
