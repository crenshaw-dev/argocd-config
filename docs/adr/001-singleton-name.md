# ADR 001: Namespaced scope and singleton name `argocd-config`

**Status:** Accepted  
**Date:** 2026-07-18

## Context

Argo CD today stores operator configuration in three ConfigMaps (`argocd-cm`, `argocd-cmd-params-cm`, `argocd-rbac-cm`) that are implicitly singletons within the Argo CD install namespace. The prototype `ArgoCDConfiguration` CRD replaces those ConfigMaps with a structured resource.

We must decide CRD scope and whether multiple configuration objects may exist per namespace.

## Decision

1. **Scope:** Namespaced — one `ArgoCDConfiguration` per Argo CD install namespace, matching how ConfigMaps are scoped today.
2. **Singleton name:** The resource **must** be named `argocd-config`. This is enforced in the CRD via CEL:

   ```cel
   self.metadata.name == 'argocd-config'
   ```

3. **No cluster-scoped variant** in this prototype.

## Rationale

- Argo CD is deployed per namespace (or per cluster with a single control-plane namespace); there is no supported multi-tenant “many Argo configs in one namespace” model.
- A fixed name makes discovery predictable for operators, GitOps, and the `argocd-config` CLI — analogous to the well-known ConfigMap names.
- CEL on `metadata.name` catches misconfiguration at admission time without requiring a controller.
- Cluster scope would imply cross-namespace configuration, which Argo CD does not consume and would complicate migration from existing ConfigMaps.

## Consequences

- GitOps repos must use `metadata.name: argocd-config` (the CLI defaults to this).
- Multiple Argo CD instances in one cluster use separate namespaces, each with its own `argocd-config`.
- Future in-tree integration should preserve this constraint unless Argo CD explicitly adds multi-config semantics.

## References

- [PHASE_P_EVAL.md](../../PHASE_P_EVAL.md) — prototype evaluation and settled open questions
- CRD: `config/crd/bases/argo.crenshaw.dev_argocdconfigurations.yaml`
