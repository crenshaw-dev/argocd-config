# ArgoCDConfiguration

> **Prototype / experimental** — A structured CRD and CLI for Argo CD ConfigMaps.
> Not production-ready, not supported by the Argo CD project, and may change or be
> abandoned. See [Scope](scope.md) for what this prototype covers.

Manage Argo CD settings as a single typed resource (`ArgoCDConfiguration`) instead of
editing `argocd-cm`, `argocd-cmd-params-cm`, and `argocd-rbac-cm` by hand. Until Argo CD
accepts this CRD in-tree, treat the CR as your source of truth and generate ConfigMaps
from it.

## Start here

- [Getting started](getting-started.md) — install the CLI, convert ConfigMaps, validate
- [Examples](examples/index.md) — browse every golden conversion case (input → CR / ConfigMaps)
- [API reference](api-reference.md) — field-level docs generated from Go types
- [Coverage matrix](coverage-matrix.md) — legacy ConfigMap keys ↔ CR fields

## Full-field sample

[EXAMPLE.yaml](https://github.com/crenshaw-dev/argocd-config/blob/main/EXAMPLE.yaml) on
GitHub exercises every CRD field. Its round-trip is documented as
[roundtrip / example-full](examples/roundtrip/example-full.md).
