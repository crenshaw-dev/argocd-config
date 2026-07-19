package mapping

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
)

func mapServerCmdParams(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	s := ensureServer(spec)
	if v, ok := kt.get("server.basehref"); ok {
		s.BaseHref = v
	}
	if v, ok := kt.get("server.rootpath"); ok {
		s.RootPath = v
	}
	if v, ok := kt.get("server.staticassets"); ok {
		s.StaticAssetsPath = v
	}
	if listen := mapServerListen(kt); listen != nil {
		s.Listen = listen
	}
	setBoolPtrInverted(kt, "server.disable.auth", &s.AuthEnabled)
	if v, ok := kt.get("server.enable.gzip"); ok {
		if strings.EqualFold(v, "true") {
			s.Compression = "gzip"
		} else {
			s.Compression = "disabled"
		}
	}
	setBoolPtr(kt, "server.enable.proxy.extension", &s.ProxyExtensionEnabled)
	setBoolPtr(kt, "server.sync.replace.allowed", &s.SyncReplaceAllowed)
	if v, ok := kt.get("server.x.frame.options"); ok {
		s.XFrameOptions = v
	}
	if v, ok := kt.get("server.api.content.types"); ok && v != "" {
		s.APIContentTypes = splitCSV(v)
	}
	if v, ok := kt.get("server.content.security.policy"); ok {
		s.ContentSecurityPolicy = v
	}
	if v, ok := kt.get("server.http.cookie.maxnumber"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			s.HTTPCookieMaxNumber = &i
		}
	}
	setBoolPtr(kt, "server.profile.enabled", &s.ProfileEnabled)
	setBoolPtr(kt, "server.grpc.enable.txt.service.config", &s.GRPCTXTServiceConfigEnabled)

	dexChanged := false
	dex := &argov1alpha1.ServerDexConnectionConfig{}
	if v, ok := kt.get("server.dex.server"); ok {
		dex.Address = v
		dexChanged = true
	}
	if setBoolPtrInverted(kt, "server.dex.server.plaintext", &dex.TLSEnabled) {
		dexChanged = true
	}
	if v, ok := kt.get("server.dex.server.strict.tls"); ok {
		b := !strings.EqualFold(v, "true")
		dex.InsecureSkipVerify = &b
		dexChanged = true
	}
	if dexChanged {
		s.DexServer = dex
	}

	cacheChanged := false
	cache := &argov1alpha1.ServerCacheConfig{}
	if v, ok := kt.get("server.app.state.cache.expiration"); ok {
		cache.AppStateExpiration, _ = parseDurationPtr(diag, "server.app.state.cache.expiration", v)
		cacheChanged = true
	}
	if v, ok := kt.get("server.connection.status.cache.expiration"); ok {
		cache.ConnectionStatusExpiration, _ = parseDurationPtr(diag, "server.connection.status.cache.expiration", v)
		cacheChanged = true
	}
	if v, ok := kt.get("server.default.cache.expiration"); ok {
		cache.DefaultExpiration, _ = parseDurationPtr(diag, "server.default.cache.expiration", v)
		cacheChanged = true
	}
	if v, ok := kt.get("server.oidc.cache.expiration"); ok {
		cache.OIDCExpiration, _ = parseDurationPtr(diag, "server.oidc.cache.expiration", v)
		cacheChanged = true
	}
	if v, ok := kt.get("server.glob.cache.size"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			cache.GlobCacheSize = &i
			cacheChanged = true
		}
	}
	if cacheChanged {
		s.Cache = cache
	}

	if tls := mapTLSVersion(kt, diag, "server.tls"); tls != nil {
		s.TLS = tls
	}
	if kc := mapK8sClient(kt, diag, "server"); kc != nil {
		s.K8sClient = kc
	}

	if v, ok := kt.get("server.webhook.parallelism.limit"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			if s.Webhook == nil {
				s.Webhook = &argov1alpha1.WebhookConfig{}
			}
			s.Webhook.ParallelismLimit = &i
		}
	}
	if v, ok := kt.get("server.webhook.refresh.workers"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			if s.Webhook == nil {
				s.Webhook = &argov1alpha1.WebhookConfig{}
			}
			if s.Webhook.Refresh == nil {
				s.Webhook.Refresh = &argov1alpha1.WebhookRefreshConfig{}
			}
			s.Webhook.Refresh.Workers = &i
		}
	}
}

