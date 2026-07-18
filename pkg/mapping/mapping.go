// Package mapping converts between Argo CD ConfigMaps and ArgoCDConfiguration.
//
// The mapping table is intentional structured data so it can later inform the
// in-tree config registry (Phase 0+) without being rewritten from scratch.
package mapping

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
)

const (
	ArgoCDCMName          = "argocd-cm"
	ArgoCDCmdParamsCMName = "argocd-cmd-params-cm"
	ArgoCDRBACCMName      = "argocd-rbac-cm"
)

// ConfigMaps is the set of ConfigMaps the CLI reads/writes.
type ConfigMaps struct {
	CM        *corev1.ConfigMap // argocd-cm
	CmdParams *corev1.ConfigMap // argocd-cmd-params-cm (optional)
	RBAC      *corev1.ConfigMap // argocd-rbac-cm (optional)
}

// DefaultConfigurationName is the only allowed metadata.name for ArgoCDConfiguration
// (enforced by a root CEL rule on the CRD).
const DefaultConfigurationName = "argocd-config"

// FromConfigMaps builds an ArgoCDConfiguration from ConfigMap data.
// Diagnostics collect unknown keys, parse failures, and lossy transforms.
func FromConfigMaps(cms ConfigMaps, name, namespace string) (*argov1alpha1.ArgoCDConfiguration, *Diagnostics, error) {
	diag := &Diagnostics{}
	if name == "" {
		name = DefaultConfigurationName
	}
	out := &argov1alpha1.ArgoCDConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: argov1alpha1.GroupVersion.String(),
			Kind:       "ArgoCDConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if cms.CM != nil {
		kt := newKeyTracker(cms.CM.Data, diag, DirCMToCR, ArgoCDCMName)
		if err := mapCM(kt, &out.Spec, diag); err != nil {
			return nil, diag, fmt.Errorf("argocd-cm: %w", err)
		}
		kt.reportUnknown()
	}
	if cms.CmdParams != nil {
		kt := newKeyTracker(cms.CmdParams.Data, diag, DirCMToCR, ArgoCDCmdParamsCMName)
		if err := mapCmdParams(kt, &out.Spec, diag); err != nil {
			return nil, diag, fmt.Errorf("argocd-cmd-params-cm: %w", err)
		}
		kt.reportUnknown()
	}
	if cms.RBAC != nil {
		kt := newKeyTracker(cms.RBAC.Data, diag, DirCMToCR, ArgoCDRBACCMName)
		if err := mapRBAC(kt, &out.Spec, diag); err != nil {
			return nil, diag, fmt.Errorf("argocd-rbac-cm: %w", err)
		}
		kt.reportUnknown()
	}
	return out, diag, nil
}

// ToConfigMaps converts an ArgoCDConfiguration into ConfigMaps.
func ToConfigMaps(cfg *argov1alpha1.ArgoCDConfiguration, namespace string) (ConfigMaps, *Diagnostics, error) {
	return ToConfigMapsWithSource(cfg, namespace, ConfigMaps{})
}

// ToConfigMapsWithSource is like ToConfigMaps but preserves labels/annotations from source.
func ToConfigMapsWithSource(cfg *argov1alpha1.ArgoCDConfiguration, namespace string, source ConfigMaps) (ConfigMaps, *Diagnostics, error) {
	diag := &Diagnostics{}
	cm := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: ArgoCDCMName, Namespace: namespace},
		Data:       map[string]string{},
	}
	cmd := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: ArgoCDCmdParamsCMName, Namespace: namespace},
		Data:       map[string]string{},
	}
	rbac := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: ArgoCDRBACCMName, Namespace: namespace},
		Data:       map[string]string{},
	}
	preserveMeta(cm, source.CM, diag)
	preserveMeta(cmd, source.CmdParams, diag)
	preserveMeta(rbac, source.RBAC, diag)

	if err := unmapCM(&cfg.Spec, cm.Data, diag); err != nil {
		return ConfigMaps{}, diag, err
	}
	if err := unmapCmdParams(&cfg.Spec, cmd.Data, diag); err != nil {
		return ConfigMaps{}, diag, err
	}
	unmapRBAC(&cfg.Spec, rbac.Data)
	return ConfigMaps{CM: cm, CmdParams: cmd, RBAC: rbac}, diag, nil
}

func preserveMeta(dst, src *corev1.ConfigMap, diag *Diagnostics) {
	if src == nil {
		if diag != nil {
			diag.Warn(DirCRToCM, dst.Name, "no source ConfigMap provided; labels/annotations cannot be preserved")
		}
		return
	}
	if len(src.Labels) > 0 {
		dst.Labels = make(map[string]string, len(src.Labels))
		for k, v := range src.Labels {
			dst.Labels[k] = v
		}
	}
	if len(src.Annotations) > 0 {
		dst.Annotations = make(map[string]string, len(src.Annotations))
		for k, v := range src.Annotations {
			dst.Annotations[k] = v
		}
	}
}

func ensureServer(spec *argov1alpha1.ArgoCDConfigurationSpec) *argov1alpha1.ServerConfig {
	if spec.Server == nil {
		spec.Server = &argov1alpha1.ServerConfig{}
	}
	return spec.Server
}

func ensureController(spec *argov1alpha1.ArgoCDConfigurationSpec) *argov1alpha1.ControllerConfig {
	if spec.Controller == nil {
		spec.Controller = &argov1alpha1.ControllerConfig{}
	}
	return spec.Controller
}

func ensureRepoServer(spec *argov1alpha1.ArgoCDConfigurationSpec) *argov1alpha1.RepoServerConfig {
	if spec.RepoServer == nil {
		spec.RepoServer = &argov1alpha1.RepoServerConfig{}
	}
	return spec.RepoServer
}

func ensureCommitServer(spec *argov1alpha1.ArgoCDConfigurationSpec) *argov1alpha1.CommitServerConfig {
	if spec.CommitServer == nil {
		spec.CommitServer = &argov1alpha1.CommitServerConfig{}
	}
	return spec.CommitServer
}

func ensureRedis(spec *argov1alpha1.ArgoCDConfigurationSpec) *argov1alpha1.RedisConfig {
	if spec.Redis == nil {
		spec.Redis = &argov1alpha1.RedisConfig{}
	}
	return spec.Redis
}

func ensureOTLP(spec *argov1alpha1.ArgoCDConfigurationSpec) *argov1alpha1.OTLPConfig {
	if spec.OTLP == nil {
		spec.OTLP = &argov1alpha1.OTLPConfig{}
	}
	return spec.OTLP
}

func ensureLogging(spec *argov1alpha1.ArgoCDConfigurationSpec) *argov1alpha1.LoggingConfig {
	if spec.Logging == nil {
		spec.Logging = &argov1alpha1.LoggingConfig{}
	}
	return spec.Logging
}

func ensureRepoServerClient(spec *argov1alpha1.ArgoCDConfigurationSpec) *argov1alpha1.RepoServerClientConfig {
	rs := ensureRepoServer(spec)
	if rs.Client == nil {
		rs.Client = &argov1alpha1.RepoServerClientConfig{}
	}
	return rs.Client
}
func mapCM(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) error {
	if v, ok := kt.get("url"); ok {
		ensureServer(spec).URL = v
	}
	if v, ok := kt.get("additionalUrls"); ok && v != "" {
		var urls []string
		if err := yaml.Unmarshal([]byte(v), &urls); err != nil {
			return fmt.Errorf("additionalUrls: %w", err)
		}
		ensureServer(spec).AdditionalURLs = urls
	}
	if v, ok := kt.get("oidc.tls.insecure.skip.verify"); ok {
		b := strings.EqualFold(v, "true")
		ensureServer(spec).OIDCInsecureSkipVerify = &b
	}
	if v, ok := kt.get("dex.config"); ok && v != "" {
		dex, err := parseDexConfig(v)
		if err != nil {
			return fmt.Errorf("dex.config: %w", err)
		}
		ensureServer(spec).Dex = dex
	}
	if v, ok := kt.get("oidc.config"); ok && v != "" {
		oidc, err := parseOIDCConfig(v, diag)
		if err != nil {
			return fmt.Errorf("oidc.config: %w", err)
		}
		ensureServer(spec).OIDC = oidc
	}
	if err := mapResource(kt, spec, diag); err != nil {
		return err
	}
	mapApplication(kt, spec, diag)
	mapUI(kt, spec, diag)
	mapUsers(kt, spec, diag)
	mapHelp(kt, spec, diag)
	mapAccounts(kt, spec, diag)
	if err := mapExtensions(kt, spec, diag); err != nil {
		return err
	}
	if err := mapGlobalProjects(kt, spec, diag); err != nil {
		return err
	}
	if err := mapDeepLinks(kt, spec, diag); err != nil {
		return err
	}
	mapKustomize(kt, spec, diag)
	mapHelm(kt, spec, diag)
	mapMisc(kt, spec, diag)
	return nil
}
func parseDexConfig(raw string) (*argov1alpha1.DexConfig, error) {
	var m map[string]any
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	dex := &argov1alpha1.DexConfig{}
	if connectors, ok := m["connectors"].([]any); ok {
		for _, c := range connectors {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			conn := argov1alpha1.DexConnector{
				Type: asString(cm["type"]),
				ID:   asString(cm["id"]),
				Name: asString(cm["name"]),
			}
			if cfg, ok := cm["config"]; ok {
				b, err := json.Marshal(cfg)
				if err != nil {
					return nil, err
				}
				conn.Config = runtime.RawExtension{Raw: b}
			}
			dex.Connectors = append(dex.Connectors, conn)
		}
		delete(m, "connectors")
	}
	if sc, ok := m["staticClients"]; ok {
		if list, ok := sc.([]any); ok {
			for _, item := range list {
				b, err := json.Marshal(item)
				if err != nil {
					return nil, err
				}
				dex.StaticClients = append(dex.StaticClients, runtime.RawExtension{Raw: b})
			}
		}
		delete(m, "staticClients")
	}
	if len(m) > 0 {
		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		dex.Extra = &runtime.RawExtension{Raw: b}
	}
	return dex, nil
}

