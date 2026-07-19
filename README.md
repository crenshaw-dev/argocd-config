# ArgoCDConfiguration

[![codecov](https://codecov.io/gh/crenshaw-dev/argocd-config/branch/main/graph/badge.svg)](https://codecov.io/gh/crenshaw-dev/argocd-config)

> **Prototype / experimental** — A structured CRD and CLI for Argo CD ConfigMaps. Not production-ready, not supported by the Argo CD project, and may change or be abandoned. See [PHASE_P_EVAL.md](PHASE_P_EVAL.md).

Manage Argo CD settings as a single typed resource (`ArgoCDConfiguration`) instead of editing `argocd-cm`, `argocd-cmd-params-cm`, and `argocd-rbac-cm` by hand. Until Argo CD accepts this CRD in-tree, treat the CR as your source of truth and generate ConfigMaps from it.

For a manifest that exercises every CRD field (useful as a schema reference), see [EXAMPLE.yaml](EXAMPLE.yaml). Completeness + CEL checks live in `pkg/validate/example_test.go`; the CR→CM→CR→CM round-trip with golden ConfigMaps is `testdata/cases/roundtrip/example-full`. Field-level API reference (generated from Go godoc): [docs/api-reference.md](docs/api-reference.md) (`make api-docs`).

## Documentation

Browse the MkDocs site (API reference, coverage matrix, and every golden case as an example):

```bash
python3 -m venv .venv-docs
source .venv-docs/bin/activate
pip install -r requirements-docs.txt
make docs-serve
```

Regenerate generated Markdown after API or case changes: `make docs`. The site is also configured for [Read the Docs](.readthedocs.yaml) (connect the GitHub repo in the RTD UI).

## Install the CLI

Requires a recent Go toolchain (see `go` version in [go.mod](go.mod)):

```bash
go install github.com/crenshaw-dev/argocd-config/cmd/argocd-config@latest
```

Confirm:

```bash
argocd-config version
```

## Convert your existing ConfigMaps

### From files on disk

```bash
argocd-config from-configmaps \
  --cm /path/to/argocd-cm.yaml \
  --cmd-params /path/to/argocd-cmd-params-cm.yaml \
  --rbac /path/to/argocd-rbac-cm.yaml \
  -o argocd-configuration.yaml
```

Any subset of the three flags is fine if you only have some of the ConfigMaps.

### From a live cluster

```bash
argocd-config from-configmaps \
  --from-cluster \
  --namespace argocd \
  -o argocd-configuration.yaml
```

Optional: `--kubeconfig` / `--context` if you are not using your current kubeconfig default.

### Check the result

```bash
argocd-config validate -f argocd-configuration.yaml
```

`from-configmaps` also validates by default and round-trips back to ConfigMaps to warn about unexpected diffs. Use `--strict` to fail on warnings, or `--permissive` to skip the round-trip check while you inspect the CR by hand.

## Use the CR as source of truth

Keep `argocd-configuration.yaml` (or an equivalent Git-managed manifest) as the document you edit. Whenever you need ConfigMaps for Argo CD today, generate them:

```bash
argocd-config to-configmaps \
  -f argocd-configuration.yaml \
  -o ./generated-cms
```

That writes `argocd-cm.yaml`, `argocd-cmd-params-cm.yaml`, and `argocd-rbac-cm.yaml` into the output directory (or multi-doc YAML to stdout with `-o -`).

### Local generation

Point your install tooling (Kustomize, Helm post-renderer, plain `kubectl apply -f`) at the generated ConfigMaps after each change to the CR.

### CI checks

In CI, regenerate ConfigMaps from the CR and fail if the checked-in ConfigMaps (or live cluster) drift. Example sketch:

```bash
argocd-config to-configmaps -f argocd-configuration.yaml -o /tmp/cms
diff -ru expected-cms/ /tmp/cms/
# or: kubectl diff -f /tmp/cms
```

You can also run `argocd-config validate -f argocd-configuration.yaml --strict` on every PR.

This bridge exists **until Argo CD can consume `ArgoCDConfiguration` directly**. There is no controller in this prototype that applies the CR for you — only conversion and validation.

## Optional: install the CRD

If you want the type available in a cluster (for storage, GitOps of the CR itself, or editor/schema tooling):

```bash
kubectl apply -f https://raw.githubusercontent.com/crenshaw-dev/argocd-config/main/config/crd/bases/argoproj.io_argocdconfigurations.yaml
```

Applying the CRD does **not** make Argo CD read it yet. You still generate ConfigMaps with the CLI.

## What to expect

- Conversion covers the usual operator-manual keys for the three ConfigMaps; see [docs/coverage-matrix.md](docs/coverage-matrix.md).
- Round-trips are best-effort. YAML formatting, composite blobs (`oidc.config`, `dex.config`, …), and a few dual-prefix keys may differ; the CLI warns when something looks unsafe.
- Secrets (`argocd-secret`) and trust-store ConfigMaps stay out of scope for now.

## CLI reference (short)

| Command | Purpose |
| --- | --- |
| `from-configmaps` | ConfigMaps → `ArgoCDConfiguration` |
| `to-configmaps` | `ArgoCDConfiguration` → ConfigMaps |
| `validate` | OpenAPI + CEL checks against the embedded CRD |
| `convert --to-version` | apiVersion conversion (hub/spoke demo) |
| `version` | Build metadata |

Useful flags: `--strict`, `--report text|json`, `--no-validate`, `--permissive` (skip round-trip on `from-configmaps`). Preserve labels/annotations on `to-configmaps` with `--source-cm` / `--source-cmd-params` / `--source-rbac`.

## Contributing

Developer workflow, golden tests, and the field-adding guide live in [CONTRIBUTING.md](CONTRIBUTING.md). Design notes: [PHASE_P_EVAL.md](PHASE_P_EVAL.md), [docs/adr/](docs/adr/).

## License

Apache-2.0