func unmapServerCmdParams(s *argov1alpha1.ServerConfig, data map[string]string) {
	if s == nil {
		return
	}
	setStr(data, "server.basehref", s.BaseHref)
	setStr(data, "server.rootpath", s.RootPath)
	setStr(data, "server.staticassets", s.StaticAssetsPath)
	unmapServerListen(s.Listen, data)
	setBoolKeyInverted(data, "server.disable.auth", s.AuthEnabled)
	switch s.Compression {
	case "gzip":
		data["server.enable.gzip"] = "true"
	case "disabled":
		data["server.enable.gzip"] = "false"
	}
	setBoolKey(data, "server.enable.proxy.extension", s.ProxyExtensionEnabled)
	setBoolKey(data, "server.sync.replace.allowed", s.SyncReplaceAllowed)
	setStr(data, "server.x.frame.options", s.XFrameOptions)
	if len(s.APIContentTypes) > 0 {
		data["server.api.content.types"] = strings.Join(s.APIContentTypes, ",")
	}
	setStr(data, "server.content.security.policy", s.ContentSecurityPolicy)
	if s.HTTPCookieMaxNumber != nil {
		data["server.http.cookie.maxnumber"] = strconv.Itoa(int(*s.HTTPCookieMaxNumber))
	}
	setBoolKey(data, "server.profile.enabled", s.ProfileEnabled)
	setBoolKey(data, "server.grpc.enable.txt.service.config", s.GRPCTXTServiceConfigEnabled)
	if w := s.Webhook; w != nil {
		if w.ParallelismLimit != nil {
			data["server.webhook.parallelism.limit"] = strconv.Itoa(int(*w.ParallelismLimit))
		}
		if w.Refresh != nil && w.Refresh.Workers != nil {
			data["server.webhook.refresh.workers"] = strconv.Itoa(int(*w.Refresh.Workers))
		}
	}
	if d := s.DexServer; d != nil {
		setStr(data, "server.dex.server", d.Address)
		setBoolKeyInverted(data, "server.dex.server.plaintext", d.TLSEnabled)
		if d.InsecureSkipVerify != nil {
			data["server.dex.server.strict.tls"] = strconv.FormatBool(!*d.InsecureSkipVerify)
		}
	}
	if c := s.Cache; c != nil {
		if s := durationString(c.AppStateExpiration); s != "" {
			data["server.app.state.cache.expiration"] = s
		}
		if s := durationString(c.ConnectionStatusExpiration); s != "" {
			data["server.connection.status.cache.expiration"] = s
		}
		if s := durationString(c.DefaultExpiration); s != "" {
			data["server.default.cache.expiration"] = s
		}
		if s := durationString(c.OIDCExpiration); s != "" {
			data["server.oidc.cache.expiration"] = s
		}
		if c.GlobCacheSize != nil {
			data["server.glob.cache.size"] = strconv.Itoa(int(*c.GlobCacheSize))
		}
	}
	unmapTLSVersion(s.TLS, data, "server.tls")
	unmapK8sClient(s.K8sClient, data, "server")
}

func mapRepoServerCmdParams(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	r := ensureRepoServer(spec)
	if v, ok := kt.get("reposerver.parallelism.limit"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			r.ParallelismLimit = &i
		}
	}
	if listen := mapListen(kt, "reposerver"); listen != nil {
		r.Listen = listen
	}
	setBoolPtrInverted(kt, "reposerver.disable.tls", &r.TLSEnabled)
	setBoolPtr(kt, "reposerver.allow.oob.symlinks", &r.AllowOOBSymlinks)
	setBoolPtr(kt, "reposerver.include.hidden.directories", &r.IncludeHiddenDirectories)

	gitChanged := false
	git := &argov1alpha1.RepoServerGitConfig{}
	if setBoolPtr(kt, "reposerver.enable.git.submodule", &git.SubmoduleEnabled) {
		gitChanged = true
	}
	if v, ok := kt.get("reposerver.git.request.timeout"); ok {
		git.RequestTimeout, _ = parseDurationPtr(diag, "reposerver.git.request.timeout", v)
		gitChanged = true
	}
	if v, ok := kt.get("reposerver.git.lsremote.parallelism.limit"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			git.LSRemoteParallelismLimit = &i
			gitChanged = true
		}
	}
	if setBoolPtr(kt, "reposerver.enable.builtin.git.config", &git.BuiltinConfigEnabled) {
		gitChanged = true
	}
	if gitChanged {
		r.Git = git
	}

	cacheChanged := false
	cache := &argov1alpha1.RepoServerCacheConfig{}
	if v, ok := kt.get("reposerver.default.cache.expiration"); ok {
		cache.DefaultExpiration, _ = parseDurationPtr(diag, "reposerver.default.cache.expiration", v)
		cacheChanged = true
	}
	if v, ok := kt.get("reposerver.repo.cache.expiration"); ok {
		cache.RepoExpiration, _ = parseDurationPtr(diag, "reposerver.repo.cache.expiration", v)
		cacheChanged = true
	}
	if cacheChanged {
		r.Cache = cache
	}

	if v, ok := kt.get("reposerver.max.combined.directory.manifests.size"); ok {
		r.MaxCombinedDirectoryManifestsSize, _ = parseQuantityPtr(diag, "reposerver.max.combined.directory.manifests.size", v)
	}

	streamChanged := false
	stream := &argov1alpha1.StreamedManifestConfig{}
	if v, ok := kt.get("reposerver.streamed.manifest.max.tar.size"); ok {
		stream.MaxTarSize, _ = parseQuantityPtr(diag, "reposerver.streamed.manifest.max.tar.size", v)
		streamChanged = true
	}
	if v, ok := kt.get("reposerver.streamed.manifest.max.extracted.size"); ok {
		stream.MaxExtractedSize, _ = parseQuantityPtr(diag, "reposerver.streamed.manifest.max.extracted.size", v)
		streamChanged = true
	}
	if streamChanged {
		r.StreamedManifest = stream
	}

	pluginChanged := false
	plugin := &argov1alpha1.RepoServerPluginConfig{}
	if setBoolPtr(kt, "reposerver.plugin.use.manifest.generate.paths", &plugin.UseManifestGeneratePaths) {
		pluginChanged = true
	}
	if pluginChanged {
		r.Plugin = plugin
	}

	setBoolPtr(kt, "reposerver.profile.enabled", &r.ProfileEnabled)
	setBoolPtr(kt, "reposerver.grpc.enable.txt.service.config", &r.GRPCTXTServiceConfigEnabled)
	if v, ok := kt.get("reposerver.client.ca.path"); ok {
		r.ClientCAPath = v
	}
	if tls := mapTLSVersion(kt, diag, "reposerver.tls"); tls != nil {
		r.TLS = tls
	}

	ociChanged := false
	oci := &argov1alpha1.RepoServerOCIConfig{}
	manifestChanged := false
	manifest := &argov1alpha1.OCIManifestConfig{}
	if v, ok := kt.get("reposerver.oci.manifest.max.extracted.size"); ok {
		manifest.MaxExtractedSize, _ = parseQuantityPtr(diag, "reposerver.oci.manifest.max.extracted.size", v)
		manifestChanged = true
	}
	if setBoolPtrInverted(kt, "reposerver.disable.oci.manifest.max.extracted.size", &manifest.MaxExtractedSizeEnabled) {
		manifestChanged = true
	}
	if manifestChanged {
		oci.Manifest = manifest
		ociChanged = true
	}
	if v, ok := kt.get("reposerver.oci.layer.media.types"); ok && v != "" {
		oci.LayerMediaTypes = splitCSV(v)
		ociChanged = true
	}
	if ociChanged {
		r.OCI = oci
	}

	if v, ok := kt.get("reposerver.grpc.max.size"); ok {
		r.GRPCMaxSize, _ = parseGRPCMaxSizeMB(diag, "reposerver.grpc.max.size", v)
	}
	if v, ok := kt.get("reposerver.revision.cache.lock.timeout"); ok {
		r.RevisionCacheLockTimeout, _ = parseDurationPtr(diag, "reposerver.revision.cache.lock.timeout", v)
	}

	helmChanged := false
	helm := &argov1alpha1.HelmConfig{}
	if r.Helm != nil {
		*helm = *r.Helm
	}
	if v, ok := kt.get("reposerver.helm.user.agent"); ok {
		helm.UserAgent = v
		helmChanged = true
	}
	hmChanged := false
	hm := &argov1alpha1.HelmManifestConfig{}
	if v, ok := kt.get("reposerver.helm.manifest.max.extracted.size"); ok {
		hm.MaxExtractedSize, _ = parseQuantityPtr(diag, "reposerver.helm.manifest.max.extracted.size", v)
		hmChanged = true
	}
	if setBoolPtrInverted(kt, "reposerver.disable.helm.manifest.max.extracted.size", &hm.MaxExtractedSizeEnabled) {
		hmChanged = true
	}
	if hmChanged {
		helm.Manifest = hm
		helmChanged = true
	}
	if helmChanged {
		r.Helm = helm
	}
}

