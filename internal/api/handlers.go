package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abuxton/pheromone/internal/config"
	"github.com/abuxton/pheromone/internal/skill"
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
		s.addAuditEntry(req.Username, "", "auth.login", "", r.RemoteAddr, "fail", "invalid credentials")
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if user.Disabled {
		s.addAuditEntry(req.Username, user.Role, "auth.login", "", r.RemoteAddr, "fail", "account disabled")
		writeError(w, http.StatusUnauthorized, "account is disabled")
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

	s.addAuditEntry(user.Username, user.Role, "auth.login", "", r.RemoteAddr, "ok", "")

	writeJSON(w, http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresAt: exp.UTC(),
		User: UserInfo{
			Username:    user.Username,
			Role:        user.Role,
			DisplayName: user.DisplayName,
			Email:       user.Email,
		},
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

// handleAgent serves GET /api/v1/agents/{id} and dispatches
// GET /api/v1/agents/{id}/traces (ADR-019).
func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	if path == "" {
		writeError(w, http.StatusBadRequest, "agent id required")
		return
	}

	// Dispatch sub-resource routes before treating the whole path as an ID.
	if strings.HasSuffix(path, "/traces") {
		s.handleAgentTraces(w, r)
		return
	}

	id := path
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

// handleUsers serves GET /api/v1/users and POST /api/v1/users (admin only)
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		defer s.mu.RUnlock()

		result := make([]UserInfo, 0, len(s.users))
		for _, u := range s.users {
			result = append(result, UserInfo{
				Username:    u.Username,
				Role:        u.Role,
				DisplayName: u.DisplayName,
				Email:       u.Email,
				Disabled:    u.Disabled,
			})
		}
		writeJSON(w, http.StatusOK, result)

	case http.MethodPost:
		caller := userFromContext(r.Context())

		var req CreateUserRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Username == "" {
			writeError(w, http.StatusBadRequest, "username is required")
			return
		}
		validRoles := map[string]bool{"admin": true, "operator": true, "observer": true, "agent": true}
		role := strings.ToLower(req.Role)
		if role == "" || !validRoles[role] {
			writeError(w, http.StatusBadRequest, "role must be one of admin, operator, observer, agent")
			return
		}
		if req.Password == "" {
			writeError(w, http.StatusBadRequest, "password is required")
			return
		}

		hash, err := GeneratePasswordHash(req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}

		key := strings.ToLower(req.Username)
		s.mu.Lock()
		if _, exists := s.users[key]; exists {
			s.mu.Unlock()
			writeError(w, http.StatusConflict, "user already exists")
			return
		}
		u := &config.UIUser{
			Username:     req.Username,
			PasswordHash: hash,
			Role:         role,
			DisplayName:  req.DisplayName,
			Email:        req.Email,
		}
		s.users[key] = u
		s.mu.Unlock()

		callerID, callerRole := "unknown", "unknown"
		if caller != nil {
			callerID, callerRole = caller.Sub, caller.Role
		}
		s.addAuditEntry(callerID, callerRole, "user.create", req.Username, r.RemoteAddr, "ok", "")

		writeJSON(w, http.StatusCreated, UserInfo{
			Username:    u.Username,
			Role:        u.Role,
			DisplayName: u.DisplayName,
			Email:       u.Email,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// routeUser dispatches /api/v1/users/{username} and /api/v1/users/{username}/api-keys.
func (s *Server) routeUser(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	if strings.HasSuffix(path, "/api-keys") {
		s.handleUserAPIKeys(w, r)
		return
	}
	if strings.Contains(path, "/api-keys/") {
		s.handleUserAPIKey(w, r)
		return
	}
	s.handleUser(w, r)
}

// handleUser serves GET/PUT/DELETE /api/v1/users/{username} (admin only)
func (s *Server) handleUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username required")
		return
	}
	key := strings.ToLower(username)
	caller := userFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		u, ok := s.users[key]
		s.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("user %q not found", username))
			return
		}
		writeJSON(w, http.StatusOK, UserInfo{
			Username:    u.Username,
			Role:        u.Role,
			DisplayName: u.DisplayName,
			Email:       u.Email,
			Disabled:    u.Disabled,
		})

	case http.MethodPut:
		var req UpdateUserRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		s.mu.Lock()
		u, ok := s.users[key]
		if !ok {
			s.mu.Unlock()
			writeError(w, http.StatusNotFound, fmt.Sprintf("user %q not found", username))
			return
		}

		if req.Role != "" {
			validRoles := map[string]bool{"admin": true, "operator": true, "observer": true, "agent": true}
			role := strings.ToLower(req.Role)
			if !validRoles[role] {
				s.mu.Unlock()
				writeError(w, http.StatusBadRequest, "role must be one of admin, operator, observer, agent")
				return
			}
			u.Role = role
		}
		if req.DisplayName != "" {
			u.DisplayName = req.DisplayName
		}
		if req.Email != "" {
			u.Email = req.Email
		}
		if req.Disabled != nil {
			u.Disabled = *req.Disabled
		}
		if req.Password != "" {
			hash, err := GeneratePasswordHash(req.Password)
			if err != nil {
				s.mu.Unlock()
				writeError(w, http.StatusInternalServerError, "failed to hash password")
				return
			}
			u.PasswordHash = hash
		}
		s.mu.Unlock()

		callerID, callerRole := "unknown", "unknown"
		if caller != nil {
			callerID, callerRole = caller.Sub, caller.Role
		}
		s.addAuditEntry(callerID, callerRole, "user.update", username, r.RemoteAddr, "ok", "")

		writeJSON(w, http.StatusOK, UserInfo{
			Username:    u.Username,
			Role:        u.Role,
			DisplayName: u.DisplayName,
			Email:       u.Email,
			Disabled:    u.Disabled,
		})

	case http.MethodDelete:
		s.mu.Lock()
		_, ok := s.users[key]
		if !ok {
			s.mu.Unlock()
			writeError(w, http.StatusNotFound, fmt.Sprintf("user %q not found", username))
			return
		}
		delete(s.users, key)
		s.mu.Unlock()

		callerID, callerRole := "unknown", "unknown"
		if caller != nil {
			callerID, callerRole = caller.Sub, caller.Role
		}
		s.addAuditEntry(callerID, callerRole, "user.delete", username, r.RemoteAddr, "ok", "")

		w.WriteHeader(http.StatusNoContent)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleUserAPIKeys serves GET /api/v1/users/{username}/api-keys and
// POST /api/v1/users/{username}/api-keys (admin or self).
func (s *Server) handleUserAPIKeys(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	username := strings.TrimSuffix(path, "/api-keys")
	key := strings.ToLower(username)
	caller := userFromContext(r.Context())

	s.mu.RLock()
	_, ok := s.users[key]
	s.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("user %q not found", username))
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		var keys []APIKey
		for _, k := range s.apiKeys {
			if strings.ToLower(k.Username) == key {
				keys = append(keys, k.APIKey)
			}
		}
		s.mu.RUnlock()
		if keys == nil {
			keys = []APIKey{}
		}
		writeJSON(w, http.StatusOK, keys)

	case http.MethodPost:
		var req CreateAPIKeyRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		// Generate raw key (ph::key::<hex>)
		rawKey, err := randomHex(32)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate API key")
			return
		}
		token := "ph::key::" + rawKey

		// API keys are high-entropy random tokens; use SHA-256 (not bcrypt).
		keyHash := hashAPIKey(token)

		id, _ := randomHex(8)
		record := &apiKeyRecord{
			APIKey: APIKey{
				ID:        id,
				Username:  username,
				Name:      req.Name,
				KeyPrefix: "ph::key::" + rawKey[:8] + "…",
				CreatedAt: time.Now().UTC(),
				ExpiresAt: req.ExpiresAt,
			},
			KeyHash: keyHash,
		}

		s.mu.Lock()
		s.apiKeys[id] = record
		s.mu.Unlock()

		callerID, callerRole := "unknown", "unknown"
		if caller != nil {
			callerID, callerRole = caller.Sub, caller.Role
		}
		s.addAuditEntry(callerID, callerRole, "apikey.create", id, r.RemoteAddr, "ok", req.Name)

		writeJSON(w, http.StatusCreated, CreateAPIKeyResponse{
			Key:    token,
			APIKey: record.APIKey,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleUserAPIKey serves DELETE /api/v1/users/{username}/api-keys/{key_id} (admin only).
func (s *Server) handleUserAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	parts := strings.SplitN(path, "/api-keys/", 2)
	if len(parts) != 2 || parts[1] == "" {
		writeError(w, http.StatusBadRequest, "api key id required")
		return
	}
	keyID := parts[1]
	caller := userFromContext(r.Context())

	s.mu.Lock()
	_, ok := s.apiKeys[keyID]
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusNotFound, fmt.Sprintf("API key %q not found", keyID))
		return
	}
	delete(s.apiKeys, keyID)
	s.mu.Unlock()

	callerID, callerRole := "unknown", "unknown"
	if caller != nil {
		callerID, callerRole = caller.Sub, caller.Role
	}
	s.addAuditEntry(callerID, callerRole, "apikey.revoke", keyID, r.RemoteAddr, "ok", "")

	w.WriteHeader(http.StatusNoContent)
}

