// Package config provides configuration types, loading, generation, and validation
// for Pheromone server and agent components.
//
// Configuration files are read from a configurable directory path. Multiple files
// may be placed in the directory; they are loaded and merged in lexicographic order.
// Both JSON (.json) and YAML (.yaml / .yml) formats are supported.
//
// Default configuration paths:
//
//	Server: /etc/pheromone/server  (override via --config-path or PHEROMONE_SERVER_CONFIG_PATH)
//	Agent:  /etc/pheromone/agent   (override via --config-path or PHEROMONE_AGENT_CONFIG_PATH)
package config

import "time"

// Format identifies the serialisation format of a configuration file.
type Format string

const (
	// FormatJSON is the JSON configuration format (.json extension).
	FormatJSON Format = "json"

	// FormatYAML is the YAML configuration format (.yaml or .yml extension).
	FormatYAML Format = "yaml"
)

// DefaultServerConfigPath is the default directory scanned for server configuration files.
const DefaultServerConfigPath = "/etc/pheromone/server"

// DefaultAgentConfigPath is the default directory scanned for agent configuration files.
const DefaultAgentConfigPath = "/etc/pheromone/agent"

// ServerConfig is the top-level configuration for the Pheromone server.
// Multiple files in the config directory are merged: later files override earlier ones
// for scalar fields; slice fields are appended.
type ServerConfig struct {
	// Server holds core operational settings for the server process.
	Server ServerSettings `json:"server" yaml:"server"`

	// Twins is the list of digital twins managed by this server instance.
	Twins []TwinConfig `json:"twins,omitempty" yaml:"twins,omitempty"`

	// Listeners configures event-listener endpoints (webhooks, NATS, Kafka, etc.).
	Listeners []ListenerConfig `json:"listeners,omitempty" yaml:"listeners,omitempty"`

	// UI configures the built-in HTTP management UI and REST API.
	UI UIConfig `json:"ui,omitempty" yaml:"ui,omitempty"`
}

// ServerSettings holds core operational settings for the Pheromone server.
type ServerSettings struct {
	// Address is the network interface the gRPC server binds to. Default: "0.0.0.0".
	Address string `json:"address,omitempty" yaml:"address,omitempty"`

	// Port is the TCP port the gRPC server listens on. Default: 50051.
	Port int `json:"port,omitempty" yaml:"port,omitempty"`

	// ConfigPath is the directory scanned for server configuration files.
	// Defaults to DefaultServerConfigPath.
	ConfigPath string `json:"config_path,omitempty" yaml:"config_path,omitempty"`

	// LogLevel sets the minimum log verbosity. Valid values: "debug", "info", "warn", "error".
	// Default: "info".
	LogLevel string `json:"log_level,omitempty" yaml:"log_level,omitempty"`

	// TLSCertFile is the path to the TLS certificate file. Leave empty to disable TLS.
	TLSCertFile string `json:"tls_cert_file,omitempty" yaml:"tls_cert_file,omitempty"`

	// TLSKeyFile is the path to the TLS private key file. Leave empty to disable TLS.
	TLSKeyFile string `json:"tls_key_file,omitempty" yaml:"tls_key_file,omitempty"`

	// HeartbeatTimeout is the maximum interval between agent heartbeats before the
	// server marks an agent as stale. Default: 30s.
	HeartbeatTimeout time.Duration `json:"heartbeat_timeout,omitempty" yaml:"heartbeat_timeout,omitempty"`
}

// AgentConfig is the top-level configuration for a Pheromone agent.
type AgentConfig struct {
	// Agent holds core operational settings for the agent process.
	Agent AgentSettings `json:"agent" yaml:"agent"`

	// Skills lists the skills this agent should load and make available.
	Skills []SkillConfig `json:"skills,omitempty" yaml:"skills,omitempty"`
}

// AgentSettings holds core operational settings for a Pheromone agent.
type AgentSettings struct {
	// ID is the unique identifier for this agent instance.
	// If empty, the agent generates a UUID on first start and persists it.
	ID string `json:"id,omitempty" yaml:"id,omitempty"`

	// Name is a human-readable label for this agent, e.g. "web-server-01".
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// ServerAddr is the gRPC address of the Pheromone server, e.g. "server.internal:50051".
	ServerAddr string `json:"server_addr,omitempty" yaml:"server_addr,omitempty"`

	// ConfigPath is the directory scanned for agent configuration files.
	// Defaults to DefaultAgentConfigPath.
	ConfigPath string `json:"config_path,omitempty" yaml:"config_path,omitempty"`

	// LogLevel sets the minimum log verbosity. Valid values: "debug", "info", "warn", "error".
	// Default: "info".
	LogLevel string `json:"log_level,omitempty" yaml:"log_level,omitempty"`

	// TickInterval is the cadence at which the agent reasoning loop runs. Default: 5s.
	TickInterval time.Duration `json:"tick_interval,omitempty" yaml:"tick_interval,omitempty"`

	// Distribution configures how the agent discovers and loads skills.
	Distribution DistributionSettings `json:"distribution,omitempty" yaml:"distribution,omitempty"`

	// TLSCACertFile is the path to the CA certificate used to verify the server's TLS certificate.
	TLSCACertFile string `json:"tls_ca_cert_file,omitempty" yaml:"tls_ca_cert_file,omitempty"`
}

