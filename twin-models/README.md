# Twin Models

This directory contains Pheromone twin model YAML files.  Each file is independently
versionable in Git; teams review configuration changes via pull request.

## Format

All twin models must conform to `schemas/twin-model.schema.json` (JSON Schema Draft-07).

The `apiVersion` field follows the pattern `pheromone.io/v<N>`.  Schema evolution
rules (ADR-004):

| Change Type | Action |
|---|---|
| Add optional field | Add with a default — backward compatible |
| Remove field | Deprecate with warning log first; remove after 1 release cycle |
| Breaking change | Increment `apiVersion` (e.g. `pheromone.io/v2`) |
| Migration | Server supports current + previous `apiVersion` for 1 release cycle |

## Files

| File | Environment | Purpose |
|---|---|---|
| `production-web-servers.yaml` | prod | Nginx cluster OS + workload twin |
| `dev-testing.yaml` | dev | Developer testing environment |

## Validate a model

```bash
# Go test (runs schema smoke test automatically):
go test ./internal/twin/... -run TestTwinModel

# Manual check with yq:
yq eval '.' production-web-servers.yaml
```

## References

- ADR-004: `docs/adr/adr-004-twin-model-schema.md`
- JSON Schema: `schemas/twin-model.schema.json`
