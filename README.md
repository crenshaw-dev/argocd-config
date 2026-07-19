# ArgoCDConfiguration

[![codecov](https://codecov.io/gh/crenshaw-dev/argocd-config/branch/main/graph/badge.svg)](https://codecov.io/gh/crenshaw-dev/argocd-config)

> **Prototype / experimental** — This repo explores a structured CRD replacement for Argo CD ConfigMaps. It is not production-ready, not supported by the Argo CD project, and may change or be abandoned. See [PHASE_P_EVAL.md](PHASE_P_EVAL.md).

Standalone prototype CRD for Argo CD configuration (`argo.crenshaw.dev/v1alpha1`).

This repo prototypes a structured replacement for `argocd-cm`, `argocd-cmd-params-cm`, and `argocd-rbac-cm` **before** anything lands in-tree in [argoproj/argo-cd](https://github.com/argoproj/argo-cd).

## What it is

- **CRD**: `ArgoCDConfiguration` — spec-only (no status, no controller)
- **CLI** (`argocd-config`):
  - `from-configmaps` — ConfigMaps → CRD YAML
  - `to-configmaps` — CRD YAML → ConfigMaps
  - `convert --to-version` — apiVersion conversion (hub/spoke ready)
  - `validate` — offline structural checks (singleton name, URL rules)
  - `version` — build metadata

```bash
make generate manifests build test
./bin/argocd-config from-configmaps \
  --cm testdata/sample-cms/argocd-cm.yaml \
  --cmd-params testdata/sample-cms/argocd-cmd-params-cm.yaml \
  --rbac testdata/sample-cms/argocd-rbac-cm.yaml \
  -o /tmp/argocdconfiguration.yaml
./bin/argocd-config to-configmaps -f /tmp/argocdconfiguration.yaml -o /tmp/cms
```

Or pull the live ConfigMaps from a cluster:

```bash
./bin/argocd-config from-configmaps --from-cluster --namespace argocd -o /tmp/argocdconfiguration.yaml
# optional: --kubeconfig /path/to/kubeconfig --context my-context
```

## Install (CRD only)

Apply the generated CRD to a cluster:

```bash
kubectl apply -f config/crd/bases/argo.crenshaw.dev_argocdconfigurations.yaml
```

There is no controller in this prototype — the CRD is a structured data store and CLI conversion target.

## End-to-end example

Convert sample ConfigMaps to a CR, validate, round-trip back, and exercise apiVersion conversion:

```bash
# ConfigMaps → CRD YAML
./bin/argocd-config from-configmaps \
  --cm testdata/sample-cms/argocd-cm.yaml \
  --cmd-params testdata/sample-cms/argocd-cmd-params-cm.yaml \
  --rbac testdata/sample-cms/argocd-rbac-cm.yaml \
  -o /tmp/acc.yaml

# Validate (OpenAPI + CEL from embedded CRD)
./bin/argocd-config validate -f /tmp/acc.yaml

# CRD YAML → ConfigMaps
./bin/argocd-config to-configmaps -f /tmp/acc.yaml -o /tmp/cms-out

# Hub → spoke conversion demo (v1beta1 carries spec.url only)
./bin/argocd-config convert -f /tmp/acc.yaml --to-version argo.crenshaw.dev/v1beta1 -o /tmp/acc-v1beta1.yaml
```

### CLI flags

Global flags on conversion commands:

| Flag | Purpose |
| --- | --- |
| `--strict` | Treat warnings as errors (non-zero exit) |
| `--report text\|json` | Diagnostics output format |
| `--permissive` | (`from-configmaps`) Skip round-trip self-check |
| `--no-validate` | Skip post-conversion validation |

`from-configmaps` takes `--cm` / `--cmd-params` / `--rbac` file paths (any subset), `-o` for output, or `--from-cluster` (with optional `--kubeconfig` / `--context`) to load the standard-named ConfigMaps from a live cluster. After conversion it always round-trips CR→CM and warns on remaining diffs after known-safe normalizations (see `pkg/mapping/roundtrip_normalize.go`); use `--permissive` to skip. `to-configmaps` can preserve labels/annotations via `--source-cm` / `--source-cmd-params` / `--source-rbac`.

## Coverage and limitations

- **Key inventory:** [docs/coverage-matrix.md](docs/coverage-matrix.md) — mapped legacy keys vs CR fields (grep-based, not a full upstream scrape).
- **Golden case corpus:** [testdata/cases/](testdata/cases/) — end-to-end conversion scenarios exercised by `TestCases` in `pkg/mapping`. Regenerate expectations with `go test ./pkg/mapping -update` (regression guards only; see [CONTRIBUTING.md](CONTRIBUTING.md)).
- **Statement coverage:** `make cover` / `make cover-gate` — handwritten `pkg/` and `cmd/` coverage with CI floors.
- **Contributing:** [CONTRIBUTING.md](CONTRIBUTING.md) — workflow and pointer to the field guide below.

### Limitations / lossy transforms

Round-trip conversion is **best-effort**. Known lossy cases include:

- **Composite blobs** (`oidc.config`, `dex.config`, `resource.compareoptions`) — CR replaces the whole document; partial CM/CR merge is not supported.
- **`$string` / secret interpolation** — preserved where possible in opaque structures; not all legacy values round-trip byte-for-byte.
- **YAML formatting** — re-emitted ConfigMap values may differ in whitespace or key order.
- **Split keys** — `controller.repo.server.*` vs `server.repo.server.*` collapse to one CR field (warning emitted when both differ).
- **v1beta1 spoke** — converts only `spec.server.urls[0]` ↔ `spec.url`; all other fields are dropped in spoke form (conversion demo only).
- **Offline validate** — OpenAPI + CEL + list-type checks against the embedded CRD schema (same `apiextensions-apiserver` libraries the apiserver uses).

See [PHASE_P_EVAL.md](PHASE_P_EVAL.md) for deferred scope (trust stores, secrets, notifications-cm content).

## Adding a field (contributor guide)

Use this when mapping a legacy ConfigMap / cmd-params key into the CRD. The same rules apply after the initial population pass when someone adds a newly introduced Argo CD setting.

### 1. Find the right place in the tree

Spec is **component-oriented**, not ConfigMap-oriented. Ask: *which process or concern owns this setting?*

| Put it under… | When the setting is mainly about… | Examples |
| --- | --- | --- |
| `spec.server` | API server, UI, auth, webhooks, accounts, extensions | `url`, OIDC/Dex, RBAC, banner, exec |
| `spec.controller` | Application controller ops **or** application/resource product config historically in `argocd-cm` | processors, self-heal, `resource.*`, instance label, sync impersonation |
| `spec.repoServer` | Repo-server address/client, Kustomize/Helm | `repo.server`, plugin tar exclusions |
| `spec.commitServer` | Commit-server + hydrator commit identity | `commit.server`, `commit.author.*` |
| `spec.applicationSet` | ApplicationSet controller | namespaces, policy, SCM allowlist |
| `spec.cluster` | Cluster registration policy | `cluster.inClusterEnabled` |
| `spec.redis` / `spec.otlp` / `spec.logging` | Shared cross-component plumbing | Redis address, OTLP collector, log timestamp |
| `spec.applicationNamespaceGlobs` | Apps-in-any-namespace (shared) | `application.namespaces` |

**Nest further** when several keys form one concept (`commit.author.name` + `email` → `commit.author`, OIDC Azure block → `oidc.azure`).

**Do not** put `argocd-secret` material on the CRD (webhook secrets, `server.secretkey`, TLS keys). Whole secrets Argo owns end-to-end use `SecretKeySelector` on the CR (see below).

Pick the field’s **migration role** (this drives pointer shape and the `Migration:` comment):

| Role | Use when… | Go shape | Precedence |
| --- | --- | --- | --- |
| **Component group** | Top-level bucket (`server`, `controller`, …) | `*FooConfig` on Spec | Organizational only — no legacy key |
| **Organizational subgroup** | Nesting for readability, but children migrate **independently** | `*T` | Parent non-nil does **not** override anything; each child has its own `Migration:` line |
| **Composite override** | One legacy YAML/document (`oidc.config`, `dex.config`, `resource.compareoptions`) | `*T` | Non-nil replaces the **whole** blob (no per-field merge) |
| **Settings group** | A **small, cohesive** key family you expect people to adopt together | `*T` | Non-nil → whole family from CR (no merge with legacy siblings) |
| **Leaf** | One legacy key | scalar / `*bool` / `*Duration` / … | If set → that key wins |
| **Collection** | List/map that replaces a CM collection | `[]T` / `map[…]` | If present → **replace** entire collection |
| **Composite child** | Field inside a composite override | ordinary value | Not merged with CM on its own |

#### Settings group vs organizational subgroup

Default to **organizational subgroup** whenever someone might reasonably set one child without the others. Wholesale **settings groups** are for tight units only (roughly a handful of closely related keys).

| Prefer organizational subgroup when… | Prefer settings group when… |
| --- | --- |
| Many independent keys (`ui.*`, `resource.*`) | 2–4 keys that are one feature (`statusbadge.*`, `ga.*`, `exec.*`) |
| Mix of unrelated concerns in one struct (`users.*` + `passwordPattern`) | Name+email, enabled+shells, etc. |
| Gradual adoption of a large area is expected | Adopting half the group would leave a confusing half-legacy state |

**Bools inside organizational subgroups** that need unset vs false must be `*bool` (value `bool` cannot fall through once the parent exists in the object graph). Same for other zero-able scalars if “explicit empty” matters.

**Current inventory (prototype):**

| Field | Role | Why |
| --- | --- | --- |
| `server.dex` / `server.oidc` / `controller.diff.compareOptions` | Composite override | Single legacy document |
| `server.ui`, `server.rbac`, `server.deepLinks`, `server.help`, `server.users`, `controller.resource`, `controller.sync`, `controller.sourceHydrator`, `repoServer.kustomize`, `repoServer.client`, `controller.metrics` | Organizational subgroup | Too broad / independent keys — migrate per child |
| `server.googleAnalytics`, `server.exec`, `server.statusBadge`, `server.dexServer` (connection), `server.tls` / `repoServer.tls`, `repoServer.helm`, `repoServer.jsonnet`, `commitServer.commit` (+ author), `controller.selfHeal`, `controller.clusterCache` | Settings group | Small cohesive feature |
| `server.webhook`, `server.cache`, `server.k8sClient`, `controller.k8sClient`, `repoServer.oci` | Organizational subgroup | Mixed sources / independent keys |

When you add a new nested struct, pick deliberately — don’t default to settings-group wholesale just because the parent is a pointer.

### 2. Name the field

1. Use clear Go / JSON names (`issuerURL`, not `issuer`).
2. Apply a **suffix** when the value’s language matters:

| Suffix | Meaning | Do / don’t |
| --- | --- | --- |
| `*URL` | Absolute URL | CEL `isURL` (scalars **and** slices). Not for `host:port`. Exception: documented path-or-URL fields (`cssURL`, `binaryURLs`) skip absolute-URL validation. |
| `*Glob` | May contain `*` / `?` wildcards | Do **not** apply qualified-name regex validation |
| `*Regex` | Go `regexp` / RE2 pattern | Not `*Pattern` — keep Regex vs Glob obvious |
| `*Expr` | Expression language (e.g. deep-link `if`) | App validates language semantics |
| `*Template` | Go `text/template` (or similar) | Do **not** CEL-validate as a plain URL |
| `*Lua` | Lua script source | The field value **is** Lua (e.g. `healthLua`, `discoveryLua`, `actionLua`). Do **not** use `*Lua` for a YAML/document wrapper that merely embeds Lua — structure that into typed children instead |
| `*Size` | Byte / payload size limit | Type is `resource.Quantity` (e.g. `50M`). Do **not** use unit-suffixed ints (`maxPayloadSizeMB`). Not for entry counts (those stay `*int32`, e.g. `globCacheSize`) |
| `*Keys` | List of exact label/annotation (or similar) keys | Prefer singular noun + `Keys` (`annotationKeys`, not `annotations` / `annotationsKeys`). Examples: `customLabelKeys`, `sensitiveMaskAnnotationKeys` |
| `*KeyGlobs` | List of label/annotation key patterns that may contain wildcards | Same idea as `*Keys`, but each entry is a glob — combine with `*Glob` rules (no qualified-name regex). Examples: `eventLabels.includeKeyGlobs` |

Examples: `urls`, `applicationNamespaceGlobs`, `passwordRegex`, `conditionExpr`, `urlTemplate`, `commitMessageTemplate`, `healthLua`, `discoveryLua`, `actionLua`, `maxPayloadSize`, `streamedManifest.maxTarSize`, `sensitiveMaskAnnotationKeys`, `eventLabels.includeKeyGlobs`.

**Retry / exponential backoff:** use the shared `BackoffConfig` shape (`duration`, `factor`, `maxDuration`) — same names as Application `spec.syncPolicy.retry.backoff`. Do **not** invent parallel names like `timeout`/`cap`/`baseBackoff` for the same concepts. Nest under `backoff` (or `retry.backoff`); put attempt limits on a sibling (`retry.max`, Application `retry.limit`).

**Enable / disable booleans:** do not require these words on every bool. When you do use them, use the **past-tense suffix** `*Enabled` (e.g. `tlsEnabled`, `profileEnabled`) — not verb prefixes (`enableGzip`, `disableTLS`) and not `*Disabled` double-negatives. Bare `enabled` is fine as the sole toggle on a small feature settings object (`helm.enabled`, `exec.enabled`, `progressiveSyncs.enabled`). Prefer an enum when the toggle may grow into multiple algorithms or modes (e.g. `server.compression: gzip|disabled`).

Always use positive polarity in the CR (`*Enabled`). When the legacy key uses the opposite sense (`disable.*`, `*.insecure`, `*.plaintext`), invert in mapping and note that in the `Migration:` line.

**Parallelism limits:** name concurrency caps `parallelismLimit` (or `subjectParallelismLimit` when the parent has several). Do **not** use `concurrent*Max` for the same idea. Nest under the limited concern when practical (`webhook.parallelismLimit`). Worker-pool sizes that are not a parallelism cap stay separate (e.g. `refreshWorkers`).

**TLS flags:**
- Prefer `tlsEnabled` for whether TLS is used (API server, repo-server, Dex, client dials, OTLP). Invert mapping from legacy `*.insecure` / `*.plaintext` / `disable.tls`.
- `insecureSkipVerify` — keep TLS, but skip certificate verification. Prefer this over inverted `strictTLS`. Legacy `*.strict.tls=true` maps to `insecureSkipVerify=false`.
- Scope in the field name when needed (`oidcInsecureSkipVerify`).

### 3. Choose types and validation

Fields stay **optional** (`omitempty`, nilable roots) so unset vs empty stays distinguishable and adoption can be gradual. When a value *is* set, prefer validating it in the CRD.

| Kind of value | Prefer | Notes |
| --- | --- | --- |
| Timeout / TTL / jitter / session length | `metav1.Duration` | Avoid raw seconds `int` or free-form strings unless necessary |
| Size limits (bytes / payloads) | `resource.Quantity` + `*Size` name | e.g. `maxPayloadSize: 50M` — no `*MB` int fields |
| Whole secret Argo controls | `corev1.SecretKeySelector` | e.g. OIDC `clientSecretRef` |
| Label / annotation keys, kinds, API groups | OpenAPI `Pattern` / `items:Pattern` | **Never** CEL `matches()` on unbounded lists (budget cost) |
| Absolute URLs (scalar or list) | CEL `isURL` (+ scheme allowlist) | More robust than a URL regex |
| URL map values | `AbsoluteHTTPURL` (typed string + Pattern) | Maps don’t get list-style CEL as cleanly |
| Dynamic collections | `[]T` with `+listType=map` + `+listMapKey=…` | Not `map[string]T` for CRD lists you want to SSA-merge by key |
| Unique scalar lists (sets) | `[]string` / `[]int` with `+listType=set` | e.g. `accounts[].capabilities` — rejects duplicates like `login` twice |

`$string` / partial secret interpolation stays only where the structure is opaque (Dex connector `config`) or partial insertion is intentional (header `prefix-$secret`). It is OK if some legacy CM values cannot convert.

### 4. Document the field

Every field needs a short prose description (it becomes the OpenAPI `description`). Include the legacy key when known: `argocd-cm: url`, `argocd-cmd-params-cm: server.insecure`.

Add a **`Migration:`** line using the role templates:

```text
// Migration: component group; no legacy key — see child fields.
// Migration: organizational subgroup; no legacy key — see child fields.
// Migration: if non-nil, takes precedence over argocd-cm oidc.config as a whole, including all child fields.
// Migration: if non-nil, takes precedence over argocd-cm statusbadge.* as a group (children apply from the CR; no merge with legacy siblings under that family).
// Migration: if set, takes precedence over argocd-cm url.
// Migration: if present, takes precedence over argocd-cm accounts.*; replaces the whole collection.
// Migration: composite child of OIDC / oidc.config; not merged independently with ConfigMaps.
```

Precedence rule: a **set** CRD value always wins over legacy; unset falls through.

### 5. Wire mapping and tests

1. Update `pkg/mapping` (`FromConfigMaps` / `ToConfigMaps`) for the new field.
2. Extend `testdata/sample-cms` and/or `pkg/mapping` tests so round-trip coverage exists.
3. Run:

```bash
make generate manifests build test
```

### Checklist

- [ ] Placed under the right Spec component (or justified a new group)
- [ ] Migration role + `Migration:` comment match pointer/collection shape
- [ ] Naming suffix applied if URL / Glob / Regex / Expr / Template / Lua / Size / Keys / KeyGlobs
- [ ] Retry/backoff uses shared `BackoffConfig` (`duration` / `factor` / `maxDuration`), not ad-hoc `timeout`/`cap`/`baseBackoff`
- [ ] TLS flags use `tlsEnabled` / `insecureSkipVerify` (not `insecure`/`plaintext`/`strictTLS`/`tlsDisabled`)
- [ ] Enable/disable bools use `*Enabled` (never `*Disabled`); invert mapping when legacy polarity differs
- [ ] Concurrency caps use `*ParallelismLimit` (not `concurrent*Max`)
- [ ] Duration / Quantity (`*Size`) / SecretKeySelector / Pattern / isURL chosen appropriately (path-or-URL exceptions documented)
- [ ] Struct slices use `+listType=map` (+ `listMapKey`) when SSA-mergeable; scalar uniqueness uses `+listType=set`; else `atomic`
- [ ] Prose docs + legacy key + `Migration:` line called out
- [ ] Mapping + test updated; `make generate manifests build test` green

## Settled design (short)

- Spec-only CRD; no status
- Nilable fields; no CRD defaults — empty vs unset distinguishable
- Dynamic collections as keyed lists (`+listType=map`)
- Dex: typed connector envelope + opaque `config`; RBAC: raw `policy.csv`
- Nothing from `argocd-secret` on the CRD except via explicit `SecretKeySelector` where we own the shape
- Operator-manual keys for `argocd-cm` / `argocd-cmd-params-cm` / `argocd-rbac-cm` are populated; trust-store CMs (SSH known hosts, TLS certs, GPG) and secrets stay out of scope for now

## License

Apache-2.0
