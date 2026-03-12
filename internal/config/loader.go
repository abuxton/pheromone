package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadServerConfig loads and merges all configuration files found under dir.
//
// Files are processed in lexicographic order so that numerically-prefixed files
// (e.g. "00-defaults.yaml", "10-overrides.json") override earlier values in a
// predictable way. Scalar fields in later files override those in earlier files;
// slice fields (Twins, Listeners) are appended.
//
// Supported file extensions: .json, .yaml, .yml
// Other files in the directory are silently ignored.
//
// If dir is empty, DefaultServerConfigPath is used.
func LoadServerConfig(dir string) (*ServerConfig, error) {
	if dir == "" {
		dir = DefaultServerConfigPath
	}

	files, err := configFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("config: list %s: %w", dir, err)
	}

	merged := &ServerConfig{}
	for _, f := range files {
		partial, parseErr := parseServerConfig(f)
		if parseErr != nil {
			return nil, fmt.Errorf("config: parse %s: %w", f, parseErr)
		}
		mergeServerConfig(merged, partial)
	}

	applyServerDefaults(merged)
	return merged, nil
}

// LoadAgentConfig loads and merges all configuration files found under dir.
//
// Files are processed in lexicographic order. See LoadServerConfig for full merge
// semantics.
//
// If dir is empty, DefaultAgentConfigPath is used.
func LoadAgentConfig(dir string) (*AgentConfig, error) {
	if dir == "" {
		dir = DefaultAgentConfigPath
	}

	files, err := configFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("config: list %s: %w", dir, err)
	}

	merged := &AgentConfig{}
	for _, f := range files {
		partial, parseErr := parseAgentConfig(f)
		if parseErr != nil {
			return nil, fmt.Errorf("config: parse %s: %w", f, parseErr)
		}
		mergeAgentConfig(merged, partial)
	}

	applyAgentDefaults(merged)
	return merged, nil
}

// configFiles returns the sorted list of JSON/YAML files in dir.
// Returns an empty slice (no error) when dir does not exist, matching the
// behaviour of an unconfigured deployment that relies entirely on defaults.
func configFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config dir %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".json" || ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

// parseServerConfig reads and decodes a single server config file.
func parseServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read server config %s: %w", path, err)
	}
	var cfg ServerConfig
	if err := decode(path, data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// parseAgentConfig reads and decodes a single agent config file.
func parseAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent config %s: %w", path, err)
	}
	var cfg AgentConfig
	if err := decode(path, data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// decode dispatches to the correct unmarshaller based on the file extension.
func decode(path string, data []byte, v interface{}) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, v); err != nil {
			return fmt.Errorf("yaml: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, v); err != nil {
			return fmt.Errorf("json: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file extension %q", ext)
	}
	return nil
}

// mergeServerConfig merges src into dst. Scalar fields from src override dst
// when non-zero; slice fields are appended.
func mergeServerConfig(dst, src *ServerConfig) {
	if src.Server.Address != "" {
		dst.Server.Address = src.Server.Address
	}
	if src.Server.Port != 0 {
		dst.Server.Port = src.Server.Port
	}
	if src.Server.ConfigPath != "" {
		dst.Server.ConfigPath = src.Server.ConfigPath
	}
	if src.Server.LogLevel != "" {
		dst.Server.LogLevel = src.Server.LogLevel
	}
	if src.Server.TLSCertFile != "" {
		dst.Server.TLSCertFile = src.Server.TLSCertFile
	}
	if src.Server.TLSKeyFile != "" {
		dst.Server.TLSKeyFile = src.Server.TLSKeyFile
	}
	if src.Server.HeartbeatTimeout != 0 {
		dst.Server.HeartbeatTimeout = src.Server.HeartbeatTimeout
	}
	dst.Twins = append(dst.Twins, src.Twins...)
	dst.Listeners = append(dst.Listeners, src.Listeners...)
}

// mergeAgentConfig merges src into dst.
func mergeAgentConfig(dst, src *AgentConfig) {
	if src.Agent.ID != "" {
		dst.Agent.ID = src.Agent.ID
	}
	if src.Agent.Name != "" {
		dst.Agent.Name = src.Agent.Name
	}
	if src.Agent.ServerAddr != "" {
		dst.Agent.ServerAddr = src.Agent.ServerAddr
	}
	if src.Agent.ConfigPath != "" {
		dst.Agent.ConfigPath = src.Agent.ConfigPath
	}
	if src.Agent.LogLevel != "" {
		dst.Agent.LogLevel = src.Agent.LogLevel
	}
	if src.Agent.TickInterval != 0 {
		dst.Agent.TickInterval = src.Agent.TickInterval
	}
	if src.Agent.TLSCACertFile != "" {
		dst.Agent.TLSCACertFile = src.Agent.TLSCACertFile
	}
	mergeDistribution(&dst.Agent.Distribution, &src.Agent.Distribution)
	dst.Skills = append(dst.Skills, src.Skills...)
}

// mergeDistribution merges src distribution settings into dst.
func mergeDistribution(dst, src *DistributionSettings) {
	if src.Mode != "" {
		dst.Mode = src.Mode
	}
	if src.RemoteURL != "" {
		dst.RemoteURL = src.RemoteURL
	}
	if src.LocalDir != "" {
		dst.LocalDir = src.LocalDir
	}
	if src.TLSVerify != nil {
		dst.TLSVerify = src.TLSVerify
	}
	if src.TimeoutSeconds != 0 {
		dst.TimeoutSeconds = src.TimeoutSeconds
	}
}

// applyServerDefaults fills in zero-value fields with documented defaults.
func applyServerDefaults(cfg *ServerConfig) {
	if cfg.Server.Address == "" {
		cfg.Server.Address = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 50051
	}
	if cfg.Server.LogLevel == "" {
		cfg.Server.LogLevel = "info"
	}
	if cfg.Server.HeartbeatTimeout == 0 {
		cfg.Server.HeartbeatTimeout = 30e9 // 30s in nanoseconds
	}
}

// applyAgentDefaults fills in zero-value fields with documented defaults.
func applyAgentDefaults(cfg *AgentConfig) {
	if cfg.Agent.LogLevel == "" {
		cfg.Agent.LogLevel = "info"
	}
	if cfg.Agent.TickInterval == 0 {
		cfg.Agent.TickInterval = 5e9 // 5s in nanoseconds
	}
	if cfg.Agent.Distribution.Mode == "" {
		cfg.Agent.Distribution.Mode = "server"
	}
	if cfg.Agent.Distribution.TLSVerify == nil {
		t := true
		cfg.Agent.Distribution.TLSVerify = &t
	}
	if cfg.Agent.Distribution.TimeoutSeconds == 0 {
		cfg.Agent.Distribution.TimeoutSeconds = 10
	}
}
