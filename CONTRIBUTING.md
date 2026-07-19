# Contributing

Thank you for contributing to the ArgoCDConfiguration prototype.

## Before you start

This project is **experimental**. See [README.md](README.md) for the user-facing workflow (install CLI, convert ConfigMaps, generate ConfigMaps from the CR).

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
    configuration.yaml          # direction: to | to-roundtrip
  expected/
    configuration.yaml          # direction: from | roundtrip | to-roundtrip (post-FromConfigMaps CR)
    configmaps/                 # direction: to | roundtrip | to-roundtrip
      argocd-cm.yaml
      ...
    diagnostics.yaml            # always golden when present
```

### `case.yaml` (required fields)

| Field | Required | Values / notes |
| --- | --- | --- |
| `description` | yes | One-line summary shown in test failures |
| `direction` | yes | Prefer `roundtrip` or `to-roundtrip` (see below). `from` / `to` only when a full loop is the wrong fixture |
| `name` | no | CR metadata name (default: singleton name) |
| `namespace` | no | Namespace for ConfigMaps (default: `argocd`) |
| `strict` | no | If true, mapping warnings fail the case |
| `validate` | no | If true, run OpenAPI/CEL on CRs (errors and warnings fail) |
| `strictDecode` | no | If true, reject unknown fields when loading `input/configuration.yaml` |
| `issue` | no | Link or id for bug repro cases |

### Choosing `direction`

**Prefer a full loop** whenever the mapping should survive conversion both ways:

| Direction | Flow | Prefer when… |
| --- | --- | --- |
| `roundtrip` | CM → CR → CM | Default for subsystem coverage starting from legacy ConfigMaps |
| `to-roundtrip` | CR → CM → CR → CM | Starting from a CR (e.g. `EXAMPLE.yaml`); also asserts ConfigMap stability via `DiffConfigMapDataNormalized` |

The fullest case is `roundtrip/example-full` (symlinked to repo-root `EXAMPLE.yaml`). After regenerating `EXAMPLE.yaml`, refresh its goldens with:

```bash
go test ./pkg/mapping -run 'TestCases/roundtrip/example-full' -update
```

**Use one-way directions only when a loop would be meaningless or hide the behavior under test:**

| Direction | Flow | Appropriate when… | Examples |
| --- | --- | --- | --- |
| `from` | CM → CR | You care about CM→CR parse shape or diagnostics, and there is no useful CR→CM expectation (invalid input, warn-then-normalize, drop-on-read) | `regressions/bad-duration`, `unknown-key-warn`, `resource/compareoptions-off` |
| `to` | CR → CM | You care about CR→CM emit or diagnostics that cannot roundtrip cleanly (lossy writers, documented limitations) | `to/oidc-secret-custom-name` (error diagnostic), focused CR→CM writer locks under `to/` |

Do **not** default to `from`/`to` for ordinary field coverage — that under-tests the other direction. If both sides should work, use `roundtrip` or `to-roundtrip`.

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
| **Full example** | `EXAMPLE.yaml` + `roundtrip/example-full` | Every CR field set, CEL, CR→CM→CR→CM stability, golden CMs |
| **Unit tables** | `pkg/mapping/mapping_table_test.go`, `helpers_unit_test.go`, … | Focused mapping rules and edge cases |
| **Property** | `pkg/mapping/property_test.go` | CM → CR → CM → CR semantic equality invariant |
| **Fuzz** | `pkg/mapping/fuzz_test.go`, `pkg/convert/convert_test.go` | Randomized inputs seeded from corpus + sample ConfigMaps |

`make cover` reports local statement coverage for handwritten code under `pkg/` and `cmd/`. CI uploads to [Codecov](https://codecov.io/gh/crenshaw-dev/argocd-config), which gates PRs via `codecov.yml` (project must not drop more than 1% vs base; patch coverage target 80%). Generated/`testdata`/`hack` paths and `examplefill.go` are ignored there.

### Round-trip self-check normalizations

`from-configmaps` always round-trips CR→CM and warns on remaining diffs (failing under `--strict`). Known-safe formatting differences are filtered by equalers in [`pkg/mapping/roundtrip_normalize.go`](pkg/mapping/roundtrip_normalize.go) (`RoundTripValueEqualers`, plus key-pair helpers like impersonation mode).

When `--strict` flags a benign serialization diff, add a focused equaler + unit test there rather than weakening the check. Use `--permissive` only as a local escape hatch to inspect output without the self-check.

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

## Pull requests

- Keep diffs focused; avoid unrelated refactors
- Ensure CI passes (formatting, vet, tests, codegen drift check)
- Do not commit secrets or environment-specific paths

## Design decisions

Architecture decision records live in [docs/adr/](docs/adr/).