// DistributionSettings mirrors skill.DistributionConfig in a serialisable form.
type DistributionSettings struct {
	// Mode selects the skill distribution strategy: "server", "remote", or "local".
	// Default: "server".
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty"`

	// RemoteURL is the skill registry base URL used when Mode == "remote".
	RemoteURL string `json:"remote_url,omitempty" yaml:"remote_url,omitempty"`

	// LocalDir is the filesystem directory scanned for skill specs when Mode == "local".
	LocalDir string `json:"local_dir,omitempty" yaml:"local_dir,omitempty"`

	// TLSVerify controls TLS certificate verification for remote HTTP calls.
	// Defaults to true. Set to false only in development/testing environments.
	TLSVerify *bool `json:"tls_verify,omitempty" yaml:"tls_verify,omitempty"`

	// TimeoutSeconds is the per-request timeout for remote HTTP calls. Default: 10.
	TimeoutSeconds int `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
}

// TwinConfig is the configuration for a managed digital twin.
type TwinConfig struct {
	// ID is the unique identifier for this twin, e.g. "web-server-01".
	ID string `json:"id" yaml:"id"`

	// Name is a human-readable label for this twin.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Type identifies the twin level: "os" or "workload".
	Type string `json:"type" yaml:"type"`

	// Metadata is an arbitrary key-value map attached to the twin.
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// AgentsFile is an optional path to an AGENTS.md file that describes
	// how AI agents should manage this twin. When set, the agent runtime
	// loads this file as a contextual prompt for the reasoning loop,
	// allowing per-twin AI behaviour customisation (see ADR-007).
	AgentsFile string `json:"agents_file,omitempty" yaml:"agents_file,omitempty"`
}

// ListenerConfig configures an event listener endpoint.
type ListenerConfig struct {
	// Name is a unique label for this listener, e.g. "alerts-webhook".
	Name string `json:"name" yaml:"name"`

	// Type identifies the listener protocol: "webhook", "nats", or "kafka".
	Type string `json:"type" yaml:"type"`

	// Address is the bind address or connection URL for this listener.
	// For webhooks this is the HTTP bind address, e.g. ":8080".
	// For NATS/Kafka this is the broker URL, e.g. "nats://localhost:4222".
	Address string `json:"address" yaml:"address"`

	// Options holds listener-specific key-value settings (e.g. topic names,
	// authentication tokens, consumer group IDs).
	Options map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
}

// SkillConfig configures a skill that the agent should load.
type SkillConfig struct {
	// Name is the skill identifier, e.g. "nginx-monitor".
	Name string `json:"name" yaml:"name"`

	// Version is the skill version to load, e.g. "1.0.0". Leave empty for latest.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`

	// Options holds skill-specific key-value settings passed to the skill at init time.
	Options map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
}

// UITLSConfig configures TLS for the management UI HTTP server.
type UITLSConfig struct {
	// Enabled controls whether the HTTP management UI serves HTTPS instead of HTTP.
	// Default: false.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// UseOSCertStore instructs the server to use the platform (OS) certificate store
	// for client certificate verification instead of an explicit CA bundle.
	// When true, ca_bundle_file is not required; the system trust pool is used.
	// The server's own cert_file and key_file are still required to present its certificate.
	UseOSCertStore bool `json:"use_os_cert_store" yaml:"use_os_cert_store"`

	// CertFile is the path to the server TLS certificate in PEM format.
	// Required when Enabled is true.
	CertFile string `json:"cert_file,omitempty" yaml:"cert_file,omitempty"`

	// KeyFile is the path to the server TLS private key in PEM format.
	// Required when Enabled is true.
	KeyFile string `json:"key_file,omitempty" yaml:"key_file,omitempty"`

	// CABundleFile is the path to the CA certificate bundle in PEM format used
	// for verifying client certificates (mutual TLS). Optional; ignored when
	// UseOSCertStore is true.
	CABundleFile string `json:"ca_bundle_file,omitempty" yaml:"ca_bundle_file,omitempty"`
}

// UIConfig configures the built-in HTTP management UI and REST API server.
type UIConfig struct {
	// Enabled controls whether the HTTP UI and REST API server is started.
	// Default: true.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Address is the network interface the HTTP server binds to. Default: "0.0.0.0".
	Address string `json:"address,omitempty" yaml:"address,omitempty"`

	// Port is the TCP port the HTTP server listens on. Default: 8081.
	Port int `json:"port,omitempty" yaml:"port,omitempty"`

	// SecretKey is the HMAC secret used to sign bearer tokens.
	// Must be set to a random value in production.
	SecretKey string `json:"secret_key,omitempty" yaml:"secret_key,omitempty"`

	// TokenTTL is the lifetime of issued bearer tokens. Default: 24h.
	// When specified in configuration files, use Go duration syntax, e.g. "24h", "1h30m".
	TokenTTL time.Duration `json:"token_ttl,omitempty" yaml:"token_ttl,omitempty"`

	// Users is the list of users allowed to access the UI and API.
	// At least one admin user should be configured.
	Users []UIUser `json:"users,omitempty" yaml:"users,omitempty"`

	// AllowedOrigins lists CORS allowed origins. Default: ["*"] (all origins).
	AllowedOrigins []string `json:"allowed_origins,omitempty" yaml:"allowed_origins,omitempty"`

	// TLS configures HTTPS for the management UI. When TLS.Enabled is false
	// (the default), the server listens on plain HTTP.
	TLS UITLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
}

// UIUser is a user that may authenticate with the management UI.
type UIUser struct {
	// Username is the login name, e.g. "admin".
	Username string `json:"username" yaml:"username"`

	// PasswordHash is the stored credential in the format "sha256:<salt>:<hex-hash>".
	// Generate with: pheromone-server ui hash-password <password>
	PasswordHash string `json:"password_hash" yaml:"password_hash"`

	// Role controls the user's permissions: "admin" (full access) or "viewer" (read-only).
	Role string `json:"role" yaml:"role"`
}
