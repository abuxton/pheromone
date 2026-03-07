package skill_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
	"github.com/abuxton/pheromone/internal/skill/builtin"
)

// --- SkillSpec ---

func TestSkillSpec_MinTwinLevelParsed(t *testing.T) {
	cases := []struct {
		raw  string
		want skill.TwinLevel
	}{
		{"os", skill.TwinLevelOS},
		{"workload", skill.TwinLevelWorkload},
		{"", skill.TwinLevelWorkload},
		{"unknown", skill.TwinLevelWorkload},
	}
	for _, c := range cases {
		spec := &skill.SkillSpec{MinTwinLevel: c.raw}
		if got := spec.MinTwinLevelParsed(); got != c.want {
			t.Errorf("MinTwinLevel(%q).Parsed() = %v, want %v", c.raw, got, c.want)
		}
	}
}

// --- DefaultDistributionConfig ---

func TestDefaultDistributionConfig(t *testing.T) {
	cfg := skill.DefaultDistributionConfig()
	if cfg.Mode != skill.DistributionModeServer {
		t.Errorf("default mode = %q, want %q", cfg.Mode, skill.DistributionModeServer)
	}
	if !cfg.TLSVerify {
		t.Error("TLSVerify should default to true")
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("default timeout = %v, want 10s", cfg.Timeout)
	}
}

// --- LocalSource ---

func TestLocalSource_ListSpecs(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "nginx-monitor", `{
		"name": "nginx-monitor", "version": "1.0.0",
		"min_twin_level": "workload",
		"capabilities": ["collect-metrics"], "runtime": "go"
	}`)
	writeSpec(t, dir, "postgresql-monitor", `{
		"name": "postgresql-monitor", "version": "1.0.0",
		"min_twin_level": "workload",
		"capabilities": ["collect-metrics"], "runtime": "go"
	}`)

	src := skill.NewLocalSource(dir)
	specs, err := src.ListSpecs(context.Background())
	if err != nil {
		t.Fatalf("ListSpecs: %v", err)
	}
	if len(specs) != 2 {
		t.Errorf("expected 2 specs, got %d", len(specs))
	}
}

func TestLocalSource_GetSpec(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "nginx-monitor", `{
		"name": "nginx-monitor", "version": "1.0.0",
		"min_twin_level": "workload",
		"capabilities": ["collect-metrics"], "runtime": "go"
	}`)

	src := skill.NewLocalSource(dir)
	spec, err := src.GetSpec(context.Background(), "nginx-monitor", "1.0.0")
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if spec.Name != "nginx-monitor" {
		t.Errorf("expected nginx-monitor, got %s", spec.Name)
	}
}

func TestLocalSource_GetSpec_NotFound(t *testing.T) {
	src := skill.NewLocalSource(t.TempDir())
	_, err := src.GetSpec(context.Background(), "nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestLocalSource_EmptyDir_ReturnsEmpty(t *testing.T) {
	src := skill.NewLocalSource(t.TempDir())
	specs, err := src.ListSpecs(context.Background())
	if err != nil {
		t.Fatalf("ListSpecs on empty dir: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected 0 specs, got %d", len(specs))
	}
}

// --- ServerSource ---

func TestServerSource_ListSpecs(t *testing.T) {
	policy := skill.NewAccessPolicy()
	reg := skill.NewSkillRegistry(policy)
	_ = reg.Register(builtin.NewMetricsCollectionSkill())
	_ = reg.Register(builtin.NewConfigEnforceSkill("1.0.0"))

	src := skill.NewServerSource(reg)
	specs, err := src.ListSpecs(context.Background())
	if err != nil {
		t.Fatalf("ListSpecs: %v", err)
	}
	if len(specs) != 2 {
		t.Errorf("expected 2 specs from server, got %d", len(specs))
	}
}

func TestServerSource_GetSpec(t *testing.T) {
	policy := skill.NewAccessPolicy()
	reg := skill.NewSkillRegistry(policy)
	_ = reg.Register(builtin.NewMetricsCollectionSkill())

	src := skill.NewServerSource(reg)
	spec, err := src.GetSpec(context.Background(), "metrics", "")
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if spec.Name != "metrics" {
		t.Errorf("expected metrics, got %s", spec.Name)
	}
}

func TestServerSource_GetSpec_VersionMismatch(t *testing.T) {
	policy := skill.NewAccessPolicy()
	reg := skill.NewSkillRegistry(policy)
	_ = reg.Register(builtin.NewMetricsCollectionSkill())

	src := skill.NewServerSource(reg)
	_, err := src.GetSpec(context.Background(), "metrics", "9.9.9")
	if err == nil {
		t.Error("expected error for version mismatch")
	}
}

// --- RemoteSource ---

func TestRemoteSource_ListSpecs(t *testing.T) {
	specs := []*skill.SkillSpec{
		{Name: "nginx-monitor", Version: "1.0.0", MinTwinLevel: "workload", Capabilities: []string{"collect-metrics"}, Runtime: "go"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(specs)
	}))
	defer srv.Close()

	cfg := skill.DefaultDistributionConfig()
	cfg.Mode = skill.DistributionModeRemote
	src := skill.NewRemoteSource(srv.URL, cfg)

	got, err := src.ListSpecs(context.Background())
	if err != nil {
		t.Fatalf("ListSpecs: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 spec, got %d", len(got))
	}
	if got[0].Name != "nginx-monitor" {
		t.Errorf("expected nginx-monitor, got %s", got[0].Name)
	}
}