func parseOIDCConfig(raw string, diag *Diagnostics) (*argov1alpha1.OIDCConfig, error) {
	var m map[string]any
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	out := &argov1alpha1.OIDCConfig{}
	out.Name = strFromMap(m, "name")
	out.IssuerURL = strFromMap(m, "issuer")
	out.ClientID = strFromMap(m, "clientID")
	if cs := strFromMap(m, "clientSecret"); cs != "" {
		ref, err := dollarSecretToRef(cs)
		if err != nil {
			return nil, fmt.Errorf("clientSecret: %w", err)
		}
		out.ClientSecretRef = ref
	}
	out.CLIClientID = strFromMap(m, "cliClientID")
	if ui, ok := m["userInfo"].(map[string]any); ok {
		out.UserInfo = parseOIDCUserInfo(ui, diag)
	} else {
		// Legacy flat keys in oidc.config
		userInfo := &argov1alpha1.OIDCUserInfoConfig{}
		uiChanged := false
		if v := strFromMap(m, "userInfoBaseURL"); v != "" {
			userInfo.BaseURL = v
			uiChanged = true
		}
		if v := strFromMap(m, "userInfoPath"); v != "" {
			userInfo.Path = v
			uiChanged = true
		}
		if d, _ := parseDurationPtr(diag, "oidc.config/userInfoCacheExpiration", strFromMap(m, "userInfoCacheExpiration")); d != nil {
			userInfo.CacheExpiration = d
			uiChanged = true
		}
		if v, ok := m["enableUserInfoGroups"].(bool); ok && v {
			userInfo.GroupsEnabled = v
			uiChanged = true
		}
		if uiChanged {
			out.UserInfo = userInfo
		}
	}
	out.LogoutURL = strFromMap(m, "logoutURL")
	out.RootCA = strFromMap(m, "rootCA")
	out.DomainHint = strFromMap(m, "domainHint")
	out.RefreshTokenThreshold, _ = parseDurationPtr(diag, "oidc.config/refreshTokenThreshold", strFromMap(m, "refreshTokenThreshold"))
	if v, ok := m["enablePKCEAuthentication"].(bool); ok {
		out.PKCEAuthenticationEnabled = v
	}
	if v, ok := m["skipAudienceCheckWhenTokenHasNoAudience"].(bool); ok {
		out.SkipAudienceCheckWhenTokenHasNoAudience = v
	}
	if scopes, ok := m["requestedScopes"].([]any); ok {
		for _, s := range scopes {
			out.RequestedScopes = append(out.RequestedScopes, asString(s))
		}
	}
	if aud, ok := m["allowedAudiences"].([]any); ok {
		for _, s := range aud {
			out.AllowedAudiences = append(out.AllowedAudiences, asString(s))
		}
	}
	if claims, ok := m["requestedIDTokenClaims"].(map[string]any); ok {
		out.RequestedIDTokenClaims = map[string]argov1alpha1.OIDCClaim{}
		for k, v := range claims {
			cm, _ := v.(map[string]any)
			claim := argov1alpha1.OIDCClaim{}
			if e, ok := cm["essential"].(bool); ok {
				claim.Essential = e
			}
			if val, ok := cm["value"].(string); ok {
				claim.Value = val
			}
			if vals, ok := cm["values"].([]any); ok {
				for _, x := range vals {
					claim.Values = append(claim.Values, asString(x))
				}
			}
			out.RequestedIDTokenClaims[k] = claim
		}
	}
	if az, ok := m["azure"].(map[string]any); ok {
		a := &argov1alpha1.AzureOIDCConfig{}
		if v, ok := az["useWorkloadIdentity"].(bool); ok {
			a.UseWorkloadIdentity = v
		}
		a.GraphAPIEndpointURL = strFromMap(az, "graphApiEndpoint")
		overageChanged := false
		overage := &argov1alpha1.AzureUserGroupOverageClaimConfig{}
		if ug, ok := az["userGroupOverageClaim"].(map[string]any); ok {
			if v, ok := ug["enabled"].(bool); ok && v {
				overage.Enabled = v
				overageChanged = true
			}
			if d, _ := parseDurationPtr(diag, "oidc.config/azure/userGroupOverageClaim/cacheExpiration", strFromMap(ug, "cacheExpiration")); d != nil {
				overage.CacheExpiration = d
				overageChanged = true
			}
		} else {
			if v, ok := az["enableUserGroupOverageClaim"].(bool); ok && v {
				overage.Enabled = v
				overageChanged = true
			}
			if d, _ := parseDurationPtr(diag, "oidc.config/azure/userGroupOverageClaimCacheExpiration", strFromMap(az, "userGroupOverageClaimCacheExpiration")); d != nil {
				overage.CacheExpiration = d
				overageChanged = true
			}
		}
		if overageChanged {
			a.UserGroupOverageClaim = overage
		}
		out.Azure = a
	}
	return out, nil
}

func parseOIDCUserInfo(m map[string]any, diag *Diagnostics) *argov1alpha1.OIDCUserInfoConfig {
	ui := &argov1alpha1.OIDCUserInfoConfig{}
	changed := false
	if v, ok := m["groupsEnabled"].(bool); ok && v {
		ui.GroupsEnabled = v
		changed = true
	}
	if v := strFromMap(m, "baseURL"); v != "" {
		ui.BaseURL = v
		changed = true
	}
	if v := strFromMap(m, "path"); v != "" {
		ui.Path = v
		changed = true
	}
	if d, _ := parseDurationPtr(diag, "oidc.config/userInfo/cacheExpiration", strFromMap(m, "cacheExpiration")); d != nil {
		ui.CacheExpiration = d
		changed = true
	}
	if !changed {
		return nil
	}
	return ui
}
func mapResource(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) error {
	r := &argov1alpha1.ResourceConfig{}
	changed := false
	if v, ok := kt.get("resource.exclusions"); ok && v != "" {
		var list []argov1alpha1.FilteredResource
		if err := yaml.Unmarshal([]byte(v), &list); err != nil {
			return fmt.Errorf("resource.exclusions: %w", err)
		}
		r.Exclusions = list
		changed = true
	}
	if v, ok := kt.get("resource.inclusions"); ok && v != "" {
		var list []argov1alpha1.FilteredResource
		if err := yaml.Unmarshal([]byte(v), &list); err != nil {
			return fmt.Errorf("resource.inclusions: %w", err)
		}
		r.Inclusions = list
		changed = true
	}
	if v, ok := kt.get("resource.compareoptions"); ok && v != "" {
		var m map[string]any
		if err := yaml.Unmarshal([]byte(v), &m); err != nil {
			return fmt.Errorf("resource.compareoptions: %w", err)
		}
		co := &argov1alpha1.CompareOptions{}
		if b, ok := m["ignoreAggregatedRoles"].(bool); ok {
			co.IgnoreAggregatedRoles = b
		}
		if s, ok := m["ignoreResourceStatusField"].(string); ok {
			orig := s
			switch s {
			case "off", "false":
				s = "none"
				if diag != nil {
					diag.Warn(DirCMToCR, "resource.compareoptions", fmt.Sprintf("ignoreResourceStatusField value %q normalized to %q", orig, s))
				}
			}
			co.IgnoreResourceStatusField = s
		}
		if b, ok := m["ignoreDifferencesOnResourceUpdates"].(bool); ok {
			co.IgnoreDifferencesOnResourceUpdates = b
		}
		r.CompareOptions = co
		changed = true
	}
	if v, ok := kt.get("resource.respectRBAC"); ok {
		r.RespectRBAC = v
		changed = true
	}
	if v, ok := kt.get("resource.ignoreResourceUpdatesEnabled"); ok {
		b := strings.EqualFold(v, "true")
		r.IgnoreResourceUpdatesEnabled = &b
		changed = true
	}
	customs, err := parseResourceCustomizations(kt, diag)
	if err != nil {
		return err
	}
	if len(customs) > 0 {
		r.Customizations = customs
		changed = true
	}
	if csv, ok := kt.get("resource.customLabels"); ok && csv != "" {
		r.CustomLabelKeys = splitCSV(csv)
		changed = true
	}
	if csv, ok := kt.get("resource.sensitive.mask.annotations"); ok && csv != "" {
		r.SensitiveMaskAnnotationKeys = splitCSV(csv)
		changed = true
	}
	if csv, ok := kt.get("resource.includeEventLabelKeys"); ok && csv != "" {
		el := r.EventLabels
		if el == nil {
			el = &argov1alpha1.EventLabelsConfig{}
			r.EventLabels = el
		}
		el.IncludeKeyGlobs = splitCSV(csv)
		changed = true
	}
	if csv, ok := kt.get("resource.excludeEventLabelKeys"); ok && csv != "" {
		el := r.EventLabels
		if el == nil {
			el = &argov1alpha1.EventLabelsConfig{}
			r.EventLabels = el
		}
		el.ExcludeKeyGlobs = splitCSV(csv)
		changed = true
	}
	if changed {
		ensureController(spec).Resource = r
	}
	return nil
}

func parseResourceCustomizations(kt *keyTracker, diag *Diagnostics) ([]argov1alpha1.ResourceCustomization, error) {
	byKey := map[string]*argov1alpha1.ResourceCustomization{}

	ensure := func(group, kind string) *argov1alpha1.ResourceCustomization {
		key := group + "/" + kind
		if c, ok := byKey[key]; ok {
			return c
		}
		c := &argov1alpha1.ResourceCustomization{Group: group, Kind: kind}
		byKey[key] = c
		return c
	}

	if v, ok := kt.get("resource.customizations"); ok && v != "" {
		var m map[string]map[string]any
		if err := yaml.Unmarshal([]byte(v), &m); err != nil {
			return nil, fmt.Errorf("resource.customizations: %w", err)
		}
		for gk, fields := range m {
			group, kind := splitGroupKind(gk)
			c := ensure(group, kind)
			if err := applyOverrideMap(c, fields); err != nil {
				return nil, fmt.Errorf("resource.customizations: %w", err)
			}
		}
	}

	const prefix = "resource.customizations."
	for k, v := range kt.source {
		if !strings.HasPrefix(k, prefix) || k == "resource.customizations" {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		parts := strings.SplitN(rest, ".", 2)
		if len(parts) != 2 {
			continue
		}
		kt.use(k)
		typ, gk := parts[0], parts[1]
		group, kind := splitGroupKind(strings.ReplaceAll(gk, "_", "/"))
		if group == "all" && kind == "" {
			group, kind = "*", "*"
		}
		if gk == "all" {
			group, kind = "*", "*"
		}
		c := ensure(group, kind)
		switch typ {
		case "health":
			c.HealthLua = v
		case "useOpenLibs":
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			c.UseOpenLibs = b
		case "actions":
			a, err := parseResourceActions(v)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			c.Actions = a
		case "ignoreDifferences":
			var d argov1alpha1.OverrideIgnoreDiff
			if err := yaml.Unmarshal([]byte(v), &d); err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			c.IgnoreDifferences = &d
		case "ignoreResourceUpdates":
			var d argov1alpha1.OverrideIgnoreDiff
			if err := yaml.Unmarshal([]byte(v), &d); err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			c.IgnoreResourceUpdates = &d
		case "knownTypeFields":
			var fields []argov1alpha1.KnownTypeField
			if err := yaml.Unmarshal([]byte(v), &fields); err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			c.KnownTypeFields = fields
		}
	}

	out := make([]argov1alpha1.ResourceCustomization, 0, len(byKey))
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, *byKey[k])
	}
	return out, nil
}

func applyOverrideMap(c *argov1alpha1.ResourceCustomization, fields map[string]any) error {
	if s, ok := fields["health.lua"].(string); ok {
		c.HealthLua = s
	}
	if b, ok := fields["health.lua.useOpenLibs"].(bool); ok {
		c.UseOpenLibs = b
	}
	if s, ok := fields["actions"].(string); ok {
		a, err := parseResourceActions(s)
		if err == nil {
			c.Actions = a
		}
	} else if m, ok := fields["actions"].(map[string]any); ok {
		b, _ := yaml.Marshal(m)
		a, err := parseResourceActions(string(b))
		if err == nil {
			c.Actions = a
		}
	}
	if id, ok := fields["ignoreDifferences"]; ok {
		b, err := yaml.Marshal(id)
		if err != nil {
			return err
		}
		var d argov1alpha1.OverrideIgnoreDiff
		if err := yaml.Unmarshal(b, &d); err != nil {
			return err
		}
		c.IgnoreDifferences = &d
	}
	if id, ok := fields["ignoreResourceUpdates"]; ok {
		b, err := yaml.Marshal(id)
		if err != nil {
			return err
		}
		var d argov1alpha1.OverrideIgnoreDiff
		if err := yaml.Unmarshal(b, &d); err != nil {
			return err
		}
		c.IgnoreResourceUpdates = &d
	}
	if ktf, ok := fields["knownTypeFields"]; ok {
		b, err := yaml.Marshal(ktf)
		if err != nil {
			return err
		}
		var ktfFields []argov1alpha1.KnownTypeField
		if err := yaml.Unmarshal(b, &ktfFields); err != nil {
			return err
		}
		c.KnownTypeFields = ktfFields
	}
	return nil
}