// handleAudit serves GET /api/v1/audit (admin only).
// Supports optional query params: user, operation, limit (default 100).
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	q := r.URL.Query()
	filterUser := q.Get("user")
	filterOp := q.Get("operation")
	limitStr := q.Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	s.mu.RLock()
	all := s.auditLog
	s.mu.RUnlock()

	result := make([]*AuditEntry, 0, len(all))
	for i := len(all) - 1; i >= 0; i-- {
		e := all[i]
		if filterUser != "" && e.UserID != filterUser {
			continue
		}
		if filterOp != "" && e.Operation != filterOp {
			continue
		}
		result = append(result, e)
		if len(result) >= limit {
			break
		}
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

// handleLogLevel serves POST /api/v1/admin/log-level (admin-only).
// It accepts a JSON body {"level":"debug|info|warn|error"} and updates the
// server's runtime log level without requiring a restart.
func (s *Server) handleLogLevel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req LogLevelRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	var level slog.Level
	switch strings.ToLower(req.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		writeError(w, http.StatusBadRequest, "invalid log level: must be debug, info, warn, or error")
		return
	}

	s.logLevel.Set(level)
	s.log.Info("log level updated",
		"level", level.String(),
		"component", "api",
		"request_id", requestIDFromContext(r.Context()),
	)
	writeJSON(w, http.StatusOK, LogLevelResponse{Level: level.String()})
}