func unmapRepoServerCmdParams(r *argov1alpha1.RepoServerConfig, data map[string]string) {
	if r == nil {
		return
	}
	if r.ParallelismLimit != nil {
		data["reposerver.parallelism.limit"] = strconv.Itoa(int(*r.ParallelismLimit))
	}
	unmapListen(r.Listen, data, "reposerver")
	setBoolKeyInverted(data, "reposerver.disable.tls", r.TLSEnabled)
	setBoolKey(data, "reposerver.allow.oob.symlinks", r.AllowOOBSymlinks)
	setBoolKey(data, "reposerver.include.hidden.directories", r.IncludeHiddenDirectories)
	if g := r.Git; g != nil {
		setBoolKey(data, "reposerver.enable.git.submodule", g.SubmoduleEnabled)
		if s := durationString(g.RequestTimeout); s != "" {
			data["reposerver.git.request.timeout"] = s
		}
		if g.LSRemoteParallelismLimit != nil {
			data["reposerver.git.lsremote.parallelism.limit"] = strconv.Itoa(int(*g.LSRemoteParallelismLimit))
		}
		setBoolKey(data, "reposerver.enable.builtin.git.config", g.BuiltinConfigEnabled)
	}
	if c := r.Cache; c != nil {
		if s := durationString(c.DefaultExpiration); s != "" {
			data["reposerver.default.cache.expiration"] = s
		}
		if s := durationString(c.RepoExpiration); s != "" {
			data["reposerver.repo.cache.expiration"] = s
		}
	}
	setQuantityKey(data, "reposerver.max.combined.directory.manifests.size", r.MaxCombinedDirectoryManifestsSize)
	if sm := r.StreamedManifest; sm != nil {
		setQuantityKey(data, "reposerver.streamed.manifest.max.tar.size", sm.MaxTarSize)
		setQuantityKey(data, "reposerver.streamed.manifest.max.extracted.size", sm.MaxExtractedSize)
	}
	if p := r.Plugin; p != nil {
		setBoolKey(data, "reposerver.plugin.use.manifest.generate.paths", p.UseManifestGeneratePaths)
	}
	setBoolKey(data, "reposerver.profile.enabled", r.ProfileEnabled)
	setBoolKey(data, "reposerver.grpc.enable.txt.service.config", r.GRPCTXTServiceConfigEnabled)
	setStr(data, "reposerver.client.ca.path", r.ClientCAPath)
	unmapTLSVersion(r.TLS, data, "reposerver.tls")
	if o := r.OCI; o != nil {
		if m := o.Manifest; m != nil {
			setQuantityKey(data, "reposerver.oci.manifest.max.extracted.size", m.MaxExtractedSize)
			setBoolKeyInverted(data, "reposerver.disable.oci.manifest.max.extracted.size", m.MaxExtractedSizeEnabled)
		}
		if len(o.LayerMediaTypes) > 0 {
			data["reposerver.oci.layer.media.types"] = strings.Join(o.LayerMediaTypes, ",")
		}
	}
	setGRPCMaxSizeMBKey(data, "reposerver.grpc.max.size", r.GRPCMaxSize)
	if s := durationString(r.RevisionCacheLockTimeout); s != "" {
		data["reposerver.revision.cache.lock.timeout"] = s
	}
	if h := r.Helm; h != nil {
		setStr(data, "reposerver.helm.user.agent", h.UserAgent)
		if m := h.Manifest; m != nil {
			setQuantityKey(data, "reposerver.helm.manifest.max.extracted.size", m.MaxExtractedSize)
			setBoolKeyInverted(data, "reposerver.disable.helm.manifest.max.extracted.size", m.MaxExtractedSizeEnabled)
		}
	}
}

