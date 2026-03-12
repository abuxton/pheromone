package skill

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// SkillSpec is the JSON manifest for a Pheromone skill, aligned with the
// agentskills.io specification (https://agentskills.io/specification).
//
// Each skill published to a registry or bundled with an agent MUST include a
// spec file (skill.json) that conforms to this structure. The spec provides:
//   - Identity metadata (name, version, author, description)
//   - Access control requirements (min_twin_level)
//   - Capability advertisement (capabilities list)
//   - Optional source reference for direct-pull distribution
//
// Example skill.json:
//
//	{
//	  "name": "nginx-monitor",
//	  "version": "1.0.0",
//	  "description": "Monitor Nginx web server metrics",
//	  "author": "platform-team",
//	  "min_twin_level": "workload",
//	  "tags": ["monitoring", "nginx"],
//	  "capabilities": ["collect-metrics"],
//	  "runtime": "go",
//	  "source": {
//	    "repository": "https://github.com/example/nginx-skill",
//	    "ref": "v1.0.0",
//	    "entry_point": "NewNginxMonitorSkill"
//	  }
//	}
type SkillSpec struct {
	// Name is the unique skill identifier, e.g. "nginx-monitor".
	Name string `json:"name"`

	// Version is the semver version of this skill, e.g. "1.0.0".
	Version string `json:"version"`

	// Description is a human-readable summary of the skill's purpose.
	Description string `json:"description"`

	// Author is the individual or team that published this skill.
	Author string `json:"author,omitempty"`

	// MinTwinLevel is the minimum twin level required to invoke this skill.
	// Valid values: "os", "workload". Defaults to "workload" if omitted.
	MinTwinLevel string `json:"min_twin_level"`

	// Tags are classification labels used by the registry for search and filtering.
	Tags []string `json:"tags,omitempty"`

	// Capabilities lists the action types this skill supports,
	// e.g. ["collect-metrics", "apply-config"].
	Capabilities []string `json:"capabilities"`

	// Runtime identifies the execution environment, e.g. "go", "python", "rust".
	Runtime string `json:"runtime,omitempty"`

	// Source describes where the skill implementation can be obtained for
	// direct-pull distribution. Omitted for server-distributed or bundled skills.
	Source *SkillSpecSource `json:"source,omitempty"`
}

// MinTwinLevelParsed converts the spec's MinTwinLevel string to a TwinLevel constant.
// An unrecognised value defaults to TwinLevelWorkload (most permissive).
func (s *SkillSpec) MinTwinLevelParsed() TwinLevel {
	switch s.MinTwinLevel {
	case "os":
		return TwinLevelOS
	default:
		return TwinLevelWorkload
	}
}

// SkillSpecSource describes where a skill implementation can be obtained for
// direct-pull distribution (agentskills.io registry or a private Git/HTTP source).
type SkillSpecSource struct {
	// Repository is the VCS or registry URL, e.g.
	// "https://agentskills.io/skills/nginx-monitor" or a private Git URL.
	Repository string `json:"repository"`

	// Ref is the version tag, commit SHA, or branch to fetch, e.g. "v1.0.0".
	Ref string `json:"ref,omitempty"`

	// EntryPoint is the constructor symbol used to instantiate the skill in Go,
	// e.g. "NewNginxMonitorSkill". Used by the SkillFactory at load time.
	EntryPoint string `json:"entry_point,omitempty"`
}

// DistributionMode controls how an agent obtains its skill specs.
type DistributionMode string

const (
	// DistributionModeServer means the Pheromone server holds the canonical
	// spec registry and distributes filtered skill bundles to agents at
	// registration time via BundleFor. No direct internet access is required.
	// This is the default and the correct choice for secure / air-gapped environments.
	DistributionModeServer DistributionMode = "server"

	// DistributionModeRemote means the agent fetches skill specs directly from a
	// configurable URL (e.g. https://agentskills.io/api/skills or a private registry).
	// Requires outbound HTTP/HTTPS access from the agent host.
	DistributionModeRemote DistributionMode = "remote"

	// DistributionModeLocal means skill specs are loaded from a directory on the
	// local filesystem. Fully air-gapped; no network access required.
	DistributionModeLocal DistributionMode = "local"
)

