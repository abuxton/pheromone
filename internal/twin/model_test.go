package twin_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/abuxton/pheromone/internal/twin"
)

// repoRoot returns the absolute path to the repository root by walking up from
// this test file's location.  This lets us load the sample twin-models/ files
// without hard-coding absolute paths.
func repoRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	// filename is  …/internal/twin/model_test.go  → go up 3 levels
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

// sampleFile returns the path to a file inside twin-models/.
func sampleFile(name string) string {
	return filepath.Join(repoRoot(), "twin-models", name)
}

// ---------------------------------------------------------------------------
// ParseTwinModel — valid YAML
// ---------------------------------------------------------------------------

var validYAML = []byte(`
apiVersion: pheromone.io/v1
kind: TwinModel
metadata:
  name: test-model
  namespace: test
  version: "1.0.0"
spec:
  selectors:
    - label: "env=test"
  twins:
    - name: os-config
      type: os-level
      desired_state:
        kernel_version: "5.15.0"
`)

func TestParseTwinModel_Valid(t *testing.T) {
	m, err := twin.ParseTwinModel(validYAML)
	if err != nil {
		t.Fatalf("ParseTwinModel: unexpected error: %v", err)
	}
	if m.APIVersion != "pheromone.io/v1" {
		t.Errorf("apiVersion: got %q, want %q", m.APIVersion, "pheromone.io/v1")
	}
	if m.Kind != "TwinModel" {
		t.Errorf("kind: got %q, want %q", m.Kind, "TwinModel")
	}
	if m.Metadata.Name != "test-model" {
		t.Errorf("metadata.name: got %q, want %q", m.Metadata.Name, "test-model")
	}
	if len(m.Spec.Twins) != 1 {
		t.Errorf("spec.twins: got %d, want 1", len(m.Spec.Twins))
	}
}

func TestParseTwinModel_InvalidYAML(t *testing.T) {
	_, err := twin.ParseTwinModel([]byte("key: [unclosed"))
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

// ---------------------------------------------------------------------------
// Validate — structural constraints (mirrors JSON Schema rules)
// ---------------------------------------------------------------------------

func TestValidate_ValidModel(t *testing.T) {
	m, _ := twin.ParseTwinModel(validYAML)
	if err := m.Validate(); err != nil {
		t.Errorf("Validate: unexpected error: %v", err)
	}
}

func TestValidate_APIVersionPattern(t *testing.T) {
	cases := []struct {
		apiVersion string
		wantErr    bool
	}{
		{"pheromone.io/v1", false},
		{"pheromone.io/v2", false},
		{"pheromone.io/v10", false},
		{"", true},
		{"v1", true},
		{"pheromone.io/v", true},
		{"pheromone.io/va", true},
		{"other.io/v1", true},
	}
	base, _ := twin.ParseTwinModel(validYAML)
	for _, tc := range cases {
		m := *base
		m.APIVersion = tc.apiVersion
		err := m.Validate()
		if tc.wantErr && err == nil {
			t.Errorf("apiVersion %q: expected validation error, got nil", tc.apiVersion)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("apiVersion %q: unexpected error: %v", tc.apiVersion, err)
		}
	}
}

func TestValidate_KindMustBeTwinModel(t *testing.T) {
	m, _ := twin.ParseTwinModel(validYAML)
	m.Kind = "Other"
	if err := m.Validate(); err == nil {
		t.Error("expected error for kind != TwinModel")
	}
}

func TestValidate_MetadataNameRequired(t *testing.T) {
	m, _ := twin.ParseTwinModel(validYAML)
	m.Metadata.Name = ""
	if err := m.Validate(); err == nil {
		t.Error("expected error for empty metadata.name")
	}
}

func TestValidate_MetadataNamePattern(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"valid-name", false},
		{"abc123", false},
		{"a", false},
		{"UPPER", true},
		{"has space", true},
		{"has_underscore", true},
	}
	base, _ := twin.ParseTwinModel(validYAML)
	for _, tc := range cases {
		m := *base
		m.Metadata.Name = tc.name
		err := m.Validate()
		if tc.wantErr && err == nil {
			t.Errorf("name %q: expected validation error, got nil", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("name %q: unexpected error: %v", tc.name, err)
		}
	}
}