func mapApplicationSetCmdParams(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	ensure := func() *argov1alpha1.ApplicationSetConfig {
		if spec.ApplicationSet == nil {
			spec.ApplicationSet = &argov1alpha1.ApplicationSetConfig{}
		}
		return spec.ApplicationSet
	}
	if v, ok := kt.get("applicationsetcontroller.allowed.scm.providers"); ok && v != "" {
		ensure().AllowedSCMProviderURLs = splitCSV(v)
	}
	if v, ok := kt.get("applicationsetcontroller.global.preserved.annotations"); ok && v != "" {
		gp := ensure().GlobalPreserved
		if gp == nil {
			gp = &argov1alpha1.GlobalPreservedKeysConfig{}
			ensure().GlobalPreserved = gp
		}
		gp.AnnotationKeys = splitCSV(v)
	}
	if v, ok := kt.get("applicationsetcontroller.global.preserved.labels"); ok && v != "" {
		gp := ensure().GlobalPreserved
		if gp == nil {
			gp = &argov1alpha1.GlobalPreservedKeysConfig{}
			ensure().GlobalPreserved = gp
		}
		gp.LabelKeys = splitCSV(v)
	}
	boolKeys := []string{
		"applicationsetcontroller.enable.scm.providers",
		"applicationsetcontroller.enable.policy.override",
		"applicationsetcontroller.enable.progressive.syncs",
		"applicationsetcontroller.enable.git.submodule",
		"applicationsetcontroller.enable.new.git.file.globbing",
		"applicationsetcontroller.enable.tokenref.strict.mode",
		"applicationsetcontroller.enable.github.api.metrics",
		"applicationsetcontroller.dryrun",
		"applicationsetcontroller.enable.leader.election",
		"applicationsetcontroller.profile.enabled",
		"applicationsetcontroller.grpc.enable.txt.service.config",
	}
	for _, k := range boolKeys {
		if _, ok := kt.get(k); ok {
			a := ensure()
			switch k {
			case "applicationsetcontroller.enable.scm.providers":
				setBoolPtr(kt, k, &a.SCMProvidersEnabled)
			case "applicationsetcontroller.enable.policy.override":
				setBoolPtr(kt, k, &a.PolicyOverrideEnabled)
			case "applicationsetcontroller.enable.progressive.syncs":
				if a.ProgressiveSyncs == nil {
					a.ProgressiveSyncs = &argov1alpha1.ProgressiveSyncsConfig{}
				}
				setBoolPtr(kt, k, &a.ProgressiveSyncs.Enabled)
			case "applicationsetcontroller.enable.git.submodule":
				setBoolPtr(kt, k, &a.GitSubmoduleEnabled)
			case "applicationsetcontroller.enable.new.git.file.globbing":
				setBoolPtr(kt, k, &a.NewGitFileGlobbingEnabled)
			case "applicationsetcontroller.enable.tokenref.strict.mode":
				setBoolPtr(kt, k, &a.TokenRefStrictModeEnabled)
			case "applicationsetcontroller.enable.github.api.metrics":
				setBoolPtr(kt, k, &a.GitHubAPIMetricsEnabled)
			case "applicationsetcontroller.dryrun":
				setBoolPtr(kt, k, &a.DryRun)
			case "applicationsetcontroller.enable.leader.election":
				setBoolPtr(kt, k, &a.LeaderElectionEnabled)
			case "applicationsetcontroller.profile.enabled":
				setBoolPtr(kt, k, &a.ProfileEnabled)
			case "applicationsetcontroller.grpc.enable.txt.service.config":
				setBoolPtr(kt, k, &a.GRPCTXTServiceConfigEnabled)
			}
		}
	}
	if v, ok := kt.get("applicationsetcontroller.concurrent.reconciliations.max"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			ensure().ReconciliationsParallelismLimit = &i
		}
	}
	if v, ok := kt.get("applicationsetcontroller.requeue.after"); ok {
		ensure().RequeueAfter, _ = parseDurationPtr(diag, "applicationsetcontroller.requeue.after", v)
	}
	if v, ok := kt.get("applicationsetcontroller.webhook.parallelism.limit"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			ensure().WebhookParallelismLimit = &i
		}
	}
	if v, ok := kt.get("applicationsetcontroller.status.max.resources.count"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			ensure().StatusMaxResourcesCount = &i
		}
	}
	if v, ok := kt.get("applicationsetcontroller.scm.root.ca.path"); ok {
		ensure().SCMRootCAPath = v
	}
	if v, ok := kt.get("applicationsetcontroller.log.format"); ok {
		ensureLog(&ensure().Log).Format = v
	}
	if v, ok := kt.get("applicationsetcontroller.log.level"); ok {
		ensureLog(&ensure().Log).Level = v
	}
	// Legacy applicationsetcontroller.debug forces log level to debug (takes precedence over log.level).
	if v, ok := kt.get("applicationsetcontroller.debug"); ok {
		if strings.EqualFold(v, "true") {
			a := ensure()
			prev := ""
			if a.Log != nil {
				prev = a.Log.Level
			}
			ensureLog(&a.Log).Level = "debug"
			if diag != nil {
				if prev != "" && prev != "debug" {
					diag.Warn(DirCMToCR, "applicationsetcontroller.debug",
						fmt.Sprintf("collapsed to log.level=debug (overrides applicationsetcontroller.log.level=%q)", prev))
				} else {
					diag.Info(DirCMToCR, "applicationsetcontroller.debug",
						"collapsed to log.level=debug (legacy alias)")
				}
			}
		}
	}
	if kc := mapK8sClient(kt, diag, "applicationsetcontroller"); kc != nil {
		ensure().K8sClient = kc
	}
	if rs := mapMTLSCerts(kt, "applicationsetcontroller"); rs != nil {
		ensure().RepoServer = rs
	}
}

