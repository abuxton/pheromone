package twin

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// apiVersionPattern is the required pattern for the apiVersion field (ADR-004).
var apiVersionPattern = regexp.MustCompile(`^pheromone\.io/v[0-9]+$`)

// namePattern enforces lowercase-alphanumeric-with-hyphens names.
var namePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// semverPattern enforces semantic versioning for metadata.version.
var semverPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// TwinModelFile is the top-level structure for a twin model YAML file (ADR-004).
type TwinModelFile struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   TwinModelMetadata `yaml:"metadata"`
	Spec       TwinModelSpec     `yaml:"spec"`
}

// TwinModelMetadata holds identifying information for a twin model.
type TwinModelMetadata struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Version     string            `yaml:"version,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

// TwinModelSpec describes the selectors, twins, rollout, and rollback configuration.
type TwinModelSpec struct {
	Selectors []TwinSelector   `yaml:"selectors"`
	Twins     []TwinDefinition `yaml:"twins"`
	Rollout   *RolloutConfig   `yaml:"rollout,omitempty"`
	Rollback  *RollbackConfig  `yaml:"rollback,omitempty"`
}

// TwinSelector specifies how agents are matched for this model.
type TwinSelector struct {
	Label           string `yaml:"label,omitempty"`
	HostnamePattern string `yaml:"hostname_pattern,omitempty"`
}

// TwinDefinition describes a single twin (OS-level or workload) within a model.
type TwinDefinition struct {
	Name         string                 `yaml:"name"`
	Type         string                 `yaml:"type"` // "os-level" or "workload"
	Metadata     *TwinDefinitionMeta    `yaml:"metadata,omitempty"`
	Dependencies []string               `yaml:"dependencies,omitempty"`
	DesiredState map[string]interface{} `yaml:"desired_state"`
}

// TwinDefinitionMeta holds optional metadata for a single twin definition.
type TwinDefinitionMeta struct {
	Description string `yaml:"description,omitempty"`
}

// RolloutConfig controls how configuration changes are applied to agents.
type RolloutConfig struct {
	Strategy        string `yaml:"strategy"` // "rolling", "canary", "blue-green"
	MaxUnavailable  string `yaml:"max_unavailable,omitempty"`
	WaitBeforeApply string `yaml:"wait_before_apply,omitempty"`
}

// RollbackConfig controls automatic rollback behaviour.
type RollbackConfig struct {
	AutoRollbackOnFailedHealthCheck bool `yaml:"auto_rollback_on_failed_health_check,omitempty"`
	HistoryRetention                int  `yaml:"history_retention,omitempty"`
}

// ParseTwinModelFile reads and parses a twin model YAML file from disk.
func ParseTwinModelFile(path string) (*TwinModelFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading twin model file %q: %w", path, err)
	}
	return ParseTwinModel(data)
}

// ParseTwinModel parses a twin model from raw YAML bytes.
func ParseTwinModel(data []byte) (*TwinModelFile, error) {
	var m TwinModelFile
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing twin model YAML: %w", err)
	}
	return &m, nil
}

// Validate checks that the model satisfies the structural constraints defined
// in the JSON Schema (twin-models/schemas/twin-model.schema.json, ADR-004).
// It returns a non-nil error describing the first violation found.
func (m *TwinModelFile) Validate() error {
	if !apiVersionPattern.MatchString(m.APIVersion) {
		return fmt.Errorf("apiVersion %q does not match required pattern %q", m.APIVersion, apiVersionPattern)
	}
	if m.Kind != "TwinModel" {
		return fmt.Errorf("kind must be \"TwinModel\", got %q", m.Kind)
	}
	if m.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if !namePattern.MatchString(m.Metadata.Name) {
		return fmt.Errorf("metadata.name %q must match pattern %q (lowercase alphanumeric with hyphens)", m.Metadata.Name, namePattern)
	}
	if m.Metadata.Version != "" && !semverPattern.MatchString(m.Metadata.Version) {
		return fmt.Errorf("metadata.version %q must be a semantic version (X.Y.Z)", m.Metadata.Version)
	}
	if len(m.Spec.Selectors) == 0 {
		return fmt.Errorf("spec.selectors must contain at least one entry")
	}
	if len(m.Spec.Twins) == 0 {
		return fmt.Errorf("spec.twins must contain at least one twin definition")
	}
	for i, twin := range m.Spec.Twins {
		if twin.Name == "" {
			return fmt.Errorf("spec.twins[%d].name is required", i)
		}
		if twin.Type != "os-level" && twin.Type != "workload" {
			return fmt.Errorf("spec.twins[%d].type must be \"os-level\" or \"workload\", got %q", i, twin.Type)
		}
		if twin.DesiredState == nil {
			return fmt.Errorf("spec.twins[%d].desired_state is required", i)
		}
	}
	if m.Spec.Rollout != nil {
		switch m.Spec.Rollout.Strategy {
		case "rolling", "canary", "blue-green":
		default:
			return fmt.Errorf("spec.rollout.strategy must be one of rolling/canary/blue-green, got %q", m.Spec.Rollout.Strategy)
		}
	}
	return nil
}
