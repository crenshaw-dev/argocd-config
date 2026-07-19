# Phase P evaluation — STOP gate

Completed: 2026-07-18 (updated after full three-CM coverage + convention audit)  
Prototype: `/Users/mcrenshaw/src/argocd-configuration` (`github.com/crenshaw-dev/argocd-config`)

## Delivered

- Spec-only CRD `ArgoCDConfiguration` (`argoproj.io/v1alpha1`)
- Generated deepcopy + CRD: `config/crd/bases/argoproj.io_argocdconfigurations.yaml`
- CLI `argocd-config`: `from-configmaps`, `to-configmaps`, `convert --to-version`
- Hub conversion scaffolding (`v1alpha1` is hub)
- Round-trip tests covering OIDC `$string` / SecretKeySelector, Dex connector envelope, RBAC raw CSV, resource customizations, cmd-params (incl. `;` separator)
- **Operator-manual key coverage** for `argocd-cm`, `argocd-cmd-params-cm`, and `argocd-rbac-cm` is complete
- Contributor conventions documented (migration roles, `*Enabled`, `*Keys`/`*KeyGlobs`, Quantity `*Size`, TLS polarity, `listType=map`/`atomic`)

```bash
make generate manifests build test
./bin/argocd-config from-configmaps \
  --cm testdata/sample-cms/argocd-cm.yaml \
  --cmd-params testdata/sample-cms/argocd-cmd-params-cm.yaml \
  --rbac testdata/sample-cms/argocd-rbac-cm.yaml \
  -o /tmp/acc.yaml
./bin/argocd-config to-configmaps -f /tmp/acc.yaml -o /tmp/cms
```

## Out of scope (deferred; may return later)

- Trust-store ConfigMaps: SSH known hosts, TLS certs, GPG keys
- `argocd-secret` values (except SecretKeySelector where the CR owns the shape)
- notifications-cm, repository / cluster Secrets

## Learnings that reshaped Phase 0+

1. **Root-nilable composites** for wholesale legacy blobs (`oidc.config`, `dex.config`, …).
2. **`runtime.RawExtension` needs JSON in `.Raw`** when set from Go (ConfigMap mapper).
3. **Component-oriented nesting** (`spec.server` / `controller` / `repoServer` / …).
4. **Almost everything still needs a CRD-native type** → Phase 1 converter suite remains mandatory.
5. **`pkg/mapping` is the Phase 0 precursor** — structured key→field catalog.
6. **Split-key families** validate `KeyFunc` design.
7. **Per-field separators** (e.g. `;` for plugin tar exclusions) belong in registry metadata.
8. **Dex envelope and RBAC raw CSV** held up well.

## Next step: Phase 0 in argo-cd (ACTIVE)

Do **not** expand Phase P with more field batches. Implement the config bus in-tree:

- Package: `util/configbus` (registry + provider; empty CRD slot)
- Inventory: `hack/config-inventory`
- Drift guard: `util/configbus/completeness_test.go`
- First slice: `timeout.reconciliation` + `resource.customizations`
- Docs: `docs/operator-manual/config-registry.md`

See the Config CRD Implementation plan (Phase 0 section).

## Open questions (do not block Phase 0)

- In-tree API group / kind name  
- Getter fallibility  
- `resource.customizations` wildcard as list item vs dedicated field  

## Settled (prototype)

- **CRD scope:** namespaced — see [docs/adr/001-singleton-name.md](docs/adr/001-singleton-name.md)  
- **Singleton name:** `argocd-config` only (CEL-enforced)  