func splitGroupKind(gk string) (group, kind string) {
	if gk == "*/*" || gk == "all" {
		return "*", "*"
	}
	if i := strings.LastIndex(gk, "/"); i >= 0 {
		return gk[:i], gk[i+1:]
	}
	return "", gk
}

func parseResourceActions(raw string) (*argov1alpha1.ResourceActionsConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var m map[string]any
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	out := &argov1alpha1.ResourceActionsConfig{}
	if s, ok := m["discovery.lua"].(string); ok {
		out.DiscoveryLua = s
	}
	if b, ok := m["mergeBuiltinActions"].(bool); ok {
		out.MergeBuiltinActions = b
	}
	if defs, ok := m["definitions"].([]any); ok {
		for _, d := range defs {
			dm, ok := d.(map[string]any)
			if !ok {
				continue
			}
			out.Definitions = append(out.Definitions, argov1alpha1.ResourceActionDefinition{
				Name:      asString(dm["name"]),
				ActionLua: asString(dm["action.lua"]),
			})
		}
	}
	return out, nil
}

func marshalResourceActions(a *argov1alpha1.ResourceActionsConfig) (string, error) {
	if a == nil {
		return "", nil
	}
	m := map[string]any{}
	if a.DiscoveryLua != "" {
		m["discovery.lua"] = a.DiscoveryLua
	}
	if a.MergeBuiltinActions {
		m["mergeBuiltinActions"] = true
	}
	if len(a.Definitions) > 0 {
		defs := make([]any, 0, len(a.Definitions))
		for _, d := range a.Definitions {
			defs = append(defs, map[string]any{
				"name":       d.Name,
				"action.lua": d.ActionLua,
			})
		}
		m["definitions"] = defs
	}
	if len(m) == 0 {
		return "", nil
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
func mapApplication(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	if v, ok := kt.get("application.instanceLabelKey"); ok {
		ensureController(spec).InstanceLabelKey = v
	}
	if v, ok := kt.get("application.resourceTrackingMethod"); ok {
		ensureController(spec).ResourceTrackingMethod = v
	}
	if v, ok := kt.get("application.allowedNodeLabels"); ok && v != "" {
		ensureController(spec).AllowedNodeLabelKeys = splitCSV(v)
	}
	syncChanged := false
	sync := &argov1alpha1.ApplicationSyncConfig{}
	if v, ok := kt.get("application.sync.impersonation.enabled"); ok {
		b := strings.EqualFold(v, "true")
		if sync.Impersonation == nil {
			sync.Impersonation = &argov1alpha1.SyncImpersonationConfig{}
		}
		sync.Impersonation.Enabled = &b
		syncChanged = true
	}
	if v, ok := kt.get("application.sync.impersonation.enforced"); ok {
		b := !strings.EqualFold(v, "false")
		if sync.Impersonation == nil {
			sync.Impersonation = &argov1alpha1.SyncImpersonationConfig{}
		}
		sync.Impersonation.Enforced = &b
		syncChanged = true
	}
	if v, ok := kt.get("application.sync.requireOverridePrivilegeForRevisionSync"); ok {
		b := strings.EqualFold(v, "true")
		sync.RequireOverridePrivilegeForRevisionSync = &b
		syncChanged = true
	}
	if syncChanged {
		ensureController(spec).Sync = sync
	}
	if v, ok := kt.get("cluster.inClusterEnabled"); ok {
		b := !strings.EqualFold(v, "false")
		if spec.Cluster == nil {
			spec.Cluster = &argov1alpha1.ClusterConfig{}
		}
		spec.Cluster.InClusterEnabled = &b
	}
}

func mapUI(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	ui := &argov1alpha1.UIConfig{}
	changed := false
	if v, ok := kt.get("ui.cssurl"); ok {
		ui.CSSURL = v
		changed = true
	}
	if v, ok := kt.get("ui.bannercontent"); ok {
		if ui.Banner == nil {
			ui.Banner = &argov1alpha1.UIBannerConfig{}
		}
		ui.Banner.Content = v
		changed = true
	}
	if v, ok := kt.get("ui.bannerurl"); ok {
		if ui.Banner == nil {
			ui.Banner = &argov1alpha1.UIBannerConfig{}
		}
		ui.Banner.URL = v
		changed = true
	}
	if v, ok := kt.get("ui.bannerpermanent"); ok {
		b := strings.EqualFold(v, "true")
		if ui.Banner == nil {
			ui.Banner = &argov1alpha1.UIBannerConfig{}
		}
		ui.Banner.Permanent = &b
		changed = true
	}
	if v, ok := kt.get("ui.bannerposition"); ok {
		if ui.Banner == nil {
			ui.Banner = &argov1alpha1.UIBannerConfig{}
		}
		ui.Banner.Position = v
		changed = true
	}
	if v, ok := kt.get("ui.loginButtonText"); ok {
		ui.LoginButtonText = v
		changed = true
	}
	if changed {
		ensureServer(spec).UI = ui
	}
}

func mapUsers(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	u := &argov1alpha1.UsersConfig{}
	changed := false
	if v, ok := kt.get("users.anonymous.enabled"); ok {
		b := strings.EqualFold(v, "true")
		u.AnonymousEnabled = &b
		changed = true
	}
	if v, ok := kt.get("users.session.duration"); ok {
		if d, _ := parseDurationPtr(diag, "users.session.duration", v); d != nil {
			u.SessionDuration = d
			changed = true
		}
	}
	if v, ok := kt.get("passwordPattern"); ok {
		u.PasswordRegex = v
		changed = true
	}
	if changed {
		ensureServer(spec).Users = u
	}
	if v, ok := kt.get("server.maxPodLogsToRender"); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			ensureServer(spec).MaxPodLogsToRender = &n
		}
	}
}

func mapHelp(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	h := &argov1alpha1.HelpConfig{}
	changed := false
	if v, ok := kt.get("help.chatUrl"); ok {
		if h.Chat == nil {
			h.Chat = &argov1alpha1.HelpChatConfig{}
		}
		h.Chat.URL = v
		changed = true
	}
	if v, ok := kt.get("help.chatText"); ok {
		if h.Chat == nil {
			h.Chat = &argov1alpha1.HelpChatConfig{}
		}
		h.Chat.Text = v
		changed = true
	}
	bins := map[string]string{}
	const prefix = "help.download."
	for k, v := range kt.source {
		if strings.HasPrefix(k, prefix) {
			bins[strings.TrimPrefix(k, prefix)] = v
			changed = true
		}
	}
	if len(bins) > 0 {
		h.BinaryURLs = bins
	}
	if changed {
		ensureServer(spec).Help = h
	}
}

func mapAccounts(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	accounts := map[string]*argov1alpha1.AccountConfig{}
	for k, v := range kt.source {
		if !strings.HasPrefix(k, "accounts.") {
			continue
		}
		kt.use(k)
		rest := strings.TrimPrefix(k, "accounts.")
		if strings.HasSuffix(rest, ".enabled") {
			name := strings.TrimSuffix(rest, ".enabled")
			a := accounts[name]
			if a == nil {
				a = &argov1alpha1.AccountConfig{Name: name}
				accounts[name] = a
			}
			a.Enabled = strings.EqualFold(v, "true")
			continue
		}
		if strings.Contains(rest, ".") {
			continue // password etc. are in secret
		}
		name := rest
		a := accounts[name]
		if a == nil {
			a = &argov1alpha1.AccountConfig{Name: name}
			accounts[name] = a
		}
		a.Capabilities = splitCSV(v)
	}
	if v, ok := kt.get("admin.enabled"); ok {
		a := accounts["admin"]
		if a == nil {
			a = &argov1alpha1.AccountConfig{Name: "admin"}
			accounts["admin"] = a
		}
		a.Enabled = strings.EqualFold(v, "true")
	}
	if len(accounts) == 0 {
		return
	}
	srv := ensureServer(spec)
	names := make([]string, 0, len(accounts))
	for name := range accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		srv.Accounts = append(srv.Accounts, *accounts[name])
	}
}

func mapExtensions(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) error {
	var exts []argov1alpha1.ExtensionConfig
	if v, ok := kt.get("extension.config"); ok && v != "" {
		var wrap struct {
			Extensions []struct {
				Name    string         `yaml:"name"`
				Backend map[string]any `yaml:"backend"`
			} `yaml:"extensions"`
		}
		if err := yaml.Unmarshal([]byte(v), &wrap); err != nil {
			return fmt.Errorf("extension.config: %w", err)
		}
		for _, e := range wrap.Extensions {
			ext := argov1alpha1.ExtensionConfig{Name: e.Name}
			b, _ := yaml.Marshal(e.Backend)
			backend, err := parseExtensionBackend(b, diag)
			if err != nil {
				return err
			}
			ext.Backend = backend
			exts = append(exts, ext)
		}
	}
	const prefix = "extension.config."
	for k, v := range kt.source {
		if !strings.HasPrefix(k, prefix) || k == "extension.config" {
			continue
		}
		kt.use(k)
		name := strings.TrimPrefix(k, prefix)
		backend, err := parseExtensionBackend([]byte(v), diag)
		if err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
		exts = append(exts, argov1alpha1.ExtensionConfig{Name: name, Backend: backend})
	}
	if len(exts) > 0 {
		ensureServer(spec).Extensions = exts
	}
	return nil
}

func parseExtensionBackend(raw []byte, diag *Diagnostics) (argov1alpha1.ExtensionBackend, error) {
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return argov1alpha1.ExtensionBackend{}, err
	}
	b := argov1alpha1.ExtensionBackend{}
	if transport, ok := m["transport"].(map[string]any); ok {
		b.Transport = parseExtensionTransport(transport, diag)
	} else {
		// Legacy flat keys in extension backend YAML
		tr := &argov1alpha1.ExtensionTransportConfig{}
		trChanged := false
		if d, _ := parseDurationPtr(diag, "extension.config/connectionTimeout", strFromMap(m, "connectionTimeout")); d != nil {
			tr.ConnectionTimeout = d
			trChanged = true
		}
		if d, _ := parseDurationPtr(diag, "extension.config/keepAlive", strFromMap(m, "keepAlive")); d != nil {
			tr.KeepAlive = d
			trChanged = true
		}
		if d, _ := parseDurationPtr(diag, "extension.config/idleConnectionTimeout", strFromMap(m, "idleConnectionTimeout")); d != nil {
			tr.IdleConnectionTimeout = d
			trChanged = true
		}
		if n, ok := asInt(m["maxIdleConnections"]); ok {
			tr.MaxIdleConnections = int32(n)
			trChanged = true
		}
		if trChanged {
			b.Transport = tr
		}
	}
	if svcs, ok := m["services"].([]any); ok {
		for _, s := range svcs {
			sm, _ := s.(map[string]any)
			svc := argov1alpha1.ExtensionService{URL: asString(sm["url"])}
			if c, ok := sm["cluster"].(map[string]any); ok {
				svc.Cluster = &argov1alpha1.ExtensionCluster{
					ServerURL: strFromMap(c, "server"),
					Name:      strFromMap(c, "name"),
				}
			}
			if hdrs, ok := sm["headers"].([]any); ok {
				for _, h := range hdrs {
					hm, _ := h.(map[string]any)
					svc.Headers = append(svc.Headers, argov1alpha1.ExtensionHeader{
						Name:  asString(hm["name"]),
						Value: asString(hm["value"]), // may be $string
					})
				}
			}
			b.Services = append(b.Services, svc)
		}
	}
	return b, nil
}