func unmapApplicationSetCmdParams(a *argov1alpha1.ApplicationSetConfig, data map[string]string) {
	if a == nil {
		return
	}
	if len(a.AllowedSCMProviderURLs) > 0 {
		data["applicationsetcontroller.allowed.scm.providers"] = strings.Join(a.AllowedSCMProviderURLs, ",")
	}
	if gp := a.GlobalPreserved; gp != nil {
		if len(gp.AnnotationKeys) > 0 {
			data["applicationsetcontroller.global.preserved.annotations"] = strings.Join(gp.AnnotationKeys, ",")
		}
		if len(gp.LabelKeys) > 0 {
			data["applicationsetcontroller.global.preserved.labels"] = strings.Join(gp.LabelKeys, ",")
		}
	}
	setBoolKey(data, "applicationsetcontroller.enable.scm.providers", a.SCMProvidersEnabled)
	setBoolKey(data, "applicationsetcontroller.enable.policy.override", a.PolicyOverrideEnabled)
	if ps := a.ProgressiveSyncs; ps != nil {
		setBoolKey(data, "applicationsetcontroller.enable.progressive.syncs", ps.Enabled)
	}
	setBoolKey(data, "applicationsetcontroller.enable.git.submodule", a.GitSubmoduleEnabled)
	setBoolKey(data, "applicationsetcontroller.enable.new.git.file.globbing", a.NewGitFileGlobbingEnabled)
	setBoolKey(data, "applicationsetcontroller.enable.tokenref.strict.mode", a.TokenRefStrictModeEnabled)
	setBoolKey(data, "applicationsetcontroller.enable.github.api.metrics", a.GitHubAPIMetricsEnabled)
	setBoolKey(data, "applicationsetcontroller.dryrun", a.DryRun)
	setBoolKey(data, "applicationsetcontroller.enable.leader.election", a.LeaderElectionEnabled)
	if a.ReconciliationsParallelismLimit != nil {
		data["applicationsetcontroller.concurrent.reconciliations.max"] = strconv.Itoa(int(*a.ReconciliationsParallelismLimit))
	}
	if s := durationString(a.RequeueAfter); s != "" {
		data["applicationsetcontroller.requeue.after"] = s
	}
	if a.WebhookParallelismLimit != nil {
		data["applicationsetcontroller.webhook.parallelism.limit"] = strconv.Itoa(int(*a.WebhookParallelismLimit))
	}
	if a.StatusMaxResourcesCount != nil {
		data["applicationsetcontroller.status.max.resources.count"] = strconv.Itoa(int(*a.StatusMaxResourcesCount))
	}
	setStr(data, "applicationsetcontroller.scm.root.ca.path", a.SCMRootCAPath)
	if l := a.Log; l != nil {
		setStr(data, "applicationsetcontroller.log.format", l.Format)
		setStr(data, "applicationsetcontroller.log.level", l.Level)
	}
	setBoolKey(data, "applicationsetcontroller.profile.enabled", a.ProfileEnabled)
	setBoolKey(data, "applicationsetcontroller.grpc.enable.txt.service.config", a.GRPCTXTServiceConfigEnabled)
	unmapK8sClient(a.K8sClient, data, "applicationsetcontroller")
	unmapMTLSCerts(a.RepoServer, data, "applicationsetcontroller")
}

