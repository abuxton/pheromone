package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// handleHealthz serves GET /healthz (Kubernetes liveness probe).
// Always returns 200 OK once the process is alive and the listener is up.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, ProbeResponse{Status: "ok"})
}

// handleReadyz serves GET /readyz (Kubernetes readiness probe).
// Returns 200 OK when the server has finished startup/seeding and is ready
// to serve traffic. Returns 503 Service Unavailable during startup.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, ProbeResponse{Status: "starting"})
		return
	}
	writeJSON(w, http.StatusOK, ProbeResponse{Status: "ok"})
}

// handleHealth serves GET /api/v1/health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		Version:   "0.1.0",
		Uptime:    time.Since(s.startTime).Round(time.Second).String(),
		Timestamp: time.Now().UTC(),
	})
}

// handleStats serves GET /api/v1/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	online := 0
	for _, a := range s.agents {
		if a.Status == "online" {
			online++
		}
	}
	active := 0
	for _, t := range s.twins {
		if t.State == "active" {
			active++
		}
	}
	pending := 0
	for _, c := range s.changesets {
		if c.Status == "pending" {
			pending++
		}
	}

	writeJSON(w, http.StatusOK, StatsResponse{
		AgentCount:      len(s.agents),
		TwinCount:       len(s.twins),
		OnlineAgents:    online,
		ActiveTwins:     active,
		SkillCount:      len(s.skills),
		GroupCount:      len(s.groups),
		PendingChanges:  pending,
		ConnectionCount: len(s.connections),
	})
}

// handleLogin serves POST /api/v1/auth/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	s.mu.RLock()
	user, ok := s.users[strings.ToLower(req.Username)]
	s.mu.RUnlock()

	if !ok || !checkPassword(req.Password, user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	ttl := s.cfg.TokenTTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	token, exp, err := generateToken(user.Username, user.Role, s.cfg.SecretKey, ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresAt: exp.UTC(),
		User:      UserInfo{Username: user.Username, Role: user.Role},
	})
}

// handleLogout serves DELETE /api/v1/auth/logout (client should discard token)
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// handleWhoami serves GET /api/v1/auth/whoami
func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user := userFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, UserInfo{Username: user.Sub, Role: user.Role})
}

// --- Agents ---

// handleAgents serves GET /api/v1/agents
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Agent, 0, len(s.agents))
	for _, a := range s.agents {
		result = append(result, a)
	}
	writeJSON(w, http.StatusOK, result)
}

// handleAgent serves GET /api/v1/agents/{id}
func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "agent id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		a, ok := s.agents[id]
		s.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("agent %q not found", id))
			return
		}
		writeJSON(w, http.StatusOK, a)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// --- Twins ---

// handleTwins serves GET /api/v1/twins
func (s *Server) handleTwins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Twin, 0, len(s.twins))
	for _, t := range s.twins {
		result = append(result, t)
	}
	writeJSON(w, http.StatusOK, result)
}