func parseExtensionTransport(m map[string]any, diag *Diagnostics) *argov1alpha1.ExtensionTransportConfig {
	tr := &argov1alpha1.ExtensionTransportConfig{}
	changed := false
	if d, _ := parseDurationPtr(diag, "extension.config/transport/connectionTimeout", strFromMap(m, "connectionTimeout")); d != nil {
		tr.ConnectionTimeout = d
		changed = true
	}
	if d, _ := parseDurationPtr(diag, "extension.config/transport/keepAlive", strFromMap(m, "keepAlive")); d != nil {
		tr.KeepAlive = d
		changed = true
	}
	if d, _ := parseDurationPtr(diag, "extension.config/transport/idleConnectionTimeout", strFromMap(m, "idleConnectionTimeout")); d != nil {
		tr.IdleConnectionTimeout = d
		changed = true
	}
	if n, ok := asInt(m["maxIdleConnections"]); ok {
		tr.MaxIdleConnections = int32(n)
		changed = true
	}
	if !changed {
		return nil
	}
	return tr
}

func mapGlobalProjects(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) error {
	v, ok := kt.get("globalProjects")
	if !ok || v == "" {
		return nil
	}
	var list []argov1alpha1.GlobalProjectConfig
	if err := yaml.Unmarshal([]byte(v), &list); err != nil {
		return fmt.Errorf("globalProjects: %w", err)
	}
	ensureController(spec).GlobalProjects = list
	return nil
}

func mapDeepLinks(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) error {
	dl := &argov1alpha1.DeepLinksConfig{}
	changed := false
	for key, dest := range map[string]*[]argov1alpha1.DeepLink{
		"application.links": &dl.Application,
		"project.links":     &dl.Project,
		"resource.links":    &dl.Resource,
	} {
		v, ok := kt.get(key)
		if !ok || v == "" {
			continue
		}
		var raw []map[string]any
		if err := yaml.Unmarshal([]byte(v), &raw); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		for _, m := range raw {
			link := argov1alpha1.DeepLink{
				URLTemplate: asString(m["url"]),
				Title:       asString(m["title"]),
			}
			if d, ok := m["description"].(string); ok {
				link.Description = d
			}
			if ic, ok := m["icon.class"].(string); ok {
				link.IconClass = ic
			}
			if cond, ok := m["if"].(string); ok {
				link.ConditionExpr = cond
			}
			*dest = append(*dest, link)
		}
		changed = true
	}
	if changed {
		ensureServer(spec).DeepLinks = dl
	}
	return nil
}

func mapKustomize(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	k := &argov1alpha1.KustomizeConfig{}
	changed := false
	if v, ok := kt.get("kustomize.enable"); ok {
		b := !strings.EqualFold(v, "false")
		k.Enabled = &b
		changed = true
	}
	if v, ok := kt.get("kustomize.buildOptions"); ok {
		k.BuildOptions = v
		changed = true
	}
	versions := map[string]*argov1alpha1.KustomizeVersion{}
	ensure := func(name string) *argov1alpha1.KustomizeVersion {
		if v, ok := versions[name]; ok {
			return v
		}
		v := &argov1alpha1.KustomizeVersion{Name: name}
		versions[name] = v
		return v
	}
	for key, val := range kt.source {
		switch {
		case strings.HasPrefix(key, "kustomize.version."):
			kt.use(key)
			name := strings.TrimPrefix(key, "kustomize.version.")
			ensure(name)
			if val != "" {
				ensure(name).Path = val
			}
			changed = true
		case strings.HasPrefix(key, "kustomize.path."):
			kt.use(key)
			name := strings.TrimPrefix(key, "kustomize.path.")
			ensure(name).Path = val
			changed = true
		case strings.HasPrefix(key, "kustomize.buildOptions."):
			kt.use(key)
			name := strings.TrimPrefix(key, "kustomize.buildOptions.")
			ensure(name).BuildOptions = val
			changed = true
		}
	}
	names := make([]string, 0, len(versions))
	for name := range versions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		k.Versions = append(k.Versions, *versions[name])
	}
	if changed {
		ensureRepoServer(spec).Kustomize = k
	}
}

func mapHelm(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	h := &argov1alpha1.HelmConfig{}
	changed := false
	if v, ok := kt.get("helm.enable"); ok {
		b := !strings.EqualFold(v, "false")
		h.Enabled = &b
		changed = true
	}
	if v, ok := kt.get("helm.valuesFileSchemes"); ok && v != "" {
		h.ValuesFileSchemes = splitCSV(v)
		changed = true
	}
	if changed {
		ensureRepoServer(spec).Helm = h
	}
}

func mapMisc(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	if v, ok := kt.get("ga.trackingid"); ok {
		ga := &argov1alpha1.GoogleAnalyticsConfig{TrackingID: v}
		if a, ok := kt.get("ga.anonymizeusers"); ok {
			ga.AnonymizeUsers = !strings.EqualFold(a, "false")
		}
		ensureServer(spec).GoogleAnalytics = ga
	}
	if v, ok := kt.get("sourceHydrator.commitMessageTemplate"); ok {
		ctrl := ensureController(spec)
		if ctrl.SourceHydrator == nil {
			ctrl.SourceHydrator = &argov1alpha1.SourceHydratorConfig{}
		}
		ctrl.SourceHydrator.CommitMessageTemplate = v
	}
	if v, ok := kt.get("sourceHydrator.readmeMessageTemplate"); ok {
		ctrl := ensureController(spec)
		if ctrl.SourceHydrator == nil {
			ctrl.SourceHydrator = &argov1alpha1.SourceHydratorConfig{}
		}
		ctrl.SourceHydrator.ReadmeMessageTemplate = v
	}
	if v, ok := kt.get("commit.author.name"); ok {
		ensureCommitAuthor(ensureCommitServer(spec)).Name = v
	}
	if v, ok := kt.get("commit.author.email"); ok {
		ensureCommitAuthor(ensureCommitServer(spec)).Email = v
	}
	if v, ok := kt.get("exec.enabled"); ok {
		srv := ensureServer(spec)
		if srv.Exec == nil {
			srv.Exec = &argov1alpha1.ExecConfig{}
		}
		srv.Exec.Enabled = strings.EqualFold(v, "true")
	}
	if v, ok := kt.get("exec.shells"); ok && v != "" {
		srv := ensureServer(spec)
		if srv.Exec == nil {
			srv.Exec = &argov1alpha1.ExecConfig{}
		}
		srv.Exec.Shells = splitCSV(v)
	}
	if v, ok := kt.get("statusbadge.enabled"); ok {
		srv := ensureServer(spec)
		if srv.StatusBadge == nil {
			srv.StatusBadge = &argov1alpha1.StatusBadgeConfig{}
		}
		srv.StatusBadge.Enabled = strings.EqualFold(v, "true")
	}
	if v, ok := kt.get("statusbadge.url"); ok {
		srv := ensureServer(spec)
		if srv.StatusBadge == nil {
			srv.StatusBadge = &argov1alpha1.StatusBadgeConfig{}
		}
		srv.StatusBadge.URL = v
	}
	if v, ok := kt.get("webhook.maxPayloadSizeMB"); ok {
		srv := ensureServer(spec)
		if srv.Webhook == nil {
			srv.Webhook = &argov1alpha1.WebhookConfig{}
		}
		q, err := resource.ParseQuantity(v + "M")
		if err != nil {
			if diag != nil {
				diag.Error(DirCMToCR, "webhook.maxPayloadSizeMB", fmt.Sprintf("invalid quantity %qM: %v", v, err))
			}
		} else {
			if diag != nil {
				diag.Warn(DirCMToCR, "webhook.maxPayloadSizeMB", "decimal megabyte (M) quantity may lose precision when round-tripped through integer MB")
			}
			srv.Webhook.MaxPayloadSize = &q
		}
	}
	if v, ok := kt.get("webhook.refresh.jitter"); ok {
		srv := ensureServer(spec)
		if srv.Webhook == nil {
			srv.Webhook = &argov1alpha1.WebhookConfig{}
		}
		if srv.Webhook.Refresh == nil {
			srv.Webhook.Refresh = &argov1alpha1.WebhookRefreshConfig{}
		}
		srv.Webhook.Refresh.Jitter, _ = parseDurationPtr(diag, "webhook.refresh.jitter", v)
	}
	if v, ok := kt.get("webhook.refresh.jitter.threshold"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			srv := ensureServer(spec)
			if srv.Webhook == nil {
				srv.Webhook = &argov1alpha1.WebhookConfig{}
			}
			if srv.Webhook.Refresh == nil {
				srv.Webhook.Refresh = &argov1alpha1.WebhookRefreshConfig{}
			}
			i := int32(n)
			srv.Webhook.Refresh.JitterThreshold = &i
		}
	}
	if v, ok := kt.get("timeout.reconciliation"); ok {
		if d, err := parseDurationPtr(diag, "timeout.reconciliation", v); err == nil && d != nil {
			ctrl := ensureController(spec)
			if ctrl.Reconciliation == nil {
				ctrl.Reconciliation = &argov1alpha1.ReconciliationConfig{}
			}
			ctrl.Reconciliation.Timeout = d
		}
	}
	if v, ok := kt.get("timeout.reconciliation.jitter"); ok {
		ctrl := ensureController(spec)
		if ctrl.Reconciliation == nil {
			ctrl.Reconciliation = &argov1alpha1.ReconciliationConfig{}
		}
		ctrl.Reconciliation.Jitter, _ = parseDurationPtr(diag, "timeout.reconciliation.jitter", v)
	}
	if v, ok := kt.get("installationID"); ok {
		spec.InstallationID = v
	}
	if v, ok := kt.get("jsonnet.enable"); ok {
		b := !strings.EqualFold(v, "false")
		ensureRepoServer(spec).Jsonnet = &argov1alpha1.JsonnetConfig{Enabled: &b}
	}
	if v, ok := kt.get("server.rbac.disableApplicationFineGrainedRBACInheritance"); ok {
		b := strings.EqualFold(v, "true")
		srv := ensureServer(spec)
		if srv.RBAC == nil {
			srv.RBAC = &argov1alpha1.RBACConfig{}
		}
		srv.RBAC.ApplicationFineGrainedInheritanceDisabled = &b
	}
}

