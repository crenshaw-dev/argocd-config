# Prototype scope

This page summarizes the Phase P evaluation for the `ArgoCDConfiguration` prototype.
The full write-up lives in
[`PHASE_P_EVAL.md`](https://github.com/crenshaw-dev/argocd-config/blob/main/PHASE_P_EVAL.md)
at the repository root.

## Delivered

- Spec-only CRD `ArgoCDConfiguration` (`argoproj.io/v1alpha1`)
- CLI `argocd-config`: `from-configmaps`, `to-configmaps`, `validate`, `convert`
- Round-trip golden tests under `testdata/cases/` (browseable as [Examples](examples/index.md))
- Operator-manual key coverage for `argocd-cm`, `argocd-cmd-params-cm`, and `argocd-rbac-cm`

## Deferred

- Trust-store ConfigMaps (SSH known hosts, TLS certs, GPG keys)
- `argocd-secret` values (except `SecretKeySelector` where the CR owns the shape)
- notifications-cm, repository / cluster Secrets

## Design decisions

- [ADR 001: Singleton name](adr/001-singleton-name.md) — namespaced CR, fixed name `argocd-config`