func mapControllerCmdParamsExtras(kt *keyTracker, diag *Diagnostics, c *argov1alpha1.ControllerConfig) {
	setBoolPtr(kt, "controller.profile.enabled", &c.ProfileEnabled)
	setBoolPtr(kt, "controller.grpc.enable.txt.service.config", &c.GRPCTXTServiceConfigEnabled)
	if v, ok := kt.get("controller.repo.error.grace.period.seconds"); ok {
		c.RepoErrorGracePeriod, _ = secondsDurationPtr(diag, "controller.repo.error.grace.period.seconds", v)
	}
	ccChanged := false
	cc := &argov1alpha1.ControllerClusterCacheConfig{}
	if setBoolPtr(kt, "controller.cluster.cache.batch.events.processing", &cc.BatchEventsProcessing) {
		ccChanged = true
	}
	if v, ok := kt.get("controller.cluster.cache.events.processing.interval"); ok {
		cc.EventsProcessingInterval, _ = parseDurationPtr(diag, "controller.cluster.cache.events.processing.interval", v)
		ccChanged = true
	}
	if ccChanged {
		c.ClusterCache = cc
	}
	if kc := mapK8sClient(kt, diag, "controller"); kc != nil {
		c.K8sClient = kc
	}
}

func unmapControllerCmdParamsExtras(c *argov1alpha1.ControllerConfig, data map[string]string) {
	if c == nil {
		return
	}
	setBoolKey(data, "controller.profile.enabled", c.ProfileEnabled)
	setBoolKey(data, "controller.grpc.enable.txt.service.config", c.GRPCTXTServiceConfigEnabled)
	if c.RepoErrorGracePeriod != nil {
		data["controller.repo.error.grace.period.seconds"] = strconv.Itoa(int(c.RepoErrorGracePeriod.Duration.Seconds()))
	}
	if cc := c.ClusterCache; cc != nil {
		setBoolKey(data, "controller.cluster.cache.batch.events.processing", cc.BatchEventsProcessing)
		if s := durationString(cc.EventsProcessingInterval); s != "" {
			data["controller.cluster.cache.events.processing.interval"] = s
		}
	}
	unmapK8sClient(c.K8sClient, data, "controller")
}

func mapCommitServerCmdParams(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	cs := ensureCommitServer(spec)
	if listen := mapListen(kt, "commitserver"); listen != nil {
		cs.Listen = listen
	}
	setBoolPtr(kt, "commitserver.grpc.enable.txt.service.config", &cs.GRPCTXTServiceConfigEnabled)
}

func unmapCommitServerCmdParams(cs *argov1alpha1.CommitServerConfig, data map[string]string) {
	if cs == nil {
		return
	}
	unmapListen(cs.Listen, data, "commitserver")
	setBoolKey(data, "commitserver.grpc.enable.txt.service.config", cs.GRPCTXTServiceConfigEnabled)
}

func mapDexServerCmdParams(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	changed := false
	d := &argov1alpha1.DexServerConfig{}
	if v, ok := kt.get("dexserver.log.format"); ok {
		ensureLog(&d.Log).Format = v
		changed = true
	}
	if v, ok := kt.get("dexserver.log.level"); ok {
		ensureLog(&d.Log).Level = v
		changed = true
	}
	if setBoolPtrInverted(kt, "dexserver.disable.tls", &d.TLSEnabled) {
		changed = true
	}
	if setBoolPtr(kt, "dexserver.connector.failure.continue", &d.ConnectorFailureContinue) {
		changed = true
	}
	if changed {
		spec.DexServer = d
	}
}

func unmapDexServerCmdParams(d *argov1alpha1.DexServerConfig, data map[string]string) {
	if d == nil {
		return
	}
	if l := d.Log; l != nil {
		setStr(data, "dexserver.log.format", l.Format)
		setStr(data, "dexserver.log.level", l.Level)
	}
	setBoolKeyInverted(data, "dexserver.disable.tls", d.TLSEnabled)
	setBoolKey(data, "dexserver.connector.failure.continue", d.ConnectorFailureContinue)
}

func mapNotificationsCmdParams(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	changed := false
	n := &argov1alpha1.NotificationsConfig{}
	if v, ok := kt.get("notificationscontroller.log.format"); ok {
		ensureLog(&n.Log).Format = v
		changed = true
	}
	if v, ok := kt.get("notificationscontroller.log.level"); ok {
		ensureLog(&n.Log).Level = v
		changed = true
	}
	if v, ok := kt.get("notificationscontroller.processors.count"); ok {
		if i, err := strconv.Atoi(v); err == nil {
			v32 := int32(i)
			n.ProcessorsCount = &v32
			changed = true
		}
	}
	if setBoolPtr(kt, "notificationscontroller.selfservice.enabled", &n.SelfServiceEnabled) {
		changed = true
	}
	if setBoolPtrInverted(kt, "notificationscontroller.repo.server.plaintext", &n.TLSEnabled) {
		changed = true
	}
	if rs := mapMTLSCerts(kt, "notificationscontroller"); rs != nil {
		n.RepoServer = rs
		changed = true
	}
	if changed {
		spec.Notifications = n
	}
}

func unmapNotificationsCmdParams(n *argov1alpha1.NotificationsConfig, data map[string]string) {
	if n == nil {
		return
	}
	if l := n.Log; l != nil {
		setStr(data, "notificationscontroller.log.format", l.Format)
		setStr(data, "notificationscontroller.log.level", l.Level)
	}
	if n.ProcessorsCount != nil {
		data["notificationscontroller.processors.count"] = strconv.Itoa(int(*n.ProcessorsCount))
	}
	setBoolKey(data, "notificationscontroller.selfservice.enabled", n.SelfServiceEnabled)
	setBoolKeyInverted(data, "notificationscontroller.repo.server.plaintext", n.TLSEnabled)
	unmapMTLSCerts(n.RepoServer, data, "notificationscontroller")
}