func mapCmdParams(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) error {
	if v, ok := kt.get("application.namespaces"); ok && v != "" {
		spec.ApplicationNamespaceGlobs = splitCSV(v)
	}
	if v, ok := kt.get("repo.server"); ok {
		ensureRepoServer(spec).Address = v
	}
	if v, ok := kt.get("commit.server"); ok {
		ensureCommitServer(spec).Address = v
	}
	if v, ok := kt.get("log.format.timestamp"); ok {
		ensureLogging(spec).FormatTimestamp = v
	}
	mapRedis(kt, spec, diag)
	mapOTLP(kt, spec, diag)
	mapRepoServerClient(kt, spec, diag)
	mapControllerCmdParams(kt, spec, diag)
	mapServerCmdParams(kt, spec, diag)
	mapRepoServerCmdParams(kt, spec, diag)
	mapApplicationSetCmdParams(kt, spec, diag)
	mapCommitServerCmdParams(kt, spec, diag)
	mapDexServerCmdParams(kt, spec, diag)
	mapNotificationsCmdParams(kt, spec, diag)

	if v, ok := kt.get("server.insecure"); ok {
		b := strings.EqualFold(v, "true")
		ensureServer(spec).TLSDisabled = &b
	}
	if v, ok := kt.get("server.log.format"); ok {
		ensureServer(spec).LogFormat = v
	}
	if v, ok := kt.get("server.log.level"); ok {
		ensureServer(spec).LogLevel = v
	}
	if v, ok := kt.get("reposerver.log.format"); ok {
		ensureRepoServer(spec).LogFormat = v
	}
	if v, ok := kt.get("reposerver.log.level"); ok {
		ensureRepoServer(spec).LogLevel = v
	}
	if v, ok := kt.get("commitserver.log.format"); ok {
		ensureCommitServer(spec).LogFormat = v
	}
	if v, ok := kt.get("commitserver.log.level"); ok {
		ensureCommitServer(spec).LogLevel = v
	}
	if v, ok := kt.get("applicationsetcontroller.namespaces"); ok && v != "" {
		if spec.ApplicationSet == nil {
			spec.ApplicationSet = &argov1alpha1.ApplicationSetConfig{}
		}
		spec.ApplicationSet.NamespaceGlobs = splitCSV(v)
	}
	if v, ok := kt.get("applicationsetcontroller.policy"); ok {
		if spec.ApplicationSet == nil {
			spec.ApplicationSet = &argov1alpha1.ApplicationSetConfig{}
		}
		spec.ApplicationSet.Policy = v
	}
	if v, ok := kt.get("reposerver.plugin.tar.exclusions"); ok && v != "" {
		rs := ensureRepoServer(spec)
		if rs.Plugin == nil {
			rs.Plugin = &argov1alpha1.RepoServerPluginConfig{}
		}
		rs.Plugin.TarExclusionGlobs = splitSep(v, ";")
	}
	return nil
}
func mapControllerCmdParams(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	c := ensureController(spec)
	if v, ok := kt.get("controller.sharding.algorithm"); ok {
		c.ShardingAlgorithm = v
	}
	if v, ok := kt.get("controller.log.format"); ok {
		c.LogFormat = v
	}
	if v, ok := kt.get("controller.log.level"); ok {
		c.LogLevel = v
	}
	if v, ok := kt.get("controller.status.processors"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			if c.Processors == nil {
				c.Processors = &argov1alpha1.ControllerProcessorsConfig{}
			}
			c.Processors.Status = &i
		}
	}
	if v, ok := kt.get("controller.operation.processors"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			if c.Processors == nil {
				c.Processors = &argov1alpha1.ControllerProcessorsConfig{}
			}
			c.Processors.Operation = &i
		}
	}
	if v, ok := kt.get("controller.hydration.processors"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			if c.Processors == nil {
				c.Processors = &argov1alpha1.ControllerProcessorsConfig{}
			}
			c.Processors.Hydration = &i
		}
	}
	if v, ok := kt.get("controller.app.state.cache.expiration"); ok {
		if c.Cache == nil {
			c.Cache = &argov1alpha1.ControllerCacheConfig{}
		}
		c.Cache.AppStateExpiration, _ = parseDurationPtr(diag, "controller.app.state.cache.expiration", v)
	}
	if v, ok := kt.get("controller.default.cache.expiration"); ok {
		if c.Cache == nil {
			c.Cache = &argov1alpha1.ControllerCacheConfig{}
		}
		c.Cache.DefaultExpiration, _ = parseDurationPtr(diag, "controller.default.cache.expiration", v)
	}
	if v, ok := kt.get("controller.resource.health.persist"); ok {
		b := strings.EqualFold(v, "true")
		c.ResourceHealthPersist = &b
	}
	if v, ok := kt.get("controller.kubectl.parallelism.limit"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			c.KubectlParallelismLimit = &i
		}
	}
	if v, ok := kt.get("controller.diff.server.side"); ok {
		b := strings.EqualFold(v, "true")
		c.DiffServerSide = &b
	}

	metricsChanged := false
	m := &argov1alpha1.ControllerMetricsConfig{}
	if v, ok := kt.get("controller.metrics.cache.expiration"); ok {
		m.CacheExpiration, _ = parseDurationPtr(diag, "controller.metrics.cache.expiration", v)
		metricsChanged = true
	}
	if v, ok := kt.get("controller.metrics.application.labels"); ok && v != "" {
		if m.Application == nil {
			m.Application = &argov1alpha1.ControllerMetricsApplicationConfig{}
		}
		m.Application.LabelKeys = splitCSV(v)
		metricsChanged = true
	}
	if v, ok := kt.get("controller.metrics.application.conditions"); ok && v != "" {
		if m.Application == nil {
			m.Application = &argov1alpha1.ControllerMetricsApplicationConfig{}
		}
		m.Application.Conditions = splitCSV(v)
		metricsChanged = true
	}
	if v, ok := kt.get("controller.metrics.cluster.labels"); ok && v != "" {
		if m.Cluster == nil {
			m.Cluster = &argov1alpha1.ControllerMetricsClusterConfig{}
		}
		m.Cluster.LabelKeys = splitCSV(v)
		metricsChanged = true
	}
	if metricsChanged {
		c.Metrics = m
	}

	selfChanged := false
	sh := &argov1alpha1.SelfHealConfig{}
	if v, ok := kt.get("controller.self.heal.timeout.seconds"); ok {
		sh.Timeout, _ = secondsDurationPtr(diag, "controller.self.heal.timeout.seconds", v)
		selfChanged = true
	}
	boChanged := false
	bo := &argov1alpha1.BackoffConfig{}
	if v, ok := kt.get("controller.self.heal.backoff.timeout.seconds"); ok {
		bo.Duration, _ = secondsDurationPtr(diag, "controller.self.heal.backoff.timeout.seconds", v)
		boChanged = true
	}
	if v, ok := kt.get("controller.self.heal.backoff.factor"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			i := int32(n)
			bo.Factor = &i
			boChanged = true
		}
	}
	if v, ok := kt.get("controller.self.heal.backoff.cap.seconds"); ok {
		bo.MaxDuration, _ = secondsDurationPtr(diag, "controller.self.heal.backoff.cap.seconds", v)
		boChanged = true
	}
	if boChanged {
		sh.Backoff = bo
		selfChanged = true
	}
	if selfChanged {
		c.SelfHeal = sh
	}

	if v, ok := kt.get("controller.sync.timeout.seconds"); ok {
		sync := c.Sync
		if sync == nil {
			sync = &argov1alpha1.ApplicationSyncConfig{}
		}
		sync.Timeout, _ = secondsDurationPtr(diag, "controller.sync.timeout.seconds", v)
		c.Sync = sync
	}
	if v, ok := kt.get("controller.sync.wave.delay.seconds"); ok {
		sync := c.Sync
		if sync == nil {
			sync = &argov1alpha1.ApplicationSyncConfig{}
		}
		sync.WaveDelay, _ = secondsDurationPtr(diag, "controller.sync.wave.delay.seconds", v)
		c.Sync = sync
	}

	if v, ok := kt.get("hydrator.enabled"); ok {
		b := strings.EqualFold(v, "true")
		if c.SourceHydrator == nil {
			c.SourceHydrator = &argov1alpha1.SourceHydratorConfig{}
		}
		c.SourceHydrator.Enabled = &b
	}
	mapControllerCmdParamsExtras(kt, diag, c)
}

func unmapControllerCmdParams(c *argov1alpha1.ControllerConfig, data map[string]string) {
	if c == nil {
		return
	}
	if c.ShardingAlgorithm != "" {
		data["controller.sharding.algorithm"] = c.ShardingAlgorithm
	}
	if c.LogFormat != "" {
		data["controller.log.format"] = c.LogFormat
	}
	if c.LogLevel != "" {
		data["controller.log.level"] = c.LogLevel
	}
	if p := c.Processors; p != nil {
		if p.Status != nil {
			data["controller.status.processors"] = strconv.Itoa(int(*p.Status))
		}
		if p.Operation != nil {
			data["controller.operation.processors"] = strconv.Itoa(int(*p.Operation))
		}
		if p.Hydration != nil {
			data["controller.hydration.processors"] = strconv.Itoa(int(*p.Hydration))
		}
	}
	if cache := c.Cache; cache != nil {
		if s := durationString(cache.AppStateExpiration); s != "" {
			data["controller.app.state.cache.expiration"] = s
		}
		if s := durationString(cache.DefaultExpiration); s != "" {
			data["controller.default.cache.expiration"] = s
		}
	}
	if c.ResourceHealthPersist != nil {
		data["controller.resource.health.persist"] = strconv.FormatBool(*c.ResourceHealthPersist)
	}
	if c.KubectlParallelismLimit != nil {
		data["controller.kubectl.parallelism.limit"] = strconv.Itoa(int(*c.KubectlParallelismLimit))
	}
	if c.DiffServerSide != nil {
		data["controller.diff.server.side"] = strconv.FormatBool(*c.DiffServerSide)
	}
	if m := c.Metrics; m != nil {
		if s := durationString(m.CacheExpiration); s != "" {
			data["controller.metrics.cache.expiration"] = s
		}
		if m.Application != nil {
			if len(m.Application.LabelKeys) > 0 {
				data["controller.metrics.application.labels"] = strings.Join(m.Application.LabelKeys, ",")
			}
			if len(m.Application.Conditions) > 0 {
				data["controller.metrics.application.conditions"] = strings.Join(m.Application.Conditions, ",")
			}
		}
		if m.Cluster != nil && len(m.Cluster.LabelKeys) > 0 {
			data["controller.metrics.cluster.labels"] = strings.Join(m.Cluster.LabelKeys, ",")
		}
	}
	if sh := c.SelfHeal; sh != nil {
		if sh.Timeout != nil {
			data["controller.self.heal.timeout.seconds"] = strconv.Itoa(int(sh.Timeout.Duration.Seconds()))
		}
		if bo := sh.Backoff; bo != nil {
			if bo.Duration != nil {
				data["controller.self.heal.backoff.timeout.seconds"] = strconv.Itoa(int(bo.Duration.Duration.Seconds()))
			}
			if bo.Factor != nil {
				data["controller.self.heal.backoff.factor"] = strconv.Itoa(int(*bo.Factor))
			}
			if bo.MaxDuration != nil {
				data["controller.self.heal.backoff.cap.seconds"] = strconv.Itoa(int(bo.MaxDuration.Duration.Seconds()))
			}
		}
	}
	if sync := c.Sync; sync != nil {
		if sync.Timeout != nil {
			data["controller.sync.timeout.seconds"] = strconv.Itoa(int(sync.Timeout.Duration.Seconds()))
		}
		if sync.WaveDelay != nil {
			data["controller.sync.wave.delay.seconds"] = strconv.Itoa(int(sync.WaveDelay.Duration.Seconds()))
		}
	}
	if hy := c.SourceHydrator; hy != nil && hy.Enabled != nil {
		data["hydrator.enabled"] = strconv.FormatBool(*hy.Enabled)
	}
	unmapControllerCmdParamsExtras(c, data)
}

func secondsDurationPtr(diag *Diagnostics, key, s string) (*metav1.Duration, error) {
	if s == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		if diag != nil && key != "" {
			diag.Error(DirCMToCR, key, fmt.Sprintf("invalid seconds duration %q: %v", s, err))
		}
		return nil, err
	}
	return &metav1.Duration{Duration: time.Duration(n) * time.Second}, nil
}

func mapRedis(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	changed := false
	r := &argov1alpha1.RedisConfig{}
	if v, ok := kt.get("redis.server"); ok {
		r.Server = v
		changed = true
	}
	if v, ok := kt.get("redis.compression"); ok {
		r.Compression = v
		changed = true
	}
	if v, ok := kt.get("redis.sentinel.hosts"); ok && v != "" {
		if r.Sentinel == nil {
			r.Sentinel = &argov1alpha1.RedisSentinelConfig{}
		}
		r.Sentinel.Hosts = splitCSV(v)
		changed = true
	}
	if v, ok := kt.get("redis.sentinel.master"); ok {
		if r.Sentinel == nil {
			r.Sentinel = &argov1alpha1.RedisSentinelConfig{}
		}
		r.Sentinel.Master = v
		changed = true
	}
	if v, ok := kt.get("redis.db"); ok {
		r.DB = v
		changed = true
	}
	if changed {
		*ensureRedis(spec) = *r
	}
}