// --- Agent Traces (ADR-019) ---

// handleAgentTraces serves GET /api/v1/agents/{id}/traces.
//
// Query param: limit (optional, default 10, max 100) — number of most-recent
// traces to return.
func (s *Server) handleAgentTraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// path: /api/v1/agents/{id}/traces
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.SplitN(path, "/", 2)
	agentID := parts[0]
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id required")
		return
	}

	s.mu.RLock()
	_, ok := s.agents[agentID]
	s.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("agent %q not found", agentID))
		return
	}

	limit := 10
	if lv := r.URL.Query().Get("limit"); lv != "" {
		if n, err := strconv.Atoi(lv); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			limit = n
		}
	}

	raw := s.traceStore.Get(agentID, limit)
	result := make([]AgentTrace, 0, len(raw))
	for _, tr := range raw {
		result = append(result, traceToAPI(tr))
	}
	writeJSON(w, http.StatusOK, result)
}

// traceToAPI converts a skill.ReasonerTrace to the API wire type.
func traceToAPI(tr skill.ReasonerTrace) AgentTrace {
	return AgentTrace{
		Timestamp:     tr.Timestamp,
		AgentID:       tr.AgentID,
		Reasoner:      tr.Reasoner,
		TwinID:        tr.Input.TwinID,
		HasDrift:      tr.Input.HasDrift,
		DriftedFields: tr.Input.DriftedFields,
		Actions:       tr.Actions,
		Outcome:       tr.Outcome,
		DurationMs:    tr.DurationMs,
		FallbackUsed:  tr.FallbackUsed,
	}
}

// --- Twin Diff (ADR-019) ---

// handleTwinDiff serves GET /api/v1/twins/{id}/diff.
//
// Returns the field-by-field delta between the twin's desired and actual state.
func (s *Server) handleTwinDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// path: /api/v1/twins/{id}/diff
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/twins/")
	parts := strings.SplitN(path, "/", 2)
	twinID := parts[0]
	if twinID == "" {
		writeError(w, http.StatusBadRequest, "twin id required")
		return
	}

	s.mu.RLock()
	t, ok := s.twins[twinID]
	s.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("twin %q not found", twinID))
		return
	}

	actual := t.ActualState
	desired := t.DesiredState
	if actual == nil {
		actual = map[string]interface{}{}
	}
	if desired == nil {
		desired = map[string]interface{}{}
	}

	// Collect the union of all keys from both states.
	keySet := make(map[string]struct{}, len(actual)+len(desired))
	for k := range actual {
		keySet[k] = struct{}{}
	}
	for k := range desired {
		keySet[k] = struct{}{}
	}

	fields := make([]DiffField, 0, len(keySet))
	hasDrift := false
	for k := range keySet {
		a := fmt.Sprintf("%v", actual[k])
		d := fmt.Sprintf("%v", desired[k])
		drifted := a != d
		if drifted {
			hasDrift = true
		}
		fields = append(fields, DiffField{
			Field:   k,
			Actual:  a,
			Desired: d,
			Drifted: drifted,
		})
	}

	writeJSON(w, http.StatusOK, TwinDiff{
		TwinID:   twinID,
		HasDrift: hasDrift,
		Fields:   fields,
	})
}
