# Contributing

Thank you for contributing to the ArgoCDConfiguration prototype.

## Before you start

This project is **experimental**. See [README.md](README.md) for scope, limitations, and the detailed field-adding guide.

## Development workflow

```bash
make generate manifests build test
```

- **`make generate`** — regenerate DeepCopy code under `api/`
- **`make manifests`** — regenerate CRD YAML under `config/crd/bases/`
- **`make build`** — build `bin/argocd-config`
- **`make test`** — run unit tests with race detector

Run `make fmt` before submitting changes.

## Adding a golden test case

End-to-end conversion scenarios live in the **case corpus** under `testdata/cases/`. This is the default home for bugs, regressions, and subsystem-specific scenarios.

### Layout

```
testdata/cases/<subsystem>/<case-name>/
  case.yaml
  input/
    argocd-cm.yaml              # direction: from | roundtrip
    argocd-cmd-params-cm.yaml   # optional
    argocd-rbac-cm.yaml         # optional
    configuration.yaml          # direction: to
  expected/
    configuration.yaml          # direction: from
    configmaps/                 # direction: to | roundtrip
      argocd-cm.yaml
      ...
    diagnostics.yaml            # always golden when present
```

### `case.yaml` (required fields)

| Field | Required | Values / notes |
| --- | --- | --- |
| `description` | yes | One-line summary shown in test failures |
| `direction` | yes | `from` (ConfigMaps → CR), `to` (CR → ConfigMaps), or `roundtrip` (CM → CR → CM) |
| `name` | no | CR metadata name (default: singleton name) |
| `namespace` | no | Namespace for ConfigMaps (default: `argocd`) |
| `strict` | no | If true, warnings fail the case (`from` only) |
| `issue` | no | Link or id for bug repro cases |

### Two workflows

**Regression guard** — behavior is already correct; you are locking it in:

```bash
go test ./pkg/mapping -update
# or: go test ./pkg/mapping -run 'Cases/<subsystem>/<case-name>' -update
```

Commit the regenerated `expected/` files. Use `-update` only when you intend to bless new output.

**Bug repro** — red test first, then fix:

1. Add `input/` and hand-write `expected/` (or minimal failing assertion).
2. Do **not** run `-update` until the fix is verified.
3. After the fix, either keep hand-written expectations or regenerate once with `-update`.

### Diagnostics are golden too

When a case should emit warnings or errors, check in `expected/diagnostics.yaml`. The harness compares diagnostic lists exactly (same as configuration and ConfigMap outputs).

### Test taxonomy

| Layer | Where | Purpose |
| --- | --- | --- |
| **Case corpus** | `testdata/cases/` (`TestCases` in `pkg/mapping/cases_test.go`) | Subsystem scenarios, bug repros, round-trip goldens |
| **Unit tables** | `pkg/mapping/mapping_table_test.go`, `helpers_unit_test.go`, … | Focused mapping rules and edge cases |
| **Property** | `pkg/mapping/property_test.go` | CM → CR → CM → CR semantic equality invariant |
| **Fuzz** | `pkg/mapping/fuzz_test.go`, `pkg/convert/convert_test.go` | Randomized inputs seeded from corpus + sample ConfigMaps |

`make cover` and `make cover-gate` report statement coverage for handwritten code under `pkg/` and `cmd/` (excluding generated `api/` and `zz_generated` files). Coverage floors are a CI guardrail, not a substitute for good cases.

## Adding or changing fields

Follow the **Adding a field (contributor guide)** section in [README.md](README.md). In short:

1. Place the field under the correct component in `api/v1alpha1/argocdconfiguration_types.go`
2. Wire `pkg/mapping` (`FromConfigMaps` / `ToConfigMaps`)
3. Extend `testdata/sample-cms` and/or `pkg/mapping` tests
4. Update [docs/coverage-matrix.md](docs/coverage-matrix.md) if you add legacy keys
5. Run `make generate manifests build test`

## Pull requests

- Keep diffs focused; avoid unrelated refactors
- Ensure CI passes (formatting, vet, tests, codegen drift check)
- Do not commit secrets or environment-specific paths

## Design decisions

Architecture decision records live in [docs/adr/](docs/adr/).