func mapOTLP(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	changed := false
	o := &argov1alpha1.OTLPConfig{}
	if v, ok := kt.get("otlp.address"); ok {
		o.Address = v
		changed = true
	}
	if v, ok := kt.get("otlp.insecure"); ok {
		b := !strings.EqualFold(v, "true")
		o.TLSEnabled = &b
		changed = true
	}
	if v, ok := kt.get("otlp.headers"); ok && v != "" {
		o.Headers = parseKVMap(v, "=", ",")
		changed = true
	}
	if v, ok := kt.get("otlp.attrs"); ok && v != "" {
		o.Attrs = parseKVMap(v, ":", ",")
		changed = true
	}
	if v, ok := kt.get("otlp.sample.ratio"); ok {
		o.SampleRatio = v
		changed = true
	}
	if changed {
		*ensureOTLP(spec) = *o
	}
}

func mapRepoServerClient(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) {
	get := func(suffix string) (string, bool) {
		ctrlKey := "controller.repo.server." + suffix
		srvKey := "server.repo.server." + suffix
		cv, cOk := kt.get(ctrlKey)
		sv, sOk := kt.get(srvKey)
		if cOk && sOk {
			if cv != sv && diag != nil {
				diag.Warn(DirCMToCR, ctrlKey, fmt.Sprintf("controller and server values differ (%q vs %q); using controller value", cv, sv))
			} else if diag != nil {
				diag.Warn(DirCMToCR, ctrlKey, "both controller.repo.server.* and server.repo.server.* present with same value; collapsed to single CR field")
			}
			return cv, true
		}
		if cOk {
			return cv, true
		}
		if sOk {
			if diag != nil {
				diag.Warn(DirCMToCR, srvKey, "using server.repo.server.* value; controller.repo.server.* is preferred")
			}
			return sv, true
		}
		return "", false
	}
	changed := false
	c := &argov1alpha1.RepoServerClientConfig{}
	if v, ok := get("timeout.seconds"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			c.Timeout = &metav1.Duration{Duration: time.Duration(n) * time.Second}
			changed = true
		}
	}
	if v, ok := get("plaintext"); ok {
		b := strings.EqualFold(v, "true")
		c.TLSDisabled = &b
		changed = true
	}
	if v, ok := get("strict.tls"); ok {
		b := !strings.EqualFold(v, "true")
		c.InsecureSkipVerify = &b
		changed = true
	}
	if v, ok := get("ca.cert.path"); ok {
		if c.MTLS == nil {
			c.MTLS = &argov1alpha1.MTLSCertConfig{}
		}
		c.MTLS.CACertPath = v
		changed = true
	}
	if v, ok := get("client.cert.path"); ok {
		if c.MTLS == nil {
			c.MTLS = &argov1alpha1.MTLSCertConfig{}
		}
		c.MTLS.ClientCertPath = v
		changed = true
	}
	if v, ok := get("client.cert.key.path"); ok {
		if c.MTLS == nil {
			c.MTLS = &argov1alpha1.MTLSCertConfig{}
		}
		c.MTLS.ClientCertKeyPath = v
		changed = true
	}
	if changed {
		*ensureRepoServerClient(spec) = *c
	}
}

func mapRBAC(kt *keyTracker, spec *argov1alpha1.ArgoCDConfigurationSpec, diag *Diagnostics) error {
	r := &argov1alpha1.RBACConfig{}
	changed := false
	if v, ok := kt.get("policy.default"); ok {
		r.Default = v
		changed = true
	}
	if v, ok := kt.get("scopes"); ok && v != "" {
		var scopes []string
		if err := yaml.Unmarshal([]byte(v), &scopes); err != nil {
			scopes = splitCSV(strings.Trim(v, "[]"))
		}
		r.Scopes = scopes
		changed = true
	}
	if v, ok := kt.get("policy.matchMode"); ok {
		r.MatchMode = v
		changed = true
	}
	if v, ok := kt.get("policy.csv"); ok {
		r.PolicyCSV = v
		changed = true
	}
	const prefix = "policy."
	const suffix = ".csv"
	for k, v := range kt.source {
		if k == "policy.csv" || k == "policy.default" || k == "policy.matchMode" {
			continue
		}
		if strings.HasPrefix(k, prefix) && strings.HasSuffix(k, suffix) {
			kt.use(k)
			name := strings.TrimSuffix(strings.TrimPrefix(k, prefix), suffix)
			r.PolicyOverlays = append(r.PolicyOverlays, argov1alpha1.RBACPolicyOverlay{Name: name, CSV: v})
			changed = true
		}
	}
	sort.Slice(r.PolicyOverlays, func(i, j int) bool {
		return r.PolicyOverlays[i].Name < r.PolicyOverlays[j].Name
	})
	if changed {
		srv := ensureServer(spec)
		var inheritDisabled *bool
		if srv.RBAC != nil {
			inheritDisabled = srv.RBAC.ApplicationFineGrainedInheritanceDisabled
		}
		srv.RBAC = r
		if inheritDisabled != nil {
			srv.RBAC.ApplicationFineGrainedInheritanceDisabled = inheritDisabled
		}
	}
	return nil
}
func unmapCM(spec *argov1alpha1.ArgoCDConfigurationSpec, data map[string]string, diag *Diagnostics) error {
	if s := spec.Server; s != nil {
		if s.URL != "" {
			data["url"] = s.URL
		}
		if len(s.AdditionalURLs) > 0 {
			b, err := yaml.Marshal(s.AdditionalURLs)
			if err != nil {
				return err
			}
			data["additionalUrls"] = string(b)
		}
		if s.OIDCInsecureSkipVerify != nil {
			data["oidc.tls.insecure.skip.verify"] = strconv.FormatBool(*s.OIDCInsecureSkipVerify)
		}
		if s.Dex != nil {
			raw, err := marshalDex(s.Dex)
			if err != nil {
				return err
			}
			data["dex.config"] = raw
		}
		if s.OIDC != nil {
			raw, err := marshalOIDC(s.OIDC)
			if err != nil {
				// Non-argocd-secret clientSecretRef (and similar) cannot be
				// represented in oidc.config; record and skip rather than failing
				// the whole conversion so other CM keys still emit.
				diag.Error(DirCRToCM, "oidc.config", err.Error())
			} else {
				data["oidc.config"] = raw
			}
		}
		unmapUI(s.UI, data)
		unmapUsers(s.Users, data)
		unmapHelp(s.Help, data)
		unmapAccounts(s.Accounts, data)
		if err := unmapExtensions(s.Extensions, data); err != nil {
			return err
		}
		if err := unmapDeepLinks(s.DeepLinks, data); err != nil {
			return err
		}
		unmapServerMisc(s, data)
	}
	if c := spec.Controller; c != nil {
		if err := unmapResource(c.Resource, data); err != nil {
			return err
		}
		unmapApplication(c, data)
		if len(c.GlobalProjects) > 0 {
			b, err := yaml.Marshal(c.GlobalProjects)
			if err != nil {
				return err
			}
			data["globalProjects"] = string(b)
		}
	}
	if r := spec.RepoServer; r != nil {
		unmapKustomize(r.Kustomize, data)
		unmapHelm(r.Helm, data)
		unmapJsonnet(r.Jsonnet, data)
	}
	unmapTopMisc(spec, data)
	return nil
}

func marshalDex(dex *argov1alpha1.DexConfig) (string, error) {
	m := map[string]any{}
	if dex.Extra != nil && len(dex.Extra.Raw) > 0 {
		_ = json.Unmarshal(dex.Extra.Raw, &m)
	}
	var connectors []any
	for _, c := range dex.Connectors {
		cm := map[string]any{
			"type": c.Type,
			"id":   c.ID,
			"name": c.Name,
		}
		if len(c.Config.Raw) > 0 {
			var cfg any
			if err := json.Unmarshal(c.Config.Raw, &cfg); err != nil {
				return "", err
			}
			cm["config"] = cfg
		}
		connectors = append(connectors, cm)
	}
	if len(connectors) > 0 {
		m["connectors"] = connectors
	}
	if len(dex.StaticClients) > 0 {
		var sc []any
		for _, c := range dex.StaticClients {
			var item any
			_ = json.Unmarshal(c.Raw, &item)
			sc = append(sc, item)
		}
		m["staticClients"] = sc
	}
	b, err := yaml.Marshal(m)
	return string(b), err
}