// handleTwin serves GET /api/v1/twins/{id}
func (s *Server) handleTwin(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/twins/")
	// Strip any trailing sub-path (e.g. /changesets)
	if slash := strings.Index(id, "/"); slash != -1 {
		id = id[:slash]
	}
	if id == "" {
		writeError(w, http.StatusBadRequest, "twin id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		t, ok := s.twins[id]
		s.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("twin %q not found", id))
			return
		}
		writeJSON(w, http.StatusOK, t)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleTwinChangesets serves GET /api/v1/twins/{id}/changesets
func (s *Server) handleTwinChangesets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// path: /api/v1/twins/{id}/changesets
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/twins/")
	parts := strings.SplitN(path, "/", 2)
	twinID := parts[0]

	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.twins[twinID]; !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("twin %q not found", twinID))
		return
	}

	result := make([]*Changeset, 0)
	for _, c := range s.changesets {
		if c.TwinID == twinID {
			result = append(result, c)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Skills ---

// handleSkills serves GET /api/v1/skills
func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Skill, 0, len(s.skills))
	for _, sk := range s.skills {
		result = append(result, sk)
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Groups ---

// handleGroups serves GET /api/v1/groups and POST /api/v1/groups
func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		result := make([]*Group, 0, len(s.groups))
		for _, g := range s.groups {
			result = append(result, g)
		}
		s.mu.RUnlock()
		writeJSON(w, http.StatusOK, result)

	case http.MethodPost:
		var req CreateGroupRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		validTypes := map[string]bool{"agents": true, "twins": true, "infrastructure": true}
		if !validTypes[req.Type] {
			writeError(w, http.StatusBadRequest, "type must be agents, twins, or infrastructure")
			return
		}
		id, _ := randomHex(8)
		g := &Group{
			ID:          id,
			Name:        req.Name,
			Type:        req.Type,
			Description: req.Description,
			Members:     req.Members,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if g.Members == nil {
			g.Members = []string{}
		}
		s.mu.Lock()
		s.groups[id] = g
		s.mu.Unlock()
		writeJSON(w, http.StatusCreated, g)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleGroup serves GET/PUT/DELETE /api/v1/groups/{id}
func (s *Server) handleGroup(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/groups/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "group id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		g, ok := s.groups[id]
		s.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("group %q not found", id))
			return
		}
		writeJSON(w, http.StatusOK, g)

	case http.MethodPut:
		s.mu.Lock()
		g, ok := s.groups[id]
		s.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("group %q not found", id))
			return
		}
		var req UpdateGroupRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		s.mu.Lock()
		if req.Name != "" {
			g.Name = req.Name
		}
		if req.Description != "" {
			g.Description = req.Description
		}
		if req.Members != nil {
			g.Members = req.Members
		}
		g.UpdatedAt = time.Now().UTC()
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, g)

	case http.MethodDelete:
		s.mu.Lock()
		_, ok := s.groups[id]
		if ok {
			delete(s.groups, id)
		}
		s.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("group %q not found", id))
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// --- Changesets ---

// handleChangesets serves GET /api/v1/changesets
func (s *Server) handleChangesets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Changeset, 0, len(s.changesets))
	for _, c := range s.changesets {
		result = append(result, c)
	}
	writeJSON(w, http.StatusOK, result)
}

// handleChangeset serves GET /api/v1/changesets/{id}
func (s *Server) handleChangeset(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/changesets/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "changeset id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		c, ok := s.changesets[id]
		s.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("changeset %q not found", id))
			return
		}
		writeJSON(w, http.StatusOK, c)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// --- Connections ---

// handleConnections serves GET /api/v1/connections
func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Connection, 0, len(s.connections))
	for _, c := range s.connections {
		result = append(result, c)
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Users (admin only) ---

// handleUsers serves GET /api/v1/users (admin only)
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]UserInfo, 0, len(s.users))
	for _, u := range s.users {
		result = append(result, UserInfo{Username: u.Username, Role: u.Role})
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Server-Sent Events ---

// handleEvents serves GET /api/v1/events as a Server-Sent Events (SSE) stream.
// Authenticated clients receive real-time JSON events for agent/twin state changes.
// The connection is kept open until the client disconnects.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Verify the response writer supports flushing (required for SSE).
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering.

	// Register this client.
	ch := make(chan []byte, 64)
	s.eventMu.Lock()
	s.eventClients[ch] = struct{}{}
	s.eventMu.Unlock()

	defer func() {
		s.eventMu.Lock()
		delete(s.eventClients, ch)
		s.eventMu.Unlock()
	}()

	// Send an initial connection acknowledgement.
	_, _ = fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	// Heartbeat ticker keeps the connection alive through proxies.
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case frame, open := <-ch:
			if !open {
				return
			}
			_, err := w.Write(frame)
			if err != nil {
				return
			}
			flusher.Flush()

		case <-heartbeat.C:
			// SSE comment line — ignored by clients but prevents proxy timeouts.
			_, err := fmt.Fprintf(w, ": heartbeat\n\n")
			if err != nil {
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}
