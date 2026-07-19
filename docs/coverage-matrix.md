# Legacy key coverage matrix

This inventory lists legacy ConfigMap / cmd-params keys that **are mapped** in `pkg/mapping` (plus patterns exercised in `testdata/sample-cms`). It is **not** a complete scrape of upstream [argoproj/argo-cd](https://github.com/argoproj/argo-cd) operator-manual keys — only what this prototype implements.

See [PHASE_P_EVAL.md](../PHASE_P_EVAL.md) for prototype scope and deferred items.

**Legend:** Mapped = ✅ when `pkg/mapping` reads/writes the key (or prefix pattern).

## Covered by (golden cases)

Subsystem → primary case paths under `testdata/cases/` (direction `roundtrip` unless noted). Roundtrip cases also assert readable `expected/configuration.yaml`.

| Subsystem | Cases |
| --- | --- |
| OIDC / Dex / auth TLS | `oidc/full-config`, `oidc/clientsecret-dollar-ref`, `dex/full-config`, `dex/github-connector`, `auth/oidc-tls-skip-verify-and-additional-urls`; `to/oidc-secret-custom-name` (error diagnostic) |
| Resource filters | `resource/full-filters`, `resource/compareoptions-off` |
| Resource customizations | `resource-customizations/split-keys-full`, `resource-customizations/monolithic-full`, `resource-customizations/split-keys-actions`, `resource-customizations/monolithic-blob` |
| Controller cmd-params | `cmd-params/controller-full` |
| Server / repo-server cmd-params | `cmd-params/server-full`, `cmd-params/reposerver-full`, `cmd-params/server-cache-tls`, `cmd-params/k8s-client-and-quantity` |
| AppSet / commit / dex / notifications | `cmd-params/applicationset-full`, `cmd-params/applicationset-flags`, `cmd-params/commit-dex-notifications`, `cmd-params/logging-and-commit` |
| Redis / OTLP / repo client | `redis/full`, `otlp/headers-and-attrs`, `repo-server-client/dual-prefix-collapse` |
| Application / users / misc | `application/product-config`, `users/full`, `misc/full`, `misc/ga-exec-statusbadge`, `to/full-server` |
| Extensions / deep links / UI / help | `extensions/multi-service`, `extensions/backend-services`, `deep-links/application-project`, `ui/banner-and-css`, `help/download-bins` |
| Helm / kustomize / accounts / webhook | `helm/enable-and-schemes`, `kustomize/versions-and-options`, `accounts/admin-and-robot`, `webhook/max-payload-mb` |
| RBAC | `rbac/full`, `rbac/scopes-csv`, `rbac/policy-overlays` |
| Regressions | `regressions/unknown-key-warn`, `regressions/bad-duration`, `regressions/bad-seconds-duration`, `regressions/empty-quantity` |
| Broad sample | `roundtrip/sample-cms`, `global-projects/basic`, `to/minimal-server-url` |

---

## argocd-cm

| Key | Mapped | CR field | Notes |
| --- | --- | --- | --- |
| `url` | ✅ | `spec.server.urls[0]` | Primary external URL |
| `additionalUrls` | ✅ | `spec.server.urls[1:]` | JSON/YAML array in CM |
| `oidc.tls.insecure.skip.verify` | ✅ | `spec.server.oidcInsecureSkipVerify` | |
| `oidc.config` | ✅ | `spec.server.oidc` | Composite override (whole blob) |
| `dex.config` | ✅ | `spec.server.dex` | Composite override |
| `admin.enabled` | ✅ | `spec.server.accounts[]` (admin) | |
| `accounts.*` | ✅ | `spec.server.accounts[]` | Prefix: capabilities + `accounts.<name>.enabled` |
| `application.instanceLabelKey` | ✅ | `spec.controller.instanceLabelKey` | |
| `application.resourceTrackingMethod` | ✅ | `spec.controller.resourceTrackingMethod` | |
| `application.allowedNodeLabels` | ✅ | `spec.controller.allowedNodeLabelKeys` | CSV |
| `application.sync.impersonation.enabled` | ✅ | `spec.controller.sync.impersonation.mode` | With `.enforced` → disabled/optional/required |
| `application.sync.impersonation.enforced` | ✅ | `spec.controller.sync.impersonation.mode` | Paired with `.enabled` |
| `application.sync.requireOverridePrivilegeForRevisionSync` | ✅ | `spec.controller.sync.requireOverridePrivilegeForRevisionSync` | |
| `application.links` | ✅ | `spec.server.deepLinks.application` | YAML list |
| `project.links` | ✅ | `spec.server.deepLinks.project` | |
| `resource.links` | ✅ | `spec.server.deepLinks.resource` | |
| `cluster.inClusterEnabled` | ✅ | `spec.cluster.inClusterEnabled` | Also cmd-params |
| `resource.exclusions` | ✅ | `spec.controller.resource.exclusions` | |
| `resource.inclusions` | ✅ | `spec.controller.resource.inclusions` | |
| `resource.compareoptions` | ✅ | `spec.controller.diff.compareOptions` | Composite |
| `resource.respectRBAC` | ✅ | `spec.controller.resource.respectRBAC` | |
| `resource.ignoreResourceUpdatesEnabled` | ✅ | `spec.controller.diff.ignoreResourceUpdatesEnabled` | |
| `resource.customLabels` | ✅ | `spec.controller.resource.customLabelKeys` | |
| `resource.sensitive.mask.annotations` | ✅ | `spec.controller.resource.sensitiveMaskAnnotationKeys` | |
| `resource.includeEventLabelKeys` | ✅ | `spec.controller.resource.eventLabels.includeKeyGlobs` | |
| `resource.excludeEventLabelKeys` | ✅ | `spec.controller.resource.eventLabels.excludeKeyGlobs` | |
| `resource.customizations` | ✅ | `spec.controller.resource.customizations` | Legacy monolithic key |
| `resource.customizations.health.*` | ✅ | `spec.controller.resource.customizations[].healthLua` | Prefix + GVK suffix |
| `resource.customizations.useOpenLibs.*` | ✅ | `spec.controller.resource.customizations[].useOpenLibs` | |
| `resource.customizations.actions.*` | ✅ | `spec.controller.resource.customizations[].actions` | |
| `resource.customizations.ignoreDifferences.*` | ✅ | `spec.controller.resource.customizations[].ignoreDifferences` | |
| `resource.customizations.ignoreResourceUpdates.*` | ✅ | `spec.controller.resource.customizations[].ignoreResourceUpdates` | |
| `resource.customizations.knownTypeFields.*` | ✅ | `spec.controller.resource.customizations[].knownTypeFields` | |
| `globalProjects` | ✅ | `spec.controller.globalProjects` | |
| `ui.cssurl` | ✅ | `spec.server.ui.cssURL` | Path-or-URL |
| `ui.bannercontent` | ✅ | `spec.server.ui.banner.content` | |
| `ui.bannerurl` | ✅ | `spec.server.ui.banner.url` | |
| `ui.bannerpermanent` | ✅ | `spec.server.ui.banner.permanent` | |
| `ui.bannerposition` | ✅ | `spec.server.ui.banner.position` | |
| `ui.loginButtonText` | ✅ | `spec.server.ui.loginButtonText` | |
| `users.anonymous.enabled` | ✅ | `spec.server.users.anonymousEnabled` | |
| `users.session.duration` | ✅ | `spec.server.users.sessionDuration` | |
| `passwordPattern` | ✅ | `spec.server.users.passwordRegex` | |
| `server.maxPodLogsToRender` | ✅ | `spec.server.logs.maxPodsToRender` | |
| `help.chatUrl` | ✅ | `spec.server.help.chat.url` | |
| `help.chatText` | ✅ | `spec.server.help.chat.text` | |
| `help.download.*` | ✅ | `spec.server.help.binaryURLs` | Prefix per arch |
| `extension.config` | ✅ | `spec.server.extensions[]` | YAML envelope |
| `extension.config.*` | ✅ | `spec.server.extensions[]` | Per-extension backend |
| `kustomize.enable` | ✅ | `spec.repoServer.kustomize.enabled` | |
| `kustomize.buildOptions` | ✅ | `spec.repoServer.kustomize.buildOptions` | |
| `kustomize.path.*` | ✅ | `spec.repoServer.kustomize.versions[].path` | Per version name |
| `kustomize.buildOptions.*` | ✅ | `spec.repoServer.kustomize.versions[].buildOptions` | |
| `helm.enable` | ✅ | `spec.repoServer.helm.enabled` | |
| `helm.valuesFileSchemes` | ✅ | `spec.repoServer.helm.valuesFileSchemes` | |
| `jsonnet.enable` | ✅ | `spec.repoServer.jsonnet.enabled` | |
| `ga.trackingid` | ✅ | `spec.server.googleAnalytics.trackingID` | |
| `ga.anonymizeusers` | ✅ | `spec.server.googleAnalytics.anonymizeUsers` | |
| `exec.enabled` | ✅ | `spec.server.exec.enabled` | |
| `exec.shells` | ✅ | `spec.server.exec.shells` | |
| `statusbadge.enabled` | ✅ | `spec.server.statusBadge.enabled` | |
| `statusbadge.url` | ✅ | `spec.server.statusBadge.url` | |
| `webhook.maxPayloadSizeMB` | ✅ | `spec.server.webhook.maxPayloadSize` | Quantity on CR |
| `webhook.refresh.jitter` | ✅ | `spec.server.webhook.refresh.jitter` | |
| `webhook.refresh.jitter.threshold` | ✅ | `spec.server.webhook.refresh.jitterThreshold` | |
| `sourceHydrator.commitMessageTemplate` | ✅ | `spec.controller.sourceHydrator.commitMessageTemplate` | |
| `sourceHydrator.readmeMessageTemplate` | ✅ | `spec.controller.sourceHydrator.readmeMessageTemplate` | |
| `commit.author.name` | ✅ | `spec.commitServer.commit.author.name` | |
| `commit.author.email` | ✅ | `spec.commitServer.commit.author.email` | |
| `timeout.reconciliation` | ✅ | `spec.controller.reconciliation.timeout` | Also cmd-params |
| `timeout.reconciliation.jitter` | ✅ | `spec.controller.reconciliation.jitter` | |
| `installationID` | ✅ | `spec.installationID` | |
| `server.rbac.disableApplicationFineGrainedRBACInheritance` | ✅ | `spec.server.rbac.applicationFineGrainedInheritanceEnabled` | Inverted |

---

## argocd-cmd-params-cm

| Key | Mapped | CR field | Notes |
| --- | --- | --- | --- |
| `application.namespaces` | ✅ | `spec.applicationNamespaceGlobs` | |
| `repo.server` | ✅ | `spec.repoServer.client.address` | |
| `commit.server` | ✅ | `spec.commitServer.address` | |
| `log.format.timestamp` | ✅ | `spec.logging.formatTimestamp` | |
| `redis.server` | ✅ | `spec.redis.server` | |
| `redis.compression` | ✅ | `spec.redis.compression` | |
| `redis.sentinel.hosts` | ✅ | `spec.redis.sentinel.hosts` | |
| `redis.sentinel.master` | ✅ | `spec.redis.sentinel.master` | |
| `redis.db` | ✅ | `spec.redis.db` | |
| `otlp.address` | ✅ | `spec.otlp.address` | |
| `otlp.insecure` | ✅ | `spec.otlp.tlsEnabled` | Inverted |
| `otlp.headers` | ✅ | `spec.otlp.headers` | |
| `otlp.attrs` | ✅ | `spec.otlp.attrs` | |
| `otlp.sample.ratio` | ✅ | `spec.otlp.sampleRatio` | |
| `server.insecure` | ✅ | `spec.server.tlsEnabled` | Inverted |
| `server.log.format` | ✅ | `spec.server.log.format` | |
| `server.log.level` | ✅ | `spec.server.log.level` | |
| `server.basehref` | ✅ | `spec.server.baseHref` | |
| `server.rootpath` | ✅ | `spec.server.rootPath` | |
| `server.staticassets` | ✅ | `spec.server.staticAssetsPath` | |
| `server.listen.address` | ✅ | `spec.server.listen.address` | |
| `server.metrics.listen.address` | ✅ | `spec.server.listen.metricsAddress` | |
| `server.disable.auth` | ✅ | `spec.server.authEnabled` | Inverted |
| `server.enable.gzip` | ✅ | `spec.server.compression` (`disabled`/`gzip`) | |
| `server.enable.proxy.extension` | ✅ | `spec.server.proxyExtensionEnabled` | |
| `server.sync.replace.allowed` | ✅ | `spec.server.syncReplaceAllowed` | |
| `server.x.frame.options` | ✅ | `spec.server.xFrameOptions` | |
| `server.api.content.types` | ✅ | `spec.server.apiContentTypes` | |
| `server.profile.enabled` | ✅ | `spec.server.profileEnabled` | |
| `server.grpc.enable.txt.service.config` | ✅ | `spec.server.grpcTXTServiceConfigEnabled` | |
| `server.dex.server` | ✅ | `spec.server.dexServer.address` | |
| `server.dex.server.plaintext` | ✅ | `spec.server.dexServer.tlsEnabled` | Inverted |
| `server.dex.server.strict.tls` | ✅ | `spec.server.dexServer.insecureSkipVerify` | Inverted |
| `server.app.state.cache.expiration` | ✅ | `spec.server.cache.appStateExpiration` | |
| `server.connection.status.cache.expiration` | ✅ | `spec.server.cache.connectionStatusExpiration` | |
| `server.default.cache.expiration` | ✅ | `spec.server.cache.defaultExpiration` | |
| `server.oidc.cache.expiration` | ✅ | `spec.server.cache.oidcExpiration` | |
| `server.glob.cache.size` | ✅ | `spec.server.cache.globCacheSize` | |
| `server.tls.minversion` | ✅ | `spec.server.tls.minVersion` | Via `mapTLSVersion` |
| `server.tls.maxversion` | ✅ | `spec.server.tls.maxVersion` | |
| `server.tls.ciphers` | ✅ | `spec.server.tls.ciphers` | |
| `server.webhook.parallelism.limit` | ✅ | `spec.server.webhook.parallelismLimit` | |
| `server.webhook.refresh.workers` | ✅ | `spec.server.webhook.refresh.workers` | |
| `server.k8s.client.*` | ✅ | `spec.server.k8sClient.*` (incl. `tcp.*`) | QPS, burst, timeouts, retry |
| `server.repo.server.*` | ✅ | `spec.repoServer.client.*` | Collapsed with controller variant |
| `reposerver.log.format` | ✅ | `spec.repoServer.log.format` | |
| `reposerver.log.level` | ✅ | `spec.repoServer.log.level` | |
| `reposerver.parallelism.limit` | ✅ | `spec.repoServer.parallelismLimit` | |
| `reposerver.listen.address` | ✅ | `spec.repoServer.listen.address` | |
| `reposerver.metrics.listen.address` | ✅ | `spec.repoServer.listen.metricsAddress` | |
| `reposerver.disable.tls` | ✅ | `spec.repoServer.tlsEnabled` | Inverted |
| `reposerver.enable.git.submodule` | ✅ | `spec.repoServer.git.submoduleEnabled` | |
| `reposerver.allow.oob.symlinks` | ✅ | `spec.repoServer.allowOOBSymlinks` | |
| `reposerver.include.hidden.directories` | ✅ | `spec.repoServer.includeHiddenDirectories` | |
| `reposerver.enable.builtin.git.config` | ✅ | `spec.repoServer.git.builtinConfigEnabled` | |
| `reposerver.git.request.timeout` | ✅ | `spec.repoServer.git.requestTimeout` | |
| `reposerver.git.lsremote.parallelism.limit` | ✅ | `spec.repoServer.git.lsRemoteParallelismLimit` | |
| `reposerver.default.cache.expiration` | ✅ | `spec.repoServer.cache.defaultExpiration` | |
| `reposerver.repo.cache.expiration` | ✅ | `spec.repoServer.cache.repoExpiration` | |
| `reposerver.max.combined.directory.manifests.size` | ✅ | `spec.repoServer.maxCombinedDirectoryManifestsSize` | |
| `reposerver.streamed.manifest.max.tar.size` | ✅ | `spec.repoServer.streamedManifest.maxTarSize` | |
| `reposerver.streamed.manifest.max.extracted.size` | ✅ | `spec.repoServer.streamedManifest.maxExtractedSize` | |
| `reposerver.plugin.use.manifest.generate.paths` | ✅ | `spec.repoServer.plugin.useManifestGeneratePaths` | |
| `reposerver.profile.enabled` | ✅ | `spec.repoServer.profileEnabled` | |
| `reposerver.grpc.enable.txt.service.config` | ✅ | `spec.repoServer.grpcTXTServiceConfigEnabled` | |
| `reposerver.client.ca.path` | ✅ | `spec.repoServer.clientCAPath` | |
| `reposerver.tls.*` | ✅ | `spec.repoServer.tls.*` | |
| `reposerver.oci.manifest.max.extracted.size` | ✅ | `spec.repoServer.oci.manifest.maxExtractedSize` | |
| `reposerver.disable.oci.manifest.max.extracted.size` | ✅ | `spec.repoServer.oci.manifest.maxExtractedSizeEnabled` | Inverted |
| `reposerver.oci.layer.media.types` | ✅ | `spec.repoServer.oci.layerMediaTypes` | |
| `reposerver.plugin.tar.exclusions` | ✅ | `spec.repoServer.plugin.tarExclusionGlobs` | `;` separator |
| `commitserver.log.format` | ✅ | `spec.commitServer.log.format` | |
| `commitserver.log.level` | ✅ | `spec.commitServer.log.level` | |
| `commitserver.listen.address` | ✅ | `spec.commitServer.listen.address` | |
| `commitserver.metrics.listen.address` | ✅ | `spec.commitServer.listen.metricsAddress` | |
| `commitserver.grpc.enable.txt.service.config` | ✅ | `spec.commitServer.grpcTXTServiceConfigEnabled` | |
| `dexserver.log.format` | ✅ | `spec.dexServer.log.format` | |
| `dexserver.log.level` | ✅ | `spec.dexServer.log.level` | |
| `dexserver.disable.tls` | ✅ | `spec.dexServer.tlsEnabled` | Inverted |
| `dexserver.connector.failure.continue` | ✅ | `spec.dexServer.connectorFailureContinue` | |
| `controller.sharding.algorithm` | ✅ | `spec.controller.sharding.algorithm` | |
| `controller.log.format` | ✅ | `spec.controller.log.format` | |
| `controller.log.level` | ✅ | `spec.controller.log.level` | |
| `controller.status.processors` | ✅ | `spec.controller.processors.status` | |
| `controller.operation.processors` | ✅ | `spec.controller.processors.operation` | |
| `controller.hydration.processors` | ✅ | `spec.controller.processors.hydration` | |
| `controller.app.state.cache.expiration` | ✅ | `spec.controller.cache.appStateExpiration` | |
| `controller.default.cache.expiration` | ✅ | `spec.controller.cache.defaultExpiration` | |
| `controller.resource.health.persist` | ✅ | `spec.controller.resourceHealthPersist` | |
| `controller.kubectl.parallelism.limit` | ✅ | `spec.controller.kubectlParallelismLimit` | |
| `controller.diff.server.side` | ✅ | `spec.controller.diff.serverSide.enabled` | |
| `controller.metrics.cache.expiration` | ✅ | `spec.controller.metrics.cacheExpiration` | |
| `controller.metrics.application.labels` | ✅ | `spec.controller.metrics.application.labelKeys` | |
| `controller.metrics.application.conditions` | ✅ | `spec.controller.metrics.application.conditions` | |
| `controller.metrics.cluster.labels` | ✅ | `spec.controller.metrics.cluster.labelKeys` | |
| `controller.self.heal.timeout.seconds` | ✅ | `spec.controller.selfHeal.timeout` | |
| `controller.self.heal.backoff.timeout.seconds` | ✅ | `spec.controller.selfHeal.backoff.duration` | |
| `controller.self.heal.backoff.factor` | ✅ | `spec.controller.selfHeal.backoff.factor` | |
| `controller.self.heal.backoff.cap.seconds` | ✅ | `spec.controller.selfHeal.backoff.maxDuration` | |
| `controller.sync.timeout.seconds` | ✅ | `spec.controller.sync.timeout` | |
| `controller.sync.wave.delay.seconds` | ✅ | `spec.controller.sync.wave.delay` | |
| `controller.profile.enabled` | ✅ | `spec.controller.profileEnabled` | |
| `controller.grpc.enable.txt.service.config` | ✅ | `spec.controller.grpcTXTServiceConfigEnabled` | |
| `controller.repo.error.grace.period.seconds` | ✅ | `spec.controller.repoErrorGracePeriod` | |
| `controller.cluster.cache.batch.events.processing` | ✅ | `spec.controller.clusterCache.batchEventsProcessing` | |
| `controller.cluster.cache.events.processing.interval` | ✅ | `spec.controller.clusterCache.eventsProcessingInterval` | |
| `controller.k8s.client.*` | ✅ | `spec.controller.k8sClient.*` (incl. `tcp.*`) | |
| `controller.repo.server.*` | ✅ | `spec.repoServer.client.*` | Preferred over `server.repo.server.*` |
| `hydrator.enabled` | ✅ | `spec.controller.hydrator.enabled` | |
| `applicationsetcontroller.namespaces` | ✅ | `spec.applicationSet.namespaceGlobs` | |
| `applicationsetcontroller.policy` | ✅ | `spec.applicationSet.policy` | |
| `applicationsetcontroller.allowed.scm.providers` | ✅ | `spec.applicationSet.allowedSCMProviderURLs` | |
| `applicationsetcontroller.global.preserved.annotations` | ✅ | `spec.applicationSet.globalPreserved.annotationKeys` | |
| `applicationsetcontroller.global.preserved.labels` | ✅ | `spec.applicationSet.globalPreserved.labelKeys` | |
| `applicationsetcontroller.enable.scm.providers` | ✅ | `spec.applicationSet.scmProvidersEnabled` | |
| `applicationsetcontroller.enable.policy.override` | ✅ | `spec.applicationSet.policyOverrideEnabled` | |
| `applicationsetcontroller.enable.progressive.syncs` | ✅ | `spec.applicationSet.progressiveSyncs.enabled` | |
| `applicationsetcontroller.enable.git.submodule` | ✅ | `spec.applicationSet.gitSubmoduleEnabled` | |
| `applicationsetcontroller.enable.new.git.file.globbing` | ✅ | `spec.applicationSet.newGitFileGlobbingEnabled` | |
| `applicationsetcontroller.enable.tokenref.strict.mode` | ✅ | `spec.applicationSet.tokenRefStrictModeEnabled` | |
| `applicationsetcontroller.enable.github.api.metrics` | ✅ | `spec.applicationSet.gitHubAPIMetricsEnabled` | |
| `applicationsetcontroller.dryrun` | ✅ | `spec.applicationSet.dryRun` | |
| `applicationsetcontroller.debug` | ✅ | `spec.applicationSet.log.level=debug` | Legacy alias; collapsed on CM→CR (takes precedence over log.level) |
| `applicationsetcontroller.enable.leader.election` | ✅ | `spec.applicationSet.leaderElectionEnabled` | |
| `applicationsetcontroller.profile.enabled` | ✅ | `spec.applicationSet.profileEnabled` | |
| `applicationsetcontroller.grpc.enable.txt.service.config` | ✅ | `spec.applicationSet.grpcTXTServiceConfigEnabled` | |
| `applicationsetcontroller.concurrent.reconciliations.max` | ✅ | `spec.applicationSet.reconciliationsParallelismLimit` | |
| `applicationsetcontroller.requeue.after` | ✅ | `spec.applicationSet.requeueAfter` | |
| `applicationsetcontroller.webhook.parallelism.limit` | ✅ | `spec.applicationSet.webhookParallelismLimit` | |
| `applicationsetcontroller.status.max.resources.count` | ✅ | `spec.applicationSet.statusMaxResourcesCount` | |
| `applicationsetcontroller.scm.root.ca.path` | ✅ | `spec.applicationSet.scmRootCAPath` | |
| `applicationsetcontroller.log.format` | ✅ | `spec.applicationSet.log.format` | |
| `applicationsetcontroller.log.level` | ✅ | `spec.applicationSet.log.level` | |
| `applicationsetcontroller.k8s.client.*` | ✅ | `spec.applicationSet.k8sClient.*` (incl. `tcp.*`) | |
| `applicationsetcontroller.repo.server.*` | ✅ | `spec.applicationSet.repoServer.*` | mTLS paths |
| `notificationscontroller.log.format` | ✅ | `spec.notifications.log.format` | Out of scope for notifications-cm content |
| `notificationscontroller.log.level` | ✅ | `spec.notifications.log.level` | |
| `notificationscontroller.processors.count` | ✅ | `spec.notifications.processorsCount` | |
| `notificationscontroller.selfservice.enabled` | ✅ | `spec.notifications.selfServiceEnabled` | |
| `notificationscontroller.repo.server.plaintext` | ✅ | `spec.notifications.tlsEnabled` | Inverted |
| `notificationscontroller.repo.server.*` | ✅ | `spec.notifications.repoServer.*` | Cert paths |

---

## argocd-rbac-cm

| Key | Mapped | CR field | Notes |
| --- | --- | --- | --- |
| `policy.default` | ✅ | `spec.server.rbac.default` | |
| `scopes` | ✅ | `spec.server.rbac.scopes` | JSON or CSV |
| `policy.matchMode` | ✅ | `spec.server.rbac.matchMode` | |
| `policy.csv` | ✅ | `spec.server.rbac.policyCSV` | Raw CSV |
| `policy.<name>.csv` | ✅ | `spec.server.rbac.overlays[]` | e.g. `policy.extra.csv` |

---

## Known gaps / not yet mapped

Categories Argo CD may expose but **this prototype does not map** (honest partial list — not exhaustive vs upstream):

| Category | Examples | Notes |
| --- | --- | --- |
| **Secrets** | `argocd-secret` (`server.secretkey`, webhook HMAC, TLS keys, repo creds) | By design; use `SecretKeySelector` only where CR owns shape |
| **Trust-store ConfigMaps** | SSH known hosts, TLS cert bundles, GPG keys | Deferred per [PHASE_P_EVAL.md](../PHASE_P_EVAL.md) |
| **Notifications content** | `argocd-notifications-cm` templates/triggers | Only cmd-params for notifications *controller* wired |
| **Repository / cluster Secrets** | Repo URLs, cluster credentials | Out of scope |
| **Account passwords** | `accounts.<name>.password` | Lives in secrets, not CRD |
| **Rare / deprecated CM keys** | Keys removed or added in newer Argo CD releases without a mapping PR | Inventory is grep-based, not upstream-complete |
| **Full CEL offline validation** | All CRD `XValidation` rules | CLI `validate` mirrors name + http(s) URLs only; cluster admission required for full CEL |

When adding mappings, update this file and `pkg/mapping` together.
