package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// GenerateServerConfig writes a default server configuration file to path in the
// requested format. The parent directory is created if it does not exist.
//
// The generated file is a fully-commented example that operators can edit in place.
func GenerateServerConfig(path string, format Format) error {
	cfg := defaultServerConfig()
	return writeConfig(path, format, cfg)
}

// GenerateAgentConfig writes a default agent configuration file to path in the
// requested format. The parent directory is created if it does not exist.
func GenerateAgentConfig(path string, format Format) error {
	cfg := defaultAgentConfig()
	return writeConfig(path, format, cfg)
}

// GenerateTwinConfig writes a default twin configuration fragment to path.
// Twin configs are typically placed in the server config directory.
func GenerateTwinConfig(path string, format Format) error {
	cfg := struct {
		Twins []TwinConfig `json:"twins" yaml:"twins"`
	}{
		Twins: []TwinConfig{defaultTwinConfig()},
	}
	return writeConfig(path, format, cfg)
}

// GenerateListenerConfig writes a default listener configuration fragment to path.
func GenerateListenerConfig(path string, format Format) error {
	cfg := struct {
		Listeners []ListenerConfig `json:"listeners" yaml:"listeners"`
	}{
		Listeners: []ListenerConfig{defaultListenerConfig()},
	}
	return writeConfig(path, format, cfg)
}

// GenerateSkillConfig writes a default skill configuration fragment to path.
func GenerateSkillConfig(path string, format Format) error {
	cfg := struct {
		Skills []SkillConfig `json:"skills" yaml:"skills"`
	}{
		Skills: []SkillConfig{defaultSkillConfig()},
	}
	return writeConfig(path, format, cfg)
}

// ParseFormat converts a user-supplied format string to a Format constant.
// Recognised values (case-insensitive): "json", "yaml", "yml".
// Returns an error for unrecognised values.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON, nil
	case "yaml", "yml":
		return FormatYAML, nil
	default:
		return "", fmt.Errorf("unsupported format %q: must be one of json, yaml", s)
	}
}

// FormatFromPath infers the Format from the file extension of path.
// Returns FormatJSON as the default for unrecognised extensions.
func FormatFromPath(path string) Format {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return FormatYAML
	default:
		return FormatJSON
	}
}

// --- default value builders ---

func defaultServerConfig() *ServerConfig {
	heartbeat := 30 * time.Second
	return &ServerConfig{
		Server: ServerSettings{
			Address:          "0.0.0.0",
			Port:             50051,
			ConfigPath:       DefaultServerConfigPath,
			LogLevel:         "info",
			HeartbeatTimeout: heartbeat,
		},
		Twins:     []TwinConfig{defaultTwinConfig()},
		Listeners: []ListenerConfig{defaultListenerConfig()},
	}
}

func defaultAgentConfig() *AgentConfig {
	tlsVerify := true
	tick := 5 * time.Second
	return &AgentConfig{
		Agent: AgentSettings{
			ID:           "agent-01",
			Name:         "my-agent",
			ServerAddr:   "localhost:50051",
			ConfigPath:   DefaultAgentConfigPath,
			LogLevel:     "info",
			TickInterval: tick,
			Distribution: DistributionSettings{
				Mode:           "server",
				TLSVerify:      &tlsVerify,
				TimeoutSeconds: 10,
			},
		},
		Skills: []SkillConfig{defaultSkillConfig()},
	}
}

func defaultTwinConfig() TwinConfig {
	return TwinConfig{
		ID:   "twin-01",
		Name: "My Twin",
		Type: "os",
		Metadata: map[string]string{
			"environment": "production",
			"region":      "us-east-1",
		},
		AgentsFile: "/etc/pheromone/twins/twin-01/AGENTS.md",
	}
}

func defaultListenerConfig() ListenerConfig {
	return ListenerConfig{
		Name:    "alerts-webhook",
		Type:    "webhook",
		Address: ":8080",
		Options: map[string]string{
			"path":   "/events",
			"secret": "",
		},
	}
}

func defaultSkillConfig() SkillConfig {
	return SkillConfig{
		Name:    "digital-twin",
		Version: "1.0.0",
		Options: map[string]string{},
	}
}

// --- serialisation helpers ---

// writeConfig encodes v into path using the specified format.
// The parent directory is created with 0755 permissions if absent.
func writeConfig(path string, format Format, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("config generate: mkdir %s: %w", filepath.Dir(path), err)
	}

	var data []byte
	var err error

	switch format {
	case FormatYAML:
		data, err = yaml.Marshal(v)
		if err != nil {
			return fmt.Errorf("config generate: yaml marshal: %w", err)
		}
	default:
		data, err = json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("config generate: json marshal: %w", err)
		}
		data = append(data, '\n')
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("config generate: write %s: %w", path, err)
	}
	return nil
}