func marshalOIDC(o *argov1alpha1.OIDCConfig) (string, error) {
	m := map[string]any{}
	setStr := func(k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	setStr("name", o.Name)
	setStr("issuer", o.IssuerURL)
	setStr("clientID", o.ClientID)
	if o.ClientSecretRef != nil {
		s, err := secretRefToDollar(o.ClientSecretRef)
		if err != nil {
			return "", err
		}
		setStr("clientSecret", s)
	}
	setStr("cliClientID", o.CLIClientID)
	if ui := o.UserInfo; ui != nil {
		uim := map[string]any{}
		if ui.GroupsEnabled {
			uim["groupsEnabled"] = true
		}
		setStrMap := func(k, v string) {
			if v != "" {
				uim[k] = v
			}
		}
		setStrMap("baseURL", ui.BaseURL)
		setStrMap("path", ui.Path)
		if s := durationString(ui.CacheExpiration); s != "" {
			uim["cacheExpiration"] = s
		}
		if len(uim) > 0 {
			m["userInfo"] = uim
		}
	}
	setStr("logoutURL", o.LogoutURL)
	setStr("rootCA", o.RootCA)
	setStr("domainHint", o.DomainHint)
	setStr("refreshTokenThreshold", durationString(o.RefreshTokenThreshold))
	if o.PKCEAuthenticationEnabled {
		m["enablePKCEAuthentication"] = true
	}
	if o.SkipAudienceCheckWhenTokenHasNoAudience {
		m["skipAudienceCheckWhenTokenHasNoAudience"] = true
	}
	if len(o.RequestedScopes) > 0 {
		m["requestedScopes"] = o.RequestedScopes
	}
	if len(o.AllowedAudiences) > 0 {
		m["allowedAudiences"] = o.AllowedAudiences
	}
	if len(o.RequestedIDTokenClaims) > 0 {
		claims := map[string]any{}
		for k, c := range o.RequestedIDTokenClaims {
			cm := map[string]any{}
			if c.Essential {
				cm["essential"] = true
			}
			if c.Value != "" {
				cm["value"] = c.Value
			}
			if len(c.Values) > 0 {
				cm["values"] = c.Values
			}
			claims[k] = cm
		}
		m["requestedIDTokenClaims"] = claims
	}
	if o.Azure != nil {
		az := map[string]any{}
		if o.Azure.UseWorkloadIdentity {
			az["useWorkloadIdentity"] = true
		}
		if o.Azure.GraphAPIEndpointURL != "" {
			az["graphApiEndpoint"] = o.Azure.GraphAPIEndpointURL
		}
		if ug := o.Azure.UserGroupOverageClaim; ug != nil {
			uom := map[string]any{}
			if ug.Enabled {
				uom["enabled"] = true
			}
			if s := durationString(ug.CacheExpiration); s != "" {
				uom["cacheExpiration"] = s
			}
			if len(uom) > 0 {
				az["userGroupOverageClaim"] = uom
			}
		}
		m["azure"] = az
	}
	b, err := yaml.Marshal(m)
	return string(b), err
}

func unmapResource(r *argov1alpha1.ResourceConfig, data map[string]string) error {
	if r == nil {
		return nil
	}
	if len(r.Exclusions) > 0 {
		b, err := yaml.Marshal(r.Exclusions)
		if err != nil {
			return err
		}
		data["resource.exclusions"] = string(b)
	}
	if len(r.Inclusions) > 0 {
		b, err := yaml.Marshal(r.Inclusions)
		if err != nil {
			return err
		}
		data["resource.inclusions"] = string(b)
	}
	if r.CompareOptions != nil {
		m := map[string]any{}
		if r.CompareOptions.IgnoreAggregatedRoles {
			m["ignoreAggregatedRoles"] = true
		}
		if r.CompareOptions.IgnoreResourceStatusField != "" {
			m["ignoreResourceStatusField"] = r.CompareOptions.IgnoreResourceStatusField
		}
		if r.CompareOptions.IgnoreDifferencesOnResourceUpdates {
			m["ignoreDifferencesOnResourceUpdates"] = true
		}
		b, err := yaml.Marshal(m)
		if err != nil {
			return err
		}
		data["resource.compareoptions"] = string(b)
	}
	if r.RespectRBAC != "" {
		data["resource.respectRBAC"] = r.RespectRBAC
	}
	if r.IgnoreResourceUpdatesEnabled != nil {
		data["resource.ignoreResourceUpdatesEnabled"] = strconv.FormatBool(*r.IgnoreResourceUpdatesEnabled)
	}
	for _, c := range r.Customizations {
		gk := c.Group + "/" + c.Kind
		if c.Group == "*" && c.Kind == "*" {
			gk = "all"
		} else if c.Group == "" {
			gk = c.Kind
		}
		gkKey := strings.ReplaceAll(gk, "/", "_")
		if c.HealthLua != "" {
			data["resource.customizations.health."+gkKey] = c.HealthLua
		}
		if c.UseOpenLibs {
			data["resource.customizations.useOpenLibs."+gkKey] = "true"
		}
		if c.Actions != nil {
			b, err := marshalResourceActions(c.Actions)
			if err != nil {
				return err
			}
			if b != "" {
				data["resource.customizations.actions."+gkKey] = b
			}
		}
		if c.IgnoreDifferences != nil {
			b, err := yaml.Marshal(c.IgnoreDifferences)
			if err != nil {
				return err
			}
			data["resource.customizations.ignoreDifferences."+gkKey] = string(b)
		}
		if c.IgnoreResourceUpdates != nil {
			b, err := yaml.Marshal(c.IgnoreResourceUpdates)
			if err != nil {
				return err
			}
			data["resource.customizations.ignoreResourceUpdates."+gkKey] = string(b)
		}
		if len(c.KnownTypeFields) > 0 {
			b, err := yaml.Marshal(c.KnownTypeFields)
			if err != nil {
				return err
			}
			data["resource.customizations.knownTypeFields."+gkKey] = string(b)
		}
	}
	if len(r.CustomLabelKeys) > 0 {
		data["resource.customLabels"] = strings.Join(r.CustomLabelKeys, ",")
	}
	if len(r.SensitiveMaskAnnotationKeys) > 0 {
		data["resource.sensitive.mask.annotations"] = strings.Join(r.SensitiveMaskAnnotationKeys, ",")
	}
	if el := r.EventLabels; el != nil {
		if len(el.IncludeKeyGlobs) > 0 {
			data["resource.includeEventLabelKeys"] = strings.Join(el.IncludeKeyGlobs, ",")
		}
		if len(el.ExcludeKeyGlobs) > 0 {
			data["resource.excludeEventLabelKeys"] = strings.Join(el.ExcludeKeyGlobs, ",")
		}
	}
	return nil
}

func unmapApplication(c *argov1alpha1.ControllerConfig, data map[string]string) {
	if c == nil {
		return
	}
	if c.InstanceLabelKey != "" {
		data["application.instanceLabelKey"] = c.InstanceLabelKey
	}
	if c.ResourceTrackingMethod != "" {
		data["application.resourceTrackingMethod"] = c.ResourceTrackingMethod
	}
	if len(c.AllowedNodeLabelKeys) > 0 {
		data["application.allowedNodeLabels"] = strings.Join(c.AllowedNodeLabelKeys, ",")
	}
	if c.Sync != nil {
		if imp := c.Sync.Impersonation; imp != nil {
			if imp.Enabled != nil {
				data["application.sync.impersonation.enabled"] = strconv.FormatBool(*imp.Enabled)
			}
			if imp.Enforced != nil {
				data["application.sync.impersonation.enforced"] = strconv.FormatBool(*imp.Enforced)
			}
		}
		if c.Sync.RequireOverridePrivilegeForRevisionSync != nil {
			data["application.sync.requireOverridePrivilegeForRevisionSync"] = strconv.FormatBool(*c.Sync.RequireOverridePrivilegeForRevisionSync)
		}
	}
}

func unmapUI(ui *argov1alpha1.UIConfig, data map[string]string) {
	if ui == nil {
		return
	}
	if ui.CSSURL != "" {
		data["ui.cssurl"] = ui.CSSURL
	}
	if b := ui.Banner; b != nil {
		if b.Content != "" {
			data["ui.bannercontent"] = b.Content
		}
		if b.URL != "" {
			data["ui.bannerurl"] = b.URL
		}
		if b.Permanent != nil {
			data["ui.bannerpermanent"] = strconv.FormatBool(*b.Permanent)
		}
		if b.Position != "" {
			data["ui.bannerposition"] = b.Position
		}
	}
	if ui.LoginButtonText != "" {
		data["ui.loginButtonText"] = ui.LoginButtonText
	}
}

func unmapUsers(u *argov1alpha1.UsersConfig, data map[string]string) {
	if u == nil {
		return
	}
	if u.AnonymousEnabled != nil {
		data["users.anonymous.enabled"] = strconv.FormatBool(*u.AnonymousEnabled)
	}
	if s := durationString(u.SessionDuration); s != "" {
		data["users.session.duration"] = s
	}
	if u.PasswordRegex != "" {
		data["passwordPattern"] = u.PasswordRegex
	}
}

func unmapHelp(h *argov1alpha1.HelpConfig, data map[string]string) {
	if h == nil {
		return
	}
	if ch := h.Chat; ch != nil {
		if ch.URL != "" {
			data["help.chatUrl"] = ch.URL
		}
		if ch.Text != "" {
			data["help.chatText"] = ch.Text
		}
	}
	for arch, url := range h.BinaryURLs {
		data["help.download."+arch] = url
	}
}

func unmapAccounts(accounts []argov1alpha1.AccountConfig, data map[string]string) {
	for _, a := range accounts {
		if a.Name == "admin" {
			data["admin.enabled"] = strconv.FormatBool(a.Enabled)
			continue
		}
		if len(a.Capabilities) > 0 {
			data["accounts."+a.Name] = strings.Join(a.Capabilities, ",")
		}
		data["accounts."+a.Name+".enabled"] = strconv.FormatBool(a.Enabled)
	}
}

func unmapExtensions(exts []argov1alpha1.ExtensionConfig, data map[string]string) error {
	if len(exts) == 0 {
		return nil
	}
	type wrapExt struct {
		Name    string         `yaml:"name"`
		Backend map[string]any `yaml:"backend"`
	}
	var list []wrapExt
	for _, e := range exts {
		be, err := extensionBackendToMap(e.Backend)
		if err != nil {
			return err
		}
		list = append(list, wrapExt{Name: e.Name, Backend: be})
	}
	b, err := yaml.Marshal(map[string]any{"extensions": list})
	if err != nil {
		return err
	}
	data["extension.config"] = string(b)
	return nil
}

func extensionBackendToMap(b argov1alpha1.ExtensionBackend) (map[string]any, error) {
	m := map[string]any{}
	if tr := b.Transport; tr != nil {
		tm := map[string]any{}
		if s := durationString(tr.ConnectionTimeout); s != "" {
			tm["connectionTimeout"] = s
		}
		if s := durationString(tr.KeepAlive); s != "" {
			tm["keepAlive"] = s
		}
		if s := durationString(tr.IdleConnectionTimeout); s != "" {
			tm["idleConnectionTimeout"] = s
		}
		if tr.MaxIdleConnections != 0 {
			tm["maxIdleConnections"] = tr.MaxIdleConnections
		}
		if len(tm) > 0 {
			m["transport"] = tm
		}
	}
	var svcs []any
	for _, s := range b.Services {
		sm := map[string]any{"url": s.URL}
		if s.Cluster != nil {
			cm := map[string]any{}
			if s.Cluster.ServerURL != "" {
				cm["server"] = s.Cluster.ServerURL
			}
			if s.Cluster.Name != "" {
				cm["name"] = s.Cluster.Name
			}
			sm["cluster"] = cm
		}
		if len(s.Headers) > 0 {
			var hdrs []any
			for _, h := range s.Headers {
				hdrs = append(hdrs, map[string]any{"name": h.Name, "value": h.Value})
			}
			sm["headers"] = hdrs
		}
		svcs = append(svcs, sm)
	}
	if len(svcs) > 0 {
		m["services"] = svcs
	}
	return m, nil
}

func unmapDeepLinks(dl *argov1alpha1.DeepLinksConfig, data map[string]string) error {
	if dl == nil {
		return nil
	}
	write := func(key string, links []argov1alpha1.DeepLink) error {
		if len(links) == 0 {
			return nil
		}
		var raw []map[string]any
		for _, l := range links {
			m := map[string]any{"url": l.URLTemplate, "title": l.Title}
			if l.Description != "" {
				m["description"] = l.Description
			}
			if l.IconClass != "" {
				m["icon.class"] = l.IconClass
			}
			if l.ConditionExpr != "" {
				m["if"] = l.ConditionExpr
			}
			raw = append(raw, m)
		}
		b, err := yaml.Marshal(raw)
		if err != nil {
			return err
		}
		data[key] = string(b)
		return nil
	}
	if err := write("application.links", dl.Application); err != nil {
		return err
	}
	if err := write("project.links", dl.Project); err != nil {
		return err
	}
	return write("resource.links", dl.Resource)
}

func unmapKustomize(k *argov1alpha1.KustomizeConfig, data map[string]string) {
	if k == nil {
		return
	}
	if k.Enabled != nil {
		data["kustomize.enable"] = strconv.FormatBool(*k.Enabled)
	}
	if k.BuildOptions != "" {
		data["kustomize.buildOptions"] = k.BuildOptions
	}
	for _, v := range k.Versions {
		if v.Path != "" {
			data["kustomize.path."+v.Name] = v.Path
		}
		if v.BuildOptions != "" {
			data["kustomize.buildOptions."+v.Name] = v.BuildOptions
		}
	}
}

func unmapHelm(h *argov1alpha1.HelmConfig, data map[string]string) {
	if h == nil {
		return
	}
	if h.Enabled != nil {
		data["helm.enable"] = strconv.FormatBool(*h.Enabled)
	}
	if len(h.ValuesFileSchemes) > 0 {
		data["helm.valuesFileSchemes"] = strings.Join(h.ValuesFileSchemes, ",")
	}
}

func unmapJsonnet(j *argov1alpha1.JsonnetConfig, data map[string]string) {
	if j == nil {
		return
	}
	if j.Enabled != nil {
		data["jsonnet.enable"] = strconv.FormatBool(*j.Enabled)
	}
}

func unmapServerMisc(s *argov1alpha1.ServerConfig, data map[string]string) {
	if s.MaxPodLogsToRender != nil {
		data["server.maxPodLogsToRender"] = strconv.FormatInt(*s.MaxPodLogsToRender, 10)
	}
	if ga := s.GoogleAnalytics; ga != nil {
		if ga.TrackingID != "" {
			data["ga.trackingid"] = ga.TrackingID
		}
		data["ga.anonymizeusers"] = strconv.FormatBool(ga.AnonymizeUsers)
	}
	if e := s.Exec; e != nil {
		data["exec.enabled"] = strconv.FormatBool(e.Enabled)
		if len(e.Shells) > 0 {
			data["exec.shells"] = strings.Join(e.Shells, ",")
		}
	}
	if sb := s.StatusBadge; sb != nil {
		data["statusbadge.enabled"] = strconv.FormatBool(sb.Enabled)
		if sb.URL != "" {
			data["statusbadge.url"] = sb.URL
		}
	}
	if w := s.Webhook; w != nil {
		if w.MaxPayloadSize != nil {
			mb := int(w.MaxPayloadSize.Value() / (1000 * 1000))
			if mb > 0 {
				data["webhook.maxPayloadSizeMB"] = strconv.Itoa(mb)
			}
		}
		if r := w.Refresh; r != nil {
			if s := durationString(r.Jitter); s != "" {
				data["webhook.refresh.jitter"] = s
			}
			if r.JitterThreshold != nil {
				data["webhook.refresh.jitter.threshold"] = strconv.Itoa(int(*r.JitterThreshold))
			}
		}
	}
	if s.RBAC != nil && s.RBAC.ApplicationFineGrainedInheritanceDisabled != nil {
		data["server.rbac.disableApplicationFineGrainedRBACInheritance"] = strconv.FormatBool(*s.RBAC.ApplicationFineGrainedInheritanceDisabled)
	}
}

func unmapTopMisc(spec *argov1alpha1.ArgoCDConfigurationSpec, data map[string]string) {
	if cl := spec.Cluster; cl != nil && cl.InClusterEnabled != nil {
		data["cluster.inClusterEnabled"] = strconv.FormatBool(*cl.InClusterEnabled)
	}
	if c := spec.Controller; c != nil {
		if sh := c.SourceHydrator; sh != nil {
			if sh.CommitMessageTemplate != "" {
				data["sourceHydrator.commitMessageTemplate"] = sh.CommitMessageTemplate
			}
			if sh.ReadmeMessageTemplate != "" {
				data["sourceHydrator.readmeMessageTemplate"] = sh.ReadmeMessageTemplate
			}
		}
		if rec := c.Reconciliation; rec != nil {
			if rec.Timeout != nil {
				data["timeout.reconciliation"] = rec.Timeout.Duration.String()
			}
			if rec.Jitter != nil {
				data["timeout.reconciliation.jitter"] = rec.Jitter.Duration.String()
			}
		}
	}
	if spec.InstallationID != "" {
		data["installationID"] = spec.InstallationID
	}
	if cs := spec.CommitServer; cs != nil {
		if c := cs.Commit; c != nil && c.Author != nil {
			if c.Author.Name != "" {
				data["commit.author.name"] = c.Author.Name
			}
			if c.Author.Email != "" {
				data["commit.author.email"] = c.Author.Email
			}
		}
	}
}

func unmapCmdParams(spec *argov1alpha1.ArgoCDConfigurationSpec, data map[string]string, diag *Diagnostics) error {
	if len(spec.ApplicationNamespaceGlobs) > 0 {
		data["application.namespaces"] = strings.Join(spec.ApplicationNamespaceGlobs, ",")
	}
	if r := spec.RepoServer; r != nil {
		if r.Address != "" {
			data["repo.server"] = r.Address
		}
		unmapRepoServerClient(r.Client, data, diag)
		if p := r.Plugin; p != nil && len(p.TarExclusionGlobs) > 0 {
			data["reposerver.plugin.tar.exclusions"] = strings.Join(p.TarExclusionGlobs, ";")
		}
		if r.LogFormat != "" {
			data["reposerver.log.format"] = r.LogFormat
		}
		if r.LogLevel != "" {
			data["reposerver.log.level"] = r.LogLevel
		}
	}
	if cs := spec.CommitServer; cs != nil {
		if cs.Address != "" {
			data["commit.server"] = cs.Address
		}
		if cs.LogFormat != "" {
			data["commitserver.log.format"] = cs.LogFormat
		}
		if cs.LogLevel != "" {
			data["commitserver.log.level"] = cs.LogLevel
		}
	}
	if l := spec.Logging; l != nil && l.FormatTimestamp != "" {
		data["log.format.timestamp"] = l.FormatTimestamp
	}
	unmapRedis(spec.Redis, data)
	unmapOTLP(spec.OTLP, data)

	unmapControllerCmdParams(spec.Controller, data)
	if s := spec.Server; s != nil {
		if s.TLSDisabled != nil {
			data["server.insecure"] = strconv.FormatBool(*s.TLSDisabled)
		}
		if s.LogFormat != "" {
			data["server.log.format"] = s.LogFormat
		}
		if s.LogLevel != "" {
			data["server.log.level"] = s.LogLevel
		}
		unmapServerCmdParams(s, data)
	}
	unmapRepoServerCmdParams(spec.RepoServer, data)
	unmapCommitServerCmdParams(spec.CommitServer, data)
	unmapDexServerCmdParams(spec.DexServer, data)
	unmapNotificationsCmdParams(spec.Notifications, data)
	if a := spec.ApplicationSet; a != nil {
		if len(a.NamespaceGlobs) > 0 {
			data["applicationsetcontroller.namespaces"] = strings.Join(a.NamespaceGlobs, ",")
		}
		if a.Policy != "" {
			data["applicationsetcontroller.policy"] = a.Policy
		}
		unmapApplicationSetCmdParams(a, data)
	}
	return nil
}

func unmapRedis(r *argov1alpha1.RedisConfig, data map[string]string) {
	if r == nil {
		return
	}
	if r.Server != "" {
		data["redis.server"] = r.Server
	}
	if r.Compression != "" {
		data["redis.compression"] = r.Compression
	}
	if s := r.Sentinel; s != nil {
		if len(s.Hosts) > 0 {
			data["redis.sentinel.hosts"] = strings.Join(s.Hosts, ",")
		}
		if s.Master != "" {
			data["redis.sentinel.master"] = s.Master
		}
	}
	if r.DB != "" {
		data["redis.db"] = r.DB
	}
}

func unmapOTLP(o *argov1alpha1.OTLPConfig, data map[string]string) {
	if o == nil {
		return
	}
	if o.Address != "" {
		data["otlp.address"] = o.Address
	}
	if o.TLSEnabled != nil {
		data["otlp.insecure"] = strconv.FormatBool(!*o.TLSEnabled)
	}
	if len(o.Headers) > 0 {
		data["otlp.headers"] = joinKVMap(o.Headers, "=", ",")
	}
	if len(o.Attrs) > 0 {
		data["otlp.attrs"] = joinKVMap(o.Attrs, ":", ",")
	}
	if o.SampleRatio != "" {
		data["otlp.sample.ratio"] = o.SampleRatio
	}
}

func unmapRepoServerClient(c *argov1alpha1.RepoServerClientConfig, data map[string]string, diag *Diagnostics) {
	if c == nil {
		return
	}
	set := func(suffix, v string) {
		if v == "" {
			return
		}
		ctrlKey := "controller.repo.server." + suffix
		srvKey := "server.repo.server." + suffix
		if diag != nil {
			diag.Warn(DirCRToCM, ctrlKey, "writing same value to controller.repo.server.* and server.repo.server.* keys")
		}
		data[ctrlKey] = v
		data[srvKey] = v
	}
	if c.Timeout != nil {
		set("timeout.seconds", strconv.Itoa(int(c.Timeout.Duration.Seconds())))
	}
	if c.TLSDisabled != nil {
		set("plaintext", strconv.FormatBool(*c.TLSDisabled))
	}
	if c.InsecureSkipVerify != nil {
		set("strict.tls", strconv.FormatBool(!*c.InsecureSkipVerify))
	}
	if mtls := c.MTLS; mtls != nil {
		set("ca.cert.path", mtls.CACertPath)
		set("client.cert.path", mtls.ClientCertPath)
		set("client.cert.key.path", mtls.ClientCertKeyPath)
	}
}

func unmapRBAC(spec *argov1alpha1.ArgoCDConfigurationSpec, data map[string]string) {
	if spec.Server == nil || spec.Server.RBAC == nil {
		return
	}
	r := spec.Server.RBAC
	if r.Default != "" {
		data["policy.default"] = r.Default
	}
	if len(r.Scopes) > 0 {
		b, _ := yaml.Marshal(r.Scopes)
		data["scopes"] = strings.TrimSpace(string(b))
	}
	if r.MatchMode != "" {
		data["policy.matchMode"] = r.MatchMode
	}
	if r.PolicyCSV != "" {
		data["policy.csv"] = r.PolicyCSV
	}
	for _, o := range r.PolicyOverlays {
		data["policy."+o.Name+".csv"] = o.CSV
	}
}

func ensureCommitAuthor(cs *argov1alpha1.CommitServerConfig) *argov1alpha1.CommitAuthor {
	if cs.Commit == nil {
		cs.Commit = &argov1alpha1.CommitConfig{}
	}
	if cs.Commit.Author == nil {
		cs.Commit.Author = &argov1alpha1.CommitAuthor{}
	}
	return cs.Commit.Author
}

func parseDurationPtr(diag *Diagnostics, key, s string) (*metav1.Duration, error) {
	if s == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		if diag != nil && key != "" {
			diag.Error(DirCMToCR, key, fmt.Sprintf("invalid duration %q: %v", s, err))
		}
		return nil, err
	}
	return &metav1.Duration{Duration: d}, nil
}