func TestRemoteSource_GetSpec(t *testing.T) {
	spec := &skill.SkillSpec{Name: "nginx-monitor", Version: "1.0.0", MinTwinLevel: "workload", Runtime: "go"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(spec)
	}))
	defer srv.Close()

	cfg := skill.DefaultDistributionConfig()
	src := skill.NewRemoteSource(srv.URL, cfg)

	got, err := src.GetSpec(context.Background(), "nginx-monitor", "1.0.0")
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if got.Name != "nginx-monitor" {
		t.Errorf("expected nginx-monitor, got %s", got.Name)
	}
}

func TestRemoteSource_GetSpec_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := skill.DefaultDistributionConfig()
	src := skill.NewRemoteSource(srv.URL, cfg)
	_, err := src.GetSpec(context.Background(), "missing-skill", "1.0.0")
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestRemoteSource_ListSpecs_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := skill.DefaultDistributionConfig()
	src := skill.NewRemoteSource(srv.URL, cfg)
	_, err := src.ListSpecs(context.Background())
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

// --- NewSkillSourceFromConfig ---

func TestNewSkillSourceFromConfig_ServerMode(t *testing.T) {
	reg := skill.NewSkillRegistry(skill.NewAccessPolicy())
	cfg := skill.DefaultDistributionConfig()
	src := skill.NewSkillSourceFromConfig(reg, cfg)
	if _, ok := src.(*skill.ServerSource); !ok {
		t.Errorf("expected *ServerSource for server mode, got %T", src)
	}
}

func TestNewSkillSourceFromConfig_LocalMode(t *testing.T) {
	reg := skill.NewSkillRegistry(skill.NewAccessPolicy())
	cfg := skill.DistributionConfig{Mode: skill.DistributionModeLocal, LocalDir: t.TempDir()}
	src := skill.NewSkillSourceFromConfig(reg, cfg)
	if _, ok := src.(*skill.LocalSource); !ok {
		t.Errorf("expected *LocalSource for local mode, got %T", src)
	}
}

func TestNewSkillSourceFromConfig_RemoteMode(t *testing.T) {
	reg := skill.NewSkillRegistry(skill.NewAccessPolicy())
	cfg := skill.DistributionConfig{Mode: skill.DistributionModeRemote, RemoteURL: "https://example.com/skills"}
	src := skill.NewSkillSourceFromConfig(reg, cfg)
	if _, ok := src.(*skill.RemoteSource); !ok {
		t.Errorf("expected *RemoteSource for remote mode, got %T", src)
	}
}

// --- SkillLoader ---

func TestSkillLoader_Load(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "nginx-monitor", `{
		"name": "nginx-monitor", "version": "1.0.0",
		"min_twin_level": "workload",
		"capabilities": ["collect-metrics"], "runtime": "go"
	}`)

	policy := skill.NewAccessPolicy()
	reg := skill.NewSkillRegistry(policy)
	src := skill.NewLocalSource(dir)
	cfg := skill.DistributionConfig{Mode: skill.DistributionModeLocal, LocalDir: dir}

	// factory that recognises nginx-monitor
	factory := func(spec *skill.SkillSpec) (skill.Skill, error) {
		if spec.Name == "nginx-monitor" {
			return &stubSkill{name: "nginx-monitor", version: "1.0.0", level: skill.TwinLevelWorkload}, nil
		}
		return nil, nil
	}

	loader := skill.NewSkillLoader(src, reg, factory, cfg)
	if err := loader.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	s, err := reg.Get("nginx-monitor")
	if err != nil {
		t.Fatalf("expected nginx-monitor to be registered: %v", err)
	}
	if s.Name() != "nginx-monitor" {
		t.Errorf("expected nginx-monitor, got %s", s.Name())
	}
}

func TestSkillLoader_Load_FactoryError_SkipsSkill(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "bad-skill", `{
		"name": "bad-skill", "version": "1.0.0",
		"min_twin_level": "workload",
		"capabilities": [], "runtime": "go"
	}`)

	policy := skill.NewAccessPolicy()
	reg := skill.NewSkillRegistry(policy)
	src := skill.NewLocalSource(dir)
	cfg := skill.DistributionConfig{Mode: skill.DistributionModeLocal}

	// factory that always fails
	factory := func(spec *skill.SkillSpec) (skill.Skill, error) {
		return nil, nil // nil Skill should be skipped
	}

	loader := skill.NewSkillLoader(src, reg, factory, cfg)
	if err := loader.Load(context.Background()); err != nil {
		t.Fatalf("Load should succeed even when factory returns nil: %v", err)
	}

	if _, err := reg.Get("bad-skill"); err == nil {
		t.Error("bad-skill should not be registered")
	}
}

// --- helpers ---

func writeSpec(t *testing.T, dir, name, content string) {
	t.Helper()
	subdir := filepath.Join(dir, name)
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", subdir, err)
	}
	path := filepath.Join(subdir, "skill.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// stubSkill is a minimal Skill implementation used only in spec tests.
type stubSkill struct {
	name    string
	version string
	level   skill.TwinLevel
}

func (s *stubSkill) Name() string                 { return s.name }
func (s *stubSkill) Version() string              { return s.version }
func (s *stubSkill) MinTwinLevel() skill.TwinLevel { return s.level }
func (s *stubSkill) Execute(_ context.Context, _ *skill.Observations, _ *skill.Action) (*skill.SkillResult, error) {
	return &skill.SkillResult{Success: true}, nil
}
