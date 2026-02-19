# ADR 004: Twin Model Schema Format - YAML/JSON Definition Language

## Status
Proposed

## Context

Infrastructure operators must define twin models (desired state templates) that describe which features and configurations apply to groups of agents (1:Many pattern). The twin model schema must be:

1. **Human-readable and editable** (operator-friendly, not compile-step required)
2. **Versionable and reviewable** via Git (part of IaC philosophy)
3. **Portable** across teams (standardized schema, not internal DSL)
4. **Structured** to support validation, schema evolution, and tooling

**Options Considered**:
- YAML (human readable, git-friendly, standard in Kubernetes)
- JSON (strict validation, tooling support)
- Hashicorp HCL (more expressive, less universal)
- Protobuf (binary, not human-readable)
- CUE (powerful validation, but niche learning curve)

**Constraints**:
- Operators (non-developers) must understand and edit models (FR-013, SC-010)
- Schema validation required (mismatched configurations must be caught early)
- Multi-environment support (dev, staging, prod twins may differ)
- Git history/audit trail preserved (immutability of past decisions)

## Decision

**Choose: YAML as Primary Format, with JSON Strict Schema Validation**

### Rationale

1. **Operator Accessibility**: YAML is familiar to DevOps/SRE teams (Kubernetes, Ansible, Docker Compose)
2. **Git-Native**: YAML diffs are readable; comments supported for documentation
3. **Minimal Tooling**: Standard YAML parsers available in all languages (Go, Python, Rust)
4. **Validation**: JSON Schema v7 can validate YAML (strict schema enforcement)
5. **Ecosystem Maturity**: Helm, Kustomize, other tools standardize on YAML for config management

### Twin Model YAML Structure

```yaml
# File: twin-models/production-web-servers.yaml

apiVersion: pheromone.io/v1
kind: TwinModel
metadata:
  name: production-web-servers
  namespace: prod
  description: "Production web server cluster (nginx, OS-level config)"
  version: "1.0.0"
  labels:
    env: production
    tier: web
spec:
  selectors:
    # Match agents by labels/metadata
    - label: "env=production"
    - label: "tier=web"
    - hostname_pattern: "web-*.prod.internal"

  twins:
    # OS-Level Twin
    - name: os-config
      type: os-level
      metadata:
        description: "Os-level configuration for production servers"
      desired_state:
        kernel_version: "5.15.0"  # minimum version
        security_patch_level: "2026-02-10"
        installed_packages:
          nginx: "1.24.0"
          openssh-server: "8.2p1"
          chrony: "4.3"

    # Workload-Level Twin
    - name: nginx-service
      type: workload
      metadata:
        description: "Nginx web service configuration"
      dependencies:
        - os-config  # depends on OS twin
      desired_state:
        service_name: nginx
        port: 80
        config: |
          # nginx.conf snippet
          worker_processes auto;
          worker_connections 2048;
          keepalive_timeout 65;
        restart_policy: "always"
        health_check:
          type: http
          endpoint: "http://localhost/health"
          interval_seconds: 30
          timeout_seconds: 5

  # Rollout strategy
  rollout:
    strategy: rolling  # or "canary", "blue-green"
    max_unavailable: "10%"
    wait_before_apply: "5m"

  # Rollback configuration
  rollback:
    auto_rollback_on_failed_health_check: true
    history_retention: 10  # keep 10 versions for rollback
```

### Schema Validation (JSON Schema)

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Pheromone Twin Model",
  "type": "object",
  "required": ["apiVersion", "kind", "metadata", "spec"],
  "properties": {
    "apiVersion": {
      "type": "string",
      "pattern": "^pheromone\\.io/v[0-9]+$"
    },
    "kind": {
      "type": "string",
      "enum": ["TwinModel"]
    },
    "metadata": {
      "type": "object",
      "required": ["name"],
      "properties": {
        "name": { "type": "string", "pattern": "^[a-z0-9-]+$" },
        "namespace": { "type": "string" },
        "version": { "type": "string", "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+$" }
      }
    },
    "spec": {
      "type": "object",
      "required": ["selectors", "twins"],
      "properties": {
        "selectors": { "type": "array" },
        "twins": {
          "type": "array",
          "minItems": 1,
          "items": {
            "required": ["name", "type", "desired_state"],
            "properties": {
              "name": { "type": "string" },
              "type": { "enum": ["os-level", "workload"] },
              "desired_state": { "type": "object" }
            }
          }
        }
      }
    }
  }
}
```

### Version Evolution Strategy

- **Field Additions**: Add optional fields with defaults (backward compatible)
- **Field Removal**: Deprecate first with warning log; remove after 1 release cycle
- **Schema Changes**: Increment `apiVersion` only on breaking changes
- **Migration Path**: Server supports current + previous apiVersion for 1 release cycle

## Consequences

### Positive
- Operators can edit models in any text editor (no special tooling required)
- Git history provides audit trail of config changes (FRs 004, 027)
- JSON Schema enables IDE validation plugins (catch errors before submit)
- YAML reduces boilerplate vs. JSON (comments, multiline strings)
- Familiar to DevOps community (similar to Kubernetes manifests)

### Negative
- YAML whitespace sensitivity requires careful editing (but mitigated by IDE validation)
- Schema validation tooling needed (requires JSON Schema implementation)
- No runtime type checking (JSON Schema is static validation only)
- YAML ↔ Protocol Buffers conversion required for gRPC (ADR-003)

### Implementation Requirements
- **Go Marshaling**: `gopkg.in/yaml.v3` for YAML parsing + struct tags
- **Schema Validation**: `github.com/xeipuuv/gojsonschema` or `ory/jsonschema`
- **Tooling**: `kubectl`-style CLI for `pheromone apply` / `pheromone get TwinModels`
- **Testing**: Validate sample YAML files against schema (smoke test FR-023)

## File Organization

```
twin-models/
├── production-web-servers.yaml
├── production-databases.yaml
├── staging-all-services.yaml
├── dev-testing.yaml
└── README.md  # documentation on model structure
```

Each model file is independently versionable in Git; teams can review changes via PR.

## Follow-Up ADRs

- **ADR-006**: Agent lifecycle interface (how agents receive and apply twin models from server)

## References

- YAML v1.2 Spec: https://yaml.org/spec/1.2/spec.html
- JSON Schema Draft 7: https://json-schema.org/draft-07/
- Kubernetes API Conventions: https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/
- Spec-001, FR-013, SC-001, SC-010
- Constitution Principle I (Layered architecture), Principle II (ADR documentation)

---

**Decision Date**: 2026-02-18
**Status Update**: Proposed (pending sample model review for usability)