func durationString(d *metav1.Duration) string {
	if d == nil {
		return ""
	}
	return d.Duration.String()
}

const defaultArgoCDSecretName = "argocd-secret"

func dollarSecretToRef(s string) (*corev1.SecretKeySelector, error) {
	if !strings.HasPrefix(s, "$") || strings.Contains(s[1:], "$") {
		return nil, fmt.Errorf("oidc clientSecret must be a $key ref to argocd-secret (got %q); raw secrets are not allowed in the CR", s)
	}
	key := strings.TrimPrefix(s, "$")
	if key == "" || strings.Contains(key, " ") {
		return nil, fmt.Errorf("invalid $string secret ref %q", s)
	}
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: defaultArgoCDSecretName},
		Key:                  key,
	}, nil
}

func secretRefToDollar(ref *corev1.SecretKeySelector) (string, error) {
	if ref == nil || ref.Key == "" {
		return "", fmt.Errorf("empty clientSecretRef")
	}
	name := ref.Name
	if name == "" || name == defaultArgoCDSecretName {
		return "$" + ref.Key, nil
	}
	return "", fmt.Errorf("clientSecretRef secret %q cannot be represented in argocd-cm oidc.config (only %s keys via $string are supported)", name, defaultArgoCDSecretName)
}

func strFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func asInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case string:
		n, err := strconv.Atoi(t)
		return n, err == nil
	default:
		return 0, false
	}
}

func splitCSV(s string) []string {
	return splitSep(s, ",")
}

func splitSep(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseKVMap(s, kvSep, listSep string) map[string]string {
	out := map[string]string{}
	for _, part := range splitSep(s, listSep) {
		k, v, ok := strings.Cut(part, kvSep)
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func joinKVMap(m map[string]string, kvSep, listSep string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+kvSep+m[k])
	}
	return strings.Join(parts, listSep)
}
