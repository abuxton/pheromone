package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/config"
)

// ---- Loader tests -----------------------------------------------------------

func TestLoadServerConfig_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.LoadServerConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Defaults must be applied even with no config files.
	if cfg.Server.Port != 50051 {
		t.Errorf("expected default port 50051, got %d", cfg.Server.Port)
	}
	if cfg.Server.Address != "0.0.0.0" {
		t.Errorf("expected default address 0.0.0.0, got %q", cfg.Server.Address)
	}
	if cfg.Server.LogLevel != "info" {
		t.Errorf("expected default log level info, got %q", cfg.Server.LogLevel)
	}
}

func TestLoadServerConfig_NonExistentDir(t *testing.T) {
	cfg, err := config.LoadServerConfig("/tmp/pheromone-nonexistent-dir-xyz")
	if err != nil {
		t.Fatalf("expected no error for missing dir, got: %v", err)
	}
	// Must still return a valid config with defaults.
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestLoadServerConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	content := `{
  "server": {
    "address": "127.0.0.1",
    "port": 9090,
    "log_level": "debug"
  }
}`
	writeFile(t, dir, "server.json", content)

	cfg, err := config.LoadServerConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Address != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %q", cfg.Server.Address)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("expected debug, got %q", cfg.Server.LogLevel)
	}
}

func TestLoadServerConfig_YAML(t *testing.T) {
	dir := t.TempDir()
	content := `
server:
  address: "10.0.0.1"
  port: 8080
  log_level: warn
`
	writeFile(t, dir, "server.yaml", content)

	cfg, err := config.LoadServerConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Address != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %q", cfg.Server.Address)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected 8080, got %d", cfg.Server.Port)
	}
}

func TestLoadServerConfig_YML(t *testing.T) {
	dir := t.TempDir()
	content := `
server:
  port: 7070
`
	writeFile(t, dir, "config.yml", content)

	cfg, err := config.LoadServerConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 7070 {
		t.Errorf("expected 7070, got %d", cfg.Server.Port)
	}
}

func TestLoadServerConfig_MultipleFiles_MergeOrder(t *testing.T) {
	dir := t.TempDir()
	// 00-base sets port 1000, 10-override raises it to 2000.
	writeFile(t, dir, "00-base.json", `{"server":{"port":1000,"log_level":"info"}}`)
	writeFile(t, dir, "10-override.json", `{"server":{"port":2000}}`)

	cfg, err := config.LoadServerConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 2000 {
		t.Errorf("expected overridden port 2000, got %d", cfg.Server.Port)
	}
	// log_level from 00-base should survive because 10-override did not set it.
	if cfg.Server.LogLevel != "info" {
		t.Errorf("expected log_level info from base, got %q", cfg.Server.LogLevel)
	}
}