// DistributionConfig configures how an agent obtains its skill specs.
//
// For secure environments without open egress, use DistributionModeServer (default)
// or DistributionModeLocal. DistributionModeRemote requires outbound HTTP access.
type DistributionConfig struct {
	// Mode selects the distribution strategy. Defaults to DistributionModeServer.
	Mode DistributionMode

	// RemoteURL is the skill registry base URL used when Mode == DistributionModeRemote.
	// Example: "https://agentskills.io/api/skills" or "https://registry.internal/skills".
	// Leave empty to use the default agentskills.io public registry URL.
	RemoteURL string

	// LocalDir is the filesystem directory scanned for skill.json spec files
	// when Mode == DistributionModeLocal.
	LocalDir string

	// TLSVerify controls TLS certificate verification for remote HTTP calls.
	// Set to false only in development/testing environments. Defaults to true.
	TLSVerify bool

	// Timeout is the per-request timeout for remote HTTP calls. Defaults to 10 s.
	Timeout time.Duration
}

// DefaultDistributionConfig returns a server-distribution config (safe for air-gapped use).
func DefaultDistributionConfig() DistributionConfig {
	return DistributionConfig{
		Mode:      DistributionModeServer,
		TLSVerify: true,
		Timeout:   10 * time.Second,
	}
}

// SkillSource is the interface for loading SkillSpecs from a configured backend.
type SkillSource interface {
	// ListSpecs returns all skill specs available from this source.
	ListSpecs(ctx context.Context) ([]*SkillSpec, error)

	// GetSpec returns the spec for a named skill at a specific version.
	// If version is empty, the latest available version is returned.
	GetSpec(ctx context.Context, name, version string) (*SkillSpec, error)
}

// SkillFactory constructs a runnable Skill from a SkillSpec.
// It is the integration point where spec metadata is connected to executable code.
// Return an error for specs whose runtime or entry_point is not supported by this factory.
type SkillFactory func(spec *SkillSpec) (Skill, error)

// SkillLoader loads SkillSpecs from a SkillSource and uses a SkillFactory to
// build runnable Skill implementations, then registers them with a SkillRegistry.
//
// Usage:
//
//	loader := skill.NewSkillLoader(source, registry, factory, skill.DefaultDistributionConfig())
//	if err := loader.Load(ctx); err != nil { ... }
type SkillLoader struct {
	source   SkillSource
	registry *SkillRegistry
	factory  SkillFactory
	cfg      DistributionConfig
	log      *slog.Logger
}

