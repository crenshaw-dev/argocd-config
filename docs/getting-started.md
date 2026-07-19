# Getting started

Requires a recent Go toolchain (see `go` version in the repo [`go.mod`](https://github.com/crenshaw-dev/argocd-config/blob/main/go.mod)).

## Install the CLI

```bash
go install github.com/crenshaw-dev/argocd-config/cmd/argocd-config@latest
argocd-config version
```

## Convert existing ConfigMaps

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

### Validate

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

Point your install tooling (Kustomize, Helm post-renderer, plain `kubectl apply -f`) at the generated ConfigMaps after each change to the CR.

## Optional: install the CRD

```bash
kubectl apply -f https://raw.githubusercontent.com/crenshaw-dev/argocd-config/main/config/crd/bases/argoproj.io_argocdconfigurations.yaml
```

Applying the CRD does **not** make Argo CD read it yet. You still generate ConfigMaps with the CLI.

## CLI reference

| Command | Purpose |
| --- | --- |
| `from-configmaps` | ConfigMaps → `ArgoCDConfiguration` |
| `to-configmaps` | `ArgoCDConfiguration` → ConfigMaps |
| `validate` | OpenAPI + CEL checks against the embedded CRD |
| `convert --to-version` | apiVersion conversion (hub/spoke demo) |
| `version` | Build metadata |

Useful flags: `--strict`, `--report text|json`, `--no-validate`, `--permissive` (skip round-trip on `from-configmaps`). Preserve labels/annotations on `to-configmaps` with `--source-cm` / `--source-cmd-params` / `--source-rbac`.

## What to expect

- Conversion covers the usual operator-manual keys for the three ConfigMaps; see the [coverage matrix](coverage-matrix.md).
- Round-trips are best-effort. YAML formatting, composite blobs (`oidc.config`, `dex.config`, …), and a few dual-prefix keys may differ; the CLI warns when something looks unsafe.
- Secrets (`argocd-secret`) and trust-store ConfigMaps stay out of scope for now.

Browse concrete conversion scenarios under [Examples](examples/index.md).