func TestLoadServerConfig_TwinsAppended(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a-twins.yaml", `
twins:
  - id: twin-1
    type: os
`)
	writeFile(t, dir, "b-twins.yaml", `
twins:
  - id: twin-2
    type: workload
`)

	cfg, err := config.LoadServerConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Twins) != 2 {
		t.Errorf("expected 2 twins after merge, got %d", len(cfg.Twins))
	}
}

func TestLoadServerConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.json", `{not valid json`)
	_, err := config.LoadServerConfig(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadServerConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.yaml", "key: [unclosed")
	_, err := config.LoadServerConfig(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadServerConfig_IgnoresNonConfigFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# docs")
	writeFile(t, dir, "notes.txt", "some notes")
	writeFile(t, dir, "server.json", `{"server":{"port":1234}}`)

	cfg, err := config.LoadServerConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 1234 {
		t.Errorf("expected 1234, got %d", cfg.Server.Port)
	}
}

func TestLoadAgentConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.LoadAgentConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Agent.LogLevel != "info" {
		t.Errorf("expected info, got %q", cfg.Agent.LogLevel)
	}
	if cfg.Agent.TickInterval != 5*time.Second {
		t.Errorf("expected 5s tick, got %v", cfg.Agent.TickInterval)
	}
	if cfg.Agent.Distribution.Mode != "server" {
		t.Errorf("expected server distribution mode, got %q", cfg.Agent.Distribution.Mode)
	}
}

func TestLoadAgentConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "agent.json", `{
  "agent": {
    "id": "agent-001",
    "name": "test-agent",
    "server_addr": "grpc.internal:50051",
    "log_level": "debug"
  }
}`)
	cfg, err := config.LoadAgentConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Agent.ID != "agent-001" {
		t.Errorf("expected agent-001, got %q", cfg.Agent.ID)
	}
	if cfg.Agent.ServerAddr != "grpc.internal:50051" {
		t.Errorf("expected grpc.internal:50051, got %q", cfg.Agent.ServerAddr)
	}
}

func TestLoadAgentConfig_YAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "agent.yaml", `
agent:
  id: agent-yaml
  name: yaml-agent
  server_addr: "localhost:50051"
  log_level: info
`)
	cfg, err := config.LoadAgentConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Agent.ID != "agent-yaml" {
		t.Errorf("expected agent-yaml, got %q", cfg.Agent.ID)
	}
}

func TestLoadAgentConfig_SkillsAppended(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "01-skills.yaml", `
skills:
  - name: digital-twin
    version: "1.0.0"
`)
	writeFile(t, dir, "02-skills.yaml", `
skills:
  - name: metrics
    version: "1.0.0"
`)
	cfg, err := config.LoadAgentConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(cfg.Skills))
	}
}

// ---- Generator tests --------------------------------------------------------

func TestGenerateServerConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")

	if err := config.GenerateServerConfig(path, config.FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("generated file is empty")
	}

	// The generated file must be loadable.
	dir2 := t.TempDir()
	writeFile(t, dir2, "server.json", string(data))
	cfg, err := config.LoadServerConfig(dir2)
	if err != nil {
		t.Fatalf("generated config not loadable: %v", err)
	}
	if cfg.Server.Port == 0 {
		t.Error("loaded config has zero port")
	}
}

func TestGenerateServerConfig_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yaml")

	if err := config.GenerateServerConfig(path, config.FormatYAML); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Fatal("generated YAML file is empty")
	}

	dir2 := t.TempDir()
	writeFile(t, dir2, "server.yaml", string(data))
	cfg, err := config.LoadServerConfig(dir2)
	if err != nil {
		t.Fatalf("generated YAML config not loadable: %v", err)
	}
	if cfg.Server.Port == 0 {
		t.Error("loaded YAML config has zero port")
	}
}

func TestGenerateAgentConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.json")

	if err := config.GenerateAgentConfig(path, config.FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dir2 := t.TempDir()
	data, _ := os.ReadFile(path)
	writeFile(t, dir2, "agent.json", string(data))
	cfg, err := config.LoadAgentConfig(dir2)
	if err != nil {
		t.Fatalf("generated agent config not loadable: %v", err)
	}
	if cfg.Agent.Name == "" {
		t.Error("expected non-empty agent name in generated config")
	}
}

func TestGenerateAgentConfig_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")

	if err := config.GenerateAgentConfig(path, config.FormatYAML); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dir2 := t.TempDir()
	data, _ := os.ReadFile(path)
	writeFile(t, dir2, "agent.yaml", string(data))
	cfg, err := config.LoadAgentConfig(dir2)
	if err != nil {
		t.Fatalf("generated YAML agent config not loadable: %v", err)
	}
	if cfg.Agent.Name == "" {
		t.Error("expected non-empty agent name in generated YAML config")
	}
}

func TestGenerateTwinConfig(t *testing.T) {
	dir := t.TempDir()
	for _, format := range []config.Format{config.FormatJSON, config.FormatYAML} {
		ext := "json"
		if format == config.FormatYAML {
			ext = "yaml"
		}
		path := filepath.Join(dir, "twin."+ext)
		if err := config.GenerateTwinConfig(path, format); err != nil {
			t.Fatalf("format %s: unexpected error: %v", format, err)
		}
		data, _ := os.ReadFile(path)
		if len(data) == 0 {
			t.Errorf("format %s: generated file is empty", format)
		}
	}
}

func TestGenerateListenerConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listener.json")
	if err := config.GenerateListenerConfig(path, config.FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateSkillConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skill.yaml")
	if err := config.GenerateSkillConfig(path, config.FormatYAML); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFormat(t *testing.T) {
	cases := []struct {
		input   string
		want    config.Format
		wantErr bool
	}{
		{"json", config.FormatJSON, false},
		{"JSON", config.FormatJSON, false},
		{"yaml", config.FormatYAML, false},
		{"YAML", config.FormatYAML, false},
		{"yml", config.FormatYAML, false},
		{"toml", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		got, err := config.ParseFormat(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseFormat(%q): expected error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseFormat(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseFormat(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---- Validator tests --------------------------------------------------------

func TestValidateServerConfig_Valid(t *testing.T) {
	cfg := &config.ServerConfig{
		Server: config.ServerSettings{
			Address:  "0.0.0.0",
			Port:     50051,
			LogLevel: "info",
		},
		Twins: []config.TwinConfig{
			{ID: "t1", Type: "os"},
		},
		Listeners: []config.ListenerConfig{
			{Name: "wh", Type: "webhook", Address: ":8080"},
		},
	}
	if err := config.ValidateServerConfig(cfg); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateServerConfig_InvalidPort(t *testing.T) {
	cfg := &config.ServerConfig{
		Server: config.ServerSettings{Port: 0},
	}
	err := config.ValidateServerConfig(cfg)
	if err == nil {
		t.Fatal("expected error for port 0")
	}
}

func TestValidateServerConfig_InvalidLogLevel(t *testing.T) {
	cfg := &config.ServerConfig{
		Server: config.ServerSettings{Port: 50051, LogLevel: "verbose"},
	}
	if err := config.ValidateServerConfig(cfg); err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestValidateServerConfig_TLSPartial(t *testing.T) {
	cfg := &config.ServerConfig{
		Server: config.ServerSettings{Port: 50051, TLSCertFile: "/cert.pem"},
	}
	if err := config.ValidateServerConfig(cfg); err == nil {
		t.Fatal("expected error for TLS with only cert (no key)")
	}
}

func TestValidateServerConfig_DuplicateTwinID(t *testing.T) {
	cfg := &config.ServerConfig{
		Server: config.ServerSettings{Port: 50051},
		Twins: []config.TwinConfig{
			{ID: "twin-1", Type: "os"},
			{ID: "twin-1", Type: "workload"},
		},
	}
	if err := config.ValidateServerConfig(cfg); err == nil {
		t.Fatal("expected error for duplicate twin ID")
	}
}

func TestValidateServerConfig_InvalidTwinType(t *testing.T) {
	cfg := &config.ServerConfig{
		Server: config.ServerSettings{Port: 50051},
		Twins:  []config.TwinConfig{{ID: "t1", Type: "container"}},
	}
	if err := config.ValidateServerConfig(cfg); err == nil {
		t.Fatal("expected error for invalid twin type")
	}
}

func TestValidateServerConfig_InvalidListenerType(t *testing.T) {
	cfg := &config.ServerConfig{
		Server:    config.ServerSettings{Port: 50051},
		Listeners: []config.ListenerConfig{{Name: "l", Type: "mqtt", Address: ":1883"}},
	}
	if err := config.ValidateServerConfig(cfg); err == nil {
		t.Fatal("expected error for invalid listener type")
	}
}

func TestValidateAgentConfig_Valid(t *testing.T) {
	cfg := &config.AgentConfig{
		Agent: config.AgentSettings{
			ServerAddr: "localhost:50051",
			LogLevel:   "info",
		},
	}
	if err := config.ValidateAgentConfig(cfg); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateAgentConfig_MissingServerAddr(t *testing.T) {
	cfg := &config.AgentConfig{}
	if err := config.ValidateAgentConfig(cfg); err == nil {
		t.Fatal("expected error for missing server_addr")
	}
}

func TestValidateAgentConfig_InvalidDistributionMode(t *testing.T) {
	cfg := &config.AgentConfig{
		Agent: config.AgentSettings{
			ServerAddr: "localhost:50051",
			Distribution: config.DistributionSettings{
				Mode: "ftp",
			},
		},
	}
	if err := config.ValidateAgentConfig(cfg); err == nil {
		t.Fatal("expected error for invalid distribution mode")
	}
}

func TestValidateAgentConfig_RemoteWithoutURL(t *testing.T) {
	cfg := &config.AgentConfig{
		Agent: config.AgentSettings{
			ServerAddr: "localhost:50051",
			Distribution: config.DistributionSettings{
				Mode: "remote",
			},
		},
	}
	if err := config.ValidateAgentConfig(cfg); err == nil {
		t.Fatal("expected error for remote mode without URL")
	}
}

func TestValidateAgentConfig_LocalWithoutDir(t *testing.T) {
	cfg := &config.AgentConfig{
		Agent: config.AgentSettings{
			ServerAddr: "localhost:50051",
			Distribution: config.DistributionSettings{
				Mode: "local",
			},
		},
	}
	if err := config.ValidateAgentConfig(cfg); err == nil {
		t.Fatal("expected error for local mode without dir")
	}
}

func TestValidateAgentConfig_DuplicateSkillName(t *testing.T) {
	cfg := &config.AgentConfig{
		Agent: config.AgentSettings{ServerAddr: "localhost:50051"},
		Skills: []config.SkillConfig{
			{Name: "digital-twin"},
			{Name: "digital-twin"},
		},
	}
	if err := config.ValidateAgentConfig(cfg); err == nil {
		t.Fatal("expected error for duplicate skill name")
	}
}

// ---- helpers ----------------------------------------------------------------

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

// ---- UITLSConfig validator tests -------------------------------------------

func validUIConfig() config.UIConfig {
	return config.UIConfig{
		Enabled:   true,
		Port:      8081,
		SecretKey: "test-secret",
	}
}

func TestValidateUIConfig_TLS_Disabled(t *testing.T) {
	cfg := validUIConfig()
	cfg.TLS = config.UITLSConfig{Enabled: false}
	if err := config.ValidateUIConfig(&cfg); err != nil {
		t.Errorf("expected valid when TLS disabled, got: %v", err)
	}
}

func TestValidateUIConfig_TLS_ValidExplicitCerts(t *testing.T) {
	cfg := validUIConfig()
	cfg.TLS = config.UITLSConfig{
		Enabled:  true,
		CertFile: "/etc/pheromone/tls/server.crt",
		KeyFile:  "/etc/pheromone/tls/server.key",
	}
	if err := config.ValidateUIConfig(&cfg); err != nil {
		t.Errorf("expected valid with explicit cert+key, got: %v", err)
	}
}

func TestValidateUIConfig_TLS_ValidWithCABundle(t *testing.T) {
	cfg := validUIConfig()
	cfg.TLS = config.UITLSConfig{
		Enabled:      true,
		CertFile:     "/etc/pheromone/tls/server.crt",
		KeyFile:      "/etc/pheromone/tls/server.key",
		CABundleFile: "/etc/pheromone/tls/ca-bundle.pem",
	}
	if err := config.ValidateUIConfig(&cfg); err != nil {
		t.Errorf("expected valid with cert+key+ca_bundle, got: %v", err)
	}
}

func TestValidateUIConfig_TLS_ValidOSCertStore(t *testing.T) {
	cfg := validUIConfig()
	cfg.TLS = config.UITLSConfig{
		Enabled:        true,
		CertFile:       "/etc/pheromone/tls/server.crt",
		KeyFile:        "/etc/pheromone/tls/server.key",
		UseOSCertStore: true,
	}
	if err := config.ValidateUIConfig(&cfg); err != nil {
		t.Errorf("expected valid with OS cert store, got: %v", err)
	}
}

func TestValidateUIConfig_TLS_MissingCert(t *testing.T) {
	cfg := validUIConfig()
	cfg.TLS = config.UITLSConfig{
		Enabled: true,
		KeyFile: "/etc/pheromone/tls/server.key",
	}
	if err := config.ValidateUIConfig(&cfg); err == nil {
		t.Fatal("expected error for missing cert_file")
	}
}

func TestValidateUIConfig_TLS_MissingKey(t *testing.T) {
	cfg := validUIConfig()
	cfg.TLS = config.UITLSConfig{
		Enabled:  true,
		CertFile: "/etc/pheromone/tls/server.crt",
	}
	if err := config.ValidateUIConfig(&cfg); err == nil {
		t.Fatal("expected error for missing key_file")
	}
}

func TestValidateUIConfig_TLS_MissingBoth(t *testing.T) {
	cfg := validUIConfig()
	cfg.TLS = config.UITLSConfig{Enabled: true}
	err := config.ValidateUIConfig(&cfg)
	if err == nil {
		t.Fatal("expected error for missing cert_file and key_file")
	}
}

func TestValidateUIConfig_TLS_OSCertStoreAndCABundle_Conflict(t *testing.T) {
	cfg := validUIConfig()
	cfg.TLS = config.UITLSConfig{
		Enabled:        true,
		CertFile:       "/etc/pheromone/tls/server.crt",
		KeyFile:        "/etc/pheromone/tls/server.key",
		UseOSCertStore: true,
		CABundleFile:   "/etc/pheromone/tls/ca-bundle.pem",
	}
	if err := config.ValidateUIConfig(&cfg); err == nil {
		t.Fatal("expected error when both use_os_cert_store and ca_bundle_file are set")
	}
}
