// Package api provides the HTTP management UI and REST API server for Pheromone.
//
// The API server exposes a JSON REST API under /api/v1/ and serves the embedded
// single-page management UI at /. Authentication is performed with bearer tokens
// issued by POST /api/v1/auth/login.
package api

import "time"

// Agent represents an agent registered with the server.
type Agent struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Status       string            `json:"status"`
	Hostname     string            `json:"hostname,omitempty"`
	TwinIDs      []string          `json:"twin_ids,omitempty"`
	Skills       []string          `json:"skills,omitempty"`
	Identities   []AgentIdentity   `json:"identities,omitempty"`
	LastSeen     time.Time         `json:"last_seen"`
	RegisteredAt time.Time         `json:"registered_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// AgentIdentity is a credential or external identifier attached to an agent.
type AgentIdentity struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

// Skill represents a skill available in the server registry.
type Skill struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	MinTwinLevel string `json:"min_twin_level"`
	Description  string `json:"description,omitempty"`
	AgentCount   int    `json:"agent_count"`
}

// Twin represents a digital twin managed by the server.
type Twin struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	State         string                 `json:"state"`
	AgentID       string                 `json:"agent_id,omitempty"`
	ConfigVersion int                    `json:"config_version"`
	LastHeartbeat time.Time              `json:"last_heartbeat"`
	Metadata      map[string]string      `json:"metadata,omitempty"`
	ActualState   map[string]interface{} `json:"actual_state,omitempty"`
	DesiredState  map[string]interface{} `json:"desired_state,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// Changeset records a proposed or applied set of changes to a twin's state.
type Changeset struct {
	ID         string        `json:"id"`
	TwinID     string        `json:"twin_id"`
	AgentID    string        `json:"agent_id"`
	Type       string        `json:"type"`
	Status     string        `json:"status"`
	Changes    []FieldChange `json:"changes"`
	ApprovedBy string        `json:"approved_by,omitempty"`
	AppliedAt  *time.Time    `json:"applied_at,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
}

// FieldChange captures a single field-level change within a changeset.
type FieldChange struct {
	Field    string `json:"field"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

// Group is a named collection of agents, twins, or infrastructure nodes.
type Group struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Description    string    `json:"description,omitempty"`
	Members        []string  `json:"members"`
	TwinNamespace  string    `json:"twin_namespace,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Connection represents an active link between an agent and a twin.
type Connection struct {
	ID       string    `json:"id"`
	AgentID  string    `json:"agent_id"`
	TwinID   string    `json:"twin_id"`
	Status   string    `json:"status"`
	Since    time.Time `json:"since"`
	LastSync time.Time `json:"last_sync"`
}

// HealthResponse is returned by GET /api/v1/health.
type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
	Timestamp time.Time `json:"timestamp"`
}

// StatsResponse is returned by GET /api/v1/stats.
type StatsResponse struct {
	AgentCount      int `json:"agent_count"`
	TwinCount       int `json:"twin_count"`
	OnlineAgents    int `json:"online_agents"`
	ActiveTwins     int `json:"active_twins"`
	SkillCount      int `json:"skill_count"`
	GroupCount      int `json:"group_count"`
	PendingChanges  int `json:"pending_changes"`
	ConnectionCount int `json:"connection_count"`
}

// LoginRequest is the payload for POST /api/v1/auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is returned on successful authentication.
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// UserInfo contains non-sensitive user information.
type UserInfo struct {
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// CreateGroupRequest is the payload for POST /api/v1/groups.
type CreateGroupRequest struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Description   string   `json:"description,omitempty"`
	Members       []string `json:"members,omitempty"`
	TwinNamespace string   `json:"twin_namespace,omitempty"`
}

// UpdateGroupRequest is the payload for PUT /api/v1/groups/{id}.
type UpdateGroupRequest struct {
	Name          string   `json:"name,omitempty"`
	Description   string   `json:"description,omitempty"`
	Members       []string `json:"members,omitempty"`
	TwinNamespace string   `json:"twin_namespace,omitempty"`
}

// ProbeResponse is returned by GET /healthz and GET /readyz.
type ProbeResponse struct {
	Status string `json:"status"`
}

// Event is a real-time server-sent event pushed to /api/v1/events subscribers.
// Type is a dot-separated namespaced event name (e.g. "agent.status_changed").
// Payload contains the updated resource.
type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// CreateUserRequest is the payload for POST /api/v1/users (admin only).
type CreateUserRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
}

// UpdateUserRequest is the payload for PUT /api/v1/users/{username} (admin only).
type UpdateUserRequest struct {
	Role        string `json:"role,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	Disabled    *bool  `json:"disabled,omitempty"`
	Password    string `json:"password,omitempty"`
}

// AuditEntry records a privileged operation performed by a named user.
type AuditEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	UserID     string    `json:"user_id"`
	Role       string    `json:"role"`
	Operation  string    `json:"operation"`
	ResourceID string    `json:"resource_id,omitempty"`
	RemoteAddr string    `json:"remote_addr,omitempty"`
	Result     string    `json:"result"`
	Message    string    `json:"message,omitempty"`
}

// APIKey represents an API key associated with a user for automated access.
type APIKey struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Name        string    `json:"name"`
	KeyPrefix   string    `json:"key_prefix"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// CreateAPIKeyRequest is the payload for POST /api/v1/users/{username}/api-keys.
type CreateAPIKeyRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// CreateAPIKeyResponse includes the plaintext key (shown once) and metadata.
type CreateAPIKeyResponse struct {
	Key    string `json:"key"`
	APIKey APIKey `json:"api_key"`
}

// LogLevelRequest is the payload for POST /api/v1/admin/log-level.
type LogLevelRequest struct {
	// Level is the desired log level: "debug", "info", "warn", or "error".
	Level string `json:"level"`
}

// LogLevelResponse is returned by POST /api/v1/admin/log-level on success.
type LogLevelResponse struct {
	Level string `json:"level"`
}