// NewSkillLoader creates a SkillLoader that uses source to discover specs, factory to
// instantiate them, and registry to register the resulting Skill implementations.
func NewSkillLoader(source SkillSource, registry *SkillRegistry, factory SkillFactory, cfg DistributionConfig) *SkillLoader {
	return &SkillLoader{
		source:   source,
		registry: registry,
		factory:  factory,
		cfg:      cfg,
		log:      slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// Load discovers all specs from the configured source, instantiates each one via the
// factory, and registers the resulting skills with the registry.
//
// Specs that cannot be instantiated by the factory are logged and skipped without
// failing the entire load — the remaining skills are still registered.
func (l *SkillLoader) Load(ctx context.Context) error {
	specs, err := l.source.ListSpecs(ctx)
	if err != nil {
		return fmt.Errorf("skill loader: list specs: %w", err)
	}

	loaded := 0
	for _, spec := range specs {
		s, factoryErr := l.factory(spec)
		if factoryErr != nil {
			l.log.Warn("skill factory could not instantiate spec",
				slog.String("skill", spec.Name),
				slog.String("version", spec.Version),
				slog.String("error", factoryErr.Error()),
			)
			continue
		}
		if s == nil {
			// Factory returns nil (no error) to signal that this spec is not supported
			// by the current factory implementation; skip without failing.
			l.log.Debug("skill factory returned nil for spec; skipping",
				slog.String("skill", spec.Name),
				slog.String("version", spec.Version),
			)
			continue
		}

		if regErr := l.registry.Register(s); regErr != nil {
			l.log.Warn("skill already registered; skipping",
				slog.String("skill", spec.Name),
				slog.String("error", regErr.Error()),
			)
			continue
		}
		loaded++
	}

	l.log.Info("skill loader complete",
		slog.String("mode", string(l.cfg.Mode)),
		slog.Int("specs_found", len(specs)),
		slog.Int("skills_loaded", loaded),
	)
	return nil
}

// --- SkillSource implementations ---

// LocalSource loads SkillSpecs from JSON files (named "skill.json") found in a
// local filesystem directory tree. Suitable for air-gapped / bundled deployments.
type LocalSource struct {
	dir string
}

// NewLocalSource creates a LocalSource that scans dir (recursively) for skill.json files.
func NewLocalSource(dir string) *LocalSource {
	return &LocalSource{dir: dir}
}

// ListSpecs walks the directory tree and parses every skill.json found.
func (s *LocalSource) ListSpecs(_ context.Context) ([]*SkillSpec, error) {
	var specs []*SkillSpec
	walkErr := filepath.WalkDir(s.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "skill.json" {
			return nil
		}
		spec, parseErr := parseSpecFile(path)
		if parseErr != nil {
			return fmt.Errorf("local source: parse %s: %w", path, parseErr)
		}
		specs = append(specs, spec)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("local source: walk %s: %w", s.dir, walkErr)
	}
	return specs, nil
}

// GetSpec returns the spec with the given name (and optionally version) from the directory.
// If version is empty the first matching name is returned.
func (s *LocalSource) GetSpec(ctx context.Context, name, version string) (*SkillSpec, error) {
	specs, err := s.ListSpecs(ctx)
	if err != nil {
		return nil, err
	}
	for _, spec := range specs {
		if spec.Name == name && (version == "" || spec.Version == version) {
			return spec, nil
		}
	}
	return nil, fmt.Errorf("local source: skill %q version %q not found", name, version)
}

// parseSpecFile reads and parses a single skill.json file.
func parseSpecFile(path string) (*SkillSpec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open spec file %s: %w", path, err)
	}
	defer f.Close()
	var spec SkillSpec
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, fmt.Errorf("decode spec file %s: %w", path, err)
	}
	return &spec, nil
}

// RemoteSource fetches SkillSpecs from an HTTP/HTTPS registry endpoint.
// It is compatible with the agentskills.io public registry and any private registry
// that serves the same JSON list format.
//
// Security note: use RemoteSource only when the agent host has controlled egress to
// the registry. For air-gapped environments use LocalSource or ServerSource instead.
type RemoteSource struct {
	baseURL string
	client  *http.Client
}

// NewRemoteSource creates a RemoteSource for the given base URL.
// If baseURL is empty, https://agentskills.io/api/skills is used.
// cfg controls TLS verification and request timeout.
func NewRemoteSource(baseURL string, cfg DistributionConfig) *RemoteSource {
	if baseURL == "" {
		baseURL = "https://agentskills.io/api/skills"
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !cfg.TLSVerify {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{} //nolint:gosec
		}
		transport.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // intentional dev/test opt-in
	}

	return &RemoteSource{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout, Transport: transport},
	}
}

// ListSpecs fetches the full skill list from the remote registry.
// The registry must return a JSON array of SkillSpec objects.
func (s *RemoteSource) ListSpecs(ctx context.Context) ([]*SkillSpec, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("remote source: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote source: fetch %s: %w", s.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote source: registry returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("remote source: read response: %w", err)
	}

	var specs []*SkillSpec
	if err := json.Unmarshal(body, &specs); err != nil {
		return nil, fmt.Errorf("remote source: parse response: %w", err)
	}
	return specs, nil
}