// --- shared helpers ---

func mapListen(kt *keyTracker, prefix string) *argov1alpha1.ListenConfig {
	listen := &argov1alpha1.ListenConfig{}
	changed := false
	if v, ok := kt.get(prefix + ".listen.address"); ok {
		listen.Address = v
		changed = true
	}
	if v, ok := kt.get(prefix + ".metrics.listen.address"); ok {
		listen.MetricsAddress = v
		changed = true
	}
	if !changed {
		return nil
	}
	return listen
}

func unmapListen(listen *argov1alpha1.ListenConfig, data map[string]string, prefix string) {
	if listen == nil {
		return
	}
	setStr(data, prefix+".listen.address", listen.Address)
	setStr(data, prefix+".metrics.listen.address", listen.MetricsAddress)
}

func mapServerListen(kt *keyTracker) *argov1alpha1.ServerListenConfig {
	listen := &argov1alpha1.ServerListenConfig{}
	changed := false
	if v, ok := kt.get("server.listen.address"); ok {
		listen.Address = v
		changed = true
	}
	if v, ok := kt.get("server.metrics.listen.address"); ok {
		listen.MetricsAddress = v
		changed = true
	}
	if !changed {
		return nil
	}
	return listen
}

func unmapServerListen(listen *argov1alpha1.ServerListenConfig, data map[string]string) {
	if listen == nil {
		return
	}
	setStr(data, "server.listen.address", listen.Address)
	setStr(data, "server.metrics.listen.address", listen.MetricsAddress)
}

func mapTLSVersion(kt *keyTracker, diag *Diagnostics, prefix string) *argov1alpha1.TLSVersionConfig {
	tls := &argov1alpha1.TLSVersionConfig{}
	changed := false
	if v, ok := kt.get(prefix + ".minversion"); ok {
		tls.MinVersion = v
		changed = true
	}
	if v, ok := kt.get(prefix + ".maxversion"); ok {
		tls.MaxVersion = v
		changed = true
	}
	if v, ok := kt.get(prefix + ".ciphers"); ok && v != "" {
		tls.Ciphers = splitCSV(v)
		changed = true
	}
	if !changed {
		return nil
	}
	return tls
}

func unmapTLSVersion(tls *argov1alpha1.TLSVersionConfig, data map[string]string, prefix string) {
	if tls == nil {
		return
	}
	setStr(data, prefix+".minversion", tls.MinVersion)
	setStr(data, prefix+".maxversion", tls.MaxVersion)
	if len(tls.Ciphers) > 0 {
		data[prefix+".ciphers"] = strings.Join(tls.Ciphers, ",")
	}
}

func mapMTLSCerts(kt *keyTracker, component string) *argov1alpha1.MTLSCertConfig {
	rs := &argov1alpha1.MTLSCertConfig{}
	changed := false
	if v, ok := kt.get(component + ".repo.server.ca.cert.path"); ok {
		rs.CACertPath = v
		changed = true
	}
	if v, ok := kt.get(component + ".repo.server.client.cert.path"); ok {
		rs.ClientCertPath = v
		changed = true
	}
	if v, ok := kt.get(component + ".repo.server.client.cert.key.path"); ok {
		rs.ClientCertKeyPath = v
		changed = true
	}
	if !changed {
		return nil
	}
	return rs
}

func unmapMTLSCerts(mtls *argov1alpha1.MTLSCertConfig, data map[string]string, component string) {
	if mtls == nil {
		return
	}
	setStr(data, component+".repo.server.ca.cert.path", mtls.CACertPath)
	setStr(data, component+".repo.server.client.cert.path", mtls.ClientCertPath)
	setStr(data, component+".repo.server.client.cert.key.path", mtls.ClientCertKeyPath)
}