func TestValidate_MetadataVersion(t *testing.T) {
	base, _ := twin.ParseTwinModel(validYAML)
	cases := []struct {
		version string
		wantErr bool
	}{
		{"1.0.0", false},
		{"10.2.3", false},
		{"", false}, // optional
		{"v1.0.0", true},
		{"1.0", true},
		{"latest", true},
	}
	for _, tc := range cases {
		m := *base
		m.Metadata.Version = tc.version
		err := m.Validate()
		if tc.wantErr && err == nil {
			t.Errorf("version %q: expected error, got nil", tc.version)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("version %q: unexpected error: %v", tc.version, err)
		}
	}
}

func TestValidate_SelectorsRequired(t *testing.T) {
	m, _ := twin.ParseTwinModel(validYAML)
	m.Spec.Selectors = nil
	if err := m.Validate(); err == nil {
		t.Error("expected error for empty selectors")
	}
}

func TestValidate_TwinsRequired(t *testing.T) {
	m, _ := twin.ParseTwinModel(validYAML)
	m.Spec.Twins = nil
	if err := m.Validate(); err == nil {
		t.Error("expected error for empty twins")
	}
}

func TestValidate_TwinType(t *testing.T) {
	base, _ := twin.ParseTwinModel(validYAML)
	cases := []struct {
		twinType string
		wantErr  bool
	}{
		{"os-level", false},
		{"workload", false},
		{"container", true},
		{"", true},
	}
	for _, tc := range cases {
		m := *base
		twins := make([]twin.TwinDefinition, len(m.Spec.Twins))
		copy(twins, m.Spec.Twins)
		twins[0].Type = tc.twinType
		m.Spec.Twins = twins
		err := m.Validate()
		if tc.wantErr && err == nil {
			t.Errorf("type %q: expected error, got nil", tc.twinType)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("type %q: unexpected error: %v", tc.twinType, err)
		}
	}
}

func TestValidate_TwinDesiredStateRequired(t *testing.T) {
	m, _ := twin.ParseTwinModel(validYAML)
	twins := make([]twin.TwinDefinition, len(m.Spec.Twins))
	copy(twins, m.Spec.Twins)
	twins[0].DesiredState = nil
	m.Spec.Twins = twins
	if err := m.Validate(); err == nil {
		t.Error("expected error for nil desired_state")
	}
}

func TestValidate_RolloutStrategy(t *testing.T) {
	base, _ := twin.ParseTwinModel(validYAML)
	cases := []struct {
		strategy string
		wantErr  bool
	}{
		{"rolling", false},
		{"canary", false},
		{"blue-green", false},
		{"in-place", true},
		{"", true},
	}
	for _, tc := range cases {
		m := *base
		m.Spec.Rollout = &twin.RolloutConfig{Strategy: tc.strategy}
		err := m.Validate()
		if tc.wantErr && err == nil {
			t.Errorf("strategy %q: expected error, got nil", tc.strategy)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("strategy %q: unexpected error: %v", tc.strategy, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Sample file smoke tests — validates the files shipped in twin-models/ are
// well-formed and pass schema validation (ADR-004 acceptance checklist item).
// ---------------------------------------------------------------------------

func TestSampleFile_ProductionWebServers(t *testing.T) {
	path := sampleFile("production-web-servers.yaml")
	m, err := twin.ParseTwinModelFile(path)
	if err != nil {
		t.Fatalf("ParseTwinModelFile: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate production-web-servers.yaml: %v", err)
	}
	// Confirm apiVersion pattern (ADR-004 checklist: "pheromone.io/v1 confirmed")
	if m.APIVersion != "pheromone.io/v1" {
		t.Errorf("expected apiVersion pheromone.io/v1, got %q", m.APIVersion)
	}
	// Confirm both twin levels present (OS + workload)
	typeSet := map[string]bool{}
	for _, td := range m.Spec.Twins {
		typeSet[td.Type] = true
	}
	if !typeSet["os-level"] {
		t.Error("production-web-servers.yaml: expected an os-level twin")
	}
	if !typeSet["workload"] {
		t.Error("production-web-servers.yaml: expected a workload twin")
	}
}

func TestSampleFile_DevTesting(t *testing.T) {
	path := sampleFile("dev-testing.yaml")
	m, err := twin.ParseTwinModelFile(path)
	if err != nil {
		t.Fatalf("ParseTwinModelFile: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate dev-testing.yaml: %v", err)
	}
}