// GetSpec fetches a single skill spec from the remote registry.
// URL pattern: <baseURL>/<name> or <baseURL>/<name>/<version>.
func (s *RemoteSource) GetSpec(ctx context.Context, name, version string) (*SkillSpec, error) {
	url := s.baseURL + "/" + name
	if version != "" {
		url += "/" + version
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("remote source: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote source: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("remote source: skill %q version %q not found", name, version)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote source: registry returned %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("remote source: read response: %w", err)
	}

	var spec SkillSpec
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, fmt.Errorf("remote source: parse response: %w", err)
	}
	return &spec, nil
}

// ServerSource adapts a SkillRegistry for use as a SkillSource.
// The server holds the canonical set of SkillSpecs; agents call ListSpecs/GetSpec via
// this adapter to discover available skills without direct internet access.
// This is the correct choice for secure, air-gapped deployments.
type ServerSource struct {
	registry *SkillRegistry
}

// NewServerSource wraps an existing SkillRegistry as a SkillSource.
func NewServerSource(registry *SkillRegistry) *ServerSource {
	return &ServerSource{registry: registry}
}

// ListSpecs returns specs for all skills currently registered on the server.
func (s *ServerSource) ListSpecs(_ context.Context) ([]*SkillSpec, error) {
	skills := s.registry.List()
	specs := make([]*SkillSpec, 0, len(skills))
	for _, sk := range skills {
		specs = append(specs, skillToSpec(sk))
	}
	return specs, nil
}

// GetSpec returns the spec for a specific skill by name (and optionally version).
func (s *ServerSource) GetSpec(_ context.Context, name, version string) (*SkillSpec, error) {
	sk, err := s.registry.Get(name)
	if err != nil {
		return nil, fmt.Errorf("server source: %w", err)
	}
	spec := skillToSpec(sk)
	if version != "" && spec.Version != version {
		return nil, fmt.Errorf("server source: skill %q version %q not found (have %s)", name, version, spec.Version)
	}
	return spec, nil
}

// RuntimeNamed is an optional interface a Skill may implement to declare its runtime
// environment, e.g. "go", "python", "rust". Skills that do not implement this
// interface are assumed to run in the same process (Go).
type RuntimeNamed interface {
	Runtime() string
}

// skillToSpec synthesises a SkillSpec from the Skill interface.
// If the skill also implements RuntimeNamed its declared runtime is used; otherwise "go".
func skillToSpec(s Skill) *SkillSpec {
	runtime := "go"
	if rn, ok := s.(RuntimeNamed); ok {
		runtime = rn.Runtime()
	}
	return &SkillSpec{
		Name:         s.Name(),
		Version:      s.Version(),
		MinTwinLevel: s.MinTwinLevel().String(),
		Runtime:      runtime,
	}
}

// NewSkillSourceFromConfig creates the appropriate SkillSource for the given DistributionConfig.
// This is the primary factory used by agents to initialise their skill source:
//
//	cfg := skill.DefaultDistributionConfig()             // server mode (air-gapped safe)
//	cfg.Mode = skill.DistributionModeRemote              // switch to remote pull
//	cfg.RemoteURL = "https://registry.internal/skills"  // private registry
//
//	source := skill.NewSkillSourceFromConfig(registry, cfg)
func NewSkillSourceFromConfig(registry *SkillRegistry, cfg DistributionConfig) SkillSource {
	switch cfg.Mode {
	case DistributionModeRemote:
		return NewRemoteSource(cfg.RemoteURL, cfg)
	case DistributionModeLocal:
		return NewLocalSource(cfg.LocalDir)
	default:
		return NewServerSource(registry)
	}
}