func mapK8sClient(kt *keyTracker, diag *Diagnostics, component string) *argov1alpha1.K8sClientConfig {
	kc := &argov1alpha1.K8sClientConfig{}
	changed := false
	if v, ok := kt.get(component + ".k8s.client.qps"); ok {
		kc.QPS = v
		changed = true
	}
	if v, ok := kt.get(component + ".k8s.client.burst"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			kc.Burst = &i
			changed = true
		}
	}
	if v, ok := kt.get(component + ".k8s.client.max.idle.connections"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			kc.MaxIdleConnections = &i
			changed = true
		}
	}
	tcpChanged := false
	tcp := &argov1alpha1.K8sClientTCPConfig{}
	if v, ok := kt.get(component + ".k8s.tcp.timeout"); ok {
		tcp.Timeout, _ = parseDurationPtr(diag, component+".k8s.tcp.timeout", v)
		tcpChanged = true
	}
	if v, ok := kt.get(component + ".k8s.tcp.keepalive"); ok {
		tcp.KeepAlive, _ = parseDurationPtr(diag, component+".k8s.tcp.keepalive", v)
		tcpChanged = true
	}
	if v, ok := kt.get(component + ".k8s.tcp.idle.timeout"); ok {
		tcp.IdleTimeout, _ = parseDurationPtr(diag, component+".k8s.tcp.idle.timeout", v)
		tcpChanged = true
	}
	if tcpChanged {
		kc.TCP = tcp
		changed = true
	}
	if v, ok := kt.get(component + ".k8s.tls.handshake.timeout"); ok {
		kc.TLSHandshakeTimeout, _ = parseDurationPtr(diag, component+".k8s.tls.handshake.timeout", v)
		changed = true
	}
	if v, ok := kt.get(component + ".k8sclient.retry.max"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			if kc.Retry == nil {
				kc.Retry = &argov1alpha1.ClientRetryConfig{}
			}
			kc.Retry.Max = &i
			changed = true
		}
	}
	if v, ok := kt.get(component + ".k8sclient.retry.base.backoff"); ok {
		if kc.Retry == nil {
			kc.Retry = &argov1alpha1.ClientRetryConfig{}
		}
		// Legacy cmd-params store this as integer milliseconds (e.g. "100"),
		// but Go duration strings (e.g. "100ms") are also accepted.
		var d *metav1.Duration
		if n, err := strconv.Atoi(v); err == nil {
			d = &metav1.Duration{Duration: time.Duration(n) * time.Millisecond}
		} else {
			d, _ = parseDurationPtr(diag, component+".k8sclient.retry.base.backoff", v)
		}
		if d != nil {
			if kc.Retry.Backoff == nil {
				kc.Retry.Backoff = &argov1alpha1.BackoffConfig{}
			}
			kc.Retry.Backoff.Duration = d
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return kc
}

func unmapK8sClient(kc *argov1alpha1.K8sClientConfig, data map[string]string, component string) {
	if kc == nil {
		return
	}
	setStr(data, component+".k8s.client.qps", kc.QPS)
	if kc.Burst != nil {
		data[component+".k8s.client.burst"] = strconv.Itoa(int(*kc.Burst))
	}
	if kc.MaxIdleConnections != nil {
		data[component+".k8s.client.max.idle.connections"] = strconv.Itoa(int(*kc.MaxIdleConnections))
	}
	if tcp := kc.TCP; tcp != nil {
		if s := durationString(tcp.Timeout); s != "" {
			data[component+".k8s.tcp.timeout"] = s
		}
		if s := durationString(tcp.KeepAlive); s != "" {
			data[component+".k8s.tcp.keepalive"] = s
		}
		if s := durationString(tcp.IdleTimeout); s != "" {
			data[component+".k8s.tcp.idle.timeout"] = s
		}
	}
	if s := durationString(kc.TLSHandshakeTimeout); s != "" {
		data[component+".k8s.tls.handshake.timeout"] = s
	}
	if r := kc.Retry; r != nil {
		if r.Max != nil {
			data[component+".k8sclient.retry.max"] = strconv.Itoa(int(*r.Max))
		}
		if r.Backoff != nil && r.Backoff.Duration != nil {
			// Emit integer milliseconds to match argocd-cmd-params-cm sample form.
			ms := r.Backoff.Duration.Duration / time.Millisecond
			data[component+".k8sclient.retry.base.backoff"] = strconv.FormatInt(int64(ms), 10)
		}
	}
}

func setBoolPtr(kt *keyTracker, key string, dest **bool) bool {
	v, ok := kt.get(key)
	if !ok {
		return false
	}
	b := strings.EqualFold(v, "true")
	*dest = &b
	return true
}

// setBoolPtrInverted maps a legacy key whose true means the opposite of the CRD field
// (e.g. disable.tls=true → tlsEnabled=false, plaintext=true → tlsEnabled=false).
func setBoolPtrInverted(kt *keyTracker, key string, dest **bool) bool {
	v, ok := kt.get(key)
	if !ok {
		return false
	}
	b := !strings.EqualFold(v, "true")
	*dest = &b
	return true
}

func setBoolKey(data map[string]string, key string, v *bool) {
	if v != nil {
		data[key] = strconv.FormatBool(*v)
	}
}

func setBoolKeyInverted(data map[string]string, key string, v *bool) {
	if v != nil {
		data[key] = strconv.FormatBool(!*v)
	}
}

func setStr(data map[string]string, key, v string) {
	if v != "" {
		data[key] = v
	}
}

func parseQuantityPtr(diag *Diagnostics, key, s string) (*resource.Quantity, error) {
	if s == "" {
		return nil, nil
	}
	q, err := resource.ParseQuantity(s)
	if err != nil {
		if diag != nil && key != "" {
			diag.Error(DirCMToCR, key, fmt.Sprintf("invalid quantity %q: %v", s, err))
		}
		return nil, err
	}
	return &q, nil
}

func setQuantityKey(data map[string]string, key string, q *resource.Quantity) {
	if q != nil {
		data[key] = q.String()
	}
}

// parseGRPCMaxSizeMB parses reposerver.grpc.max.size, which is historically an
// integer megabyte count (ARGOCD_GRPC_MAX_SIZE_MB). Bare integers are treated as
// binary megabytes; other strings go through resource.ParseQuantity.
func parseGRPCMaxSizeMB(diag *Diagnostics, key, s string) (*resource.Quantity, error) {
	if s == "" {
		return nil, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		q := resource.MustParse(fmt.Sprintf("%dMi", n))
		return &q, nil
	}
	return parseQuantityPtr(diag, key, s)
}

func setGRPCMaxSizeMBKey(data map[string]string, key string, q *resource.Quantity) {
	if q == nil {
		return
	}
	mb := int(q.Value() / (1024 * 1024))
	if mb < 1 && q.Value() > 0 {
		mb = 1
	}
	if mb > 0 {
		data[key] = strconv.Itoa(mb)
	}
}
