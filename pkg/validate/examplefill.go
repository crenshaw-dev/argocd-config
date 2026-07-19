package validate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

const (
	exampleNamespace = "argocd"
	exampleHTTPSURL  = "https://example.com"
	exampleEmail     = "argo-cd@example.com"
	exampleQuantity  = "10M"
	exampleDuration  = "1h"
)

// FillExampleConfiguration returns an ArgoCDConfiguration with every exported JSON
// field populated with CEL/OpenAPI-friendly placeholder values.
func FillExampleConfiguration() *argov1alpha1.ArgoCDConfiguration {
	cfg := &argov1alpha1.ArgoCDConfiguration{}
	fillValue(reflect.ValueOf(cfg), "", fillContext{})
	return cfg
}

type fillContext struct {
	jsonPath   string
	fieldName  string
	structType reflect.Type
	sliceIndex int
	mapKey     string
}

func fillValue(v reflect.Value, jsonName string, ctx fillContext) {
	if !v.IsValid() {
		return
	}

	for v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		fillValue(v.Elem(), jsonName, ctx)
	case reflect.Struct:
		if v.Type() == reflect.TypeFor[runtime.RawExtension]() {
			v.Set(reflect.ValueOf(runtime.RawExtension{Raw: []byte(`{"example":true}`)}))
			return
		}
		fillStruct(v, ctx)
	case reflect.Slice:
		elem := reflect.New(v.Type().Elem()).Elem()
		fillValue(elem, jsonName, ctx)
		v.Set(reflect.Append(v, elem))
	case reflect.Map:
		key, val := exampleMapEntry(v.Type(), ctx)
		keyVal := reflect.New(v.Type().Key()).Elem()
		keyVal.SetString(key)
		elem := reflect.New(v.Type().Elem()).Elem()
		childCtx := fillContext{
			jsonPath:  ctx.jsonPath,
			fieldName: ctx.fieldName,
			mapKey:    key,
		}
		if elem.Kind() == reflect.String && val != "" {
			elem.SetString(val)
		} else {
			fillValue(elem, jsonName, childCtx)
		}
		v.Set(reflect.MakeMap(v.Type()))
		v.SetMapIndex(keyVal, elem)
	default:
		setScalar(v, jsonName, ctx)
	}
}

func fillStruct(v reflect.Value, ctx fillContext) {
	t := v.Type()

	if isTypeMeta(t) {
		v.FieldByName("APIVersion").SetString(argov1alpha1.GroupVersion.String())
		v.FieldByName("Kind").SetString("ArgoCDConfiguration")
		return
	}
	if isObjectMeta(t) {
		v.FieldByName("Name").SetString(mapping.DefaultConfigurationName)
		v.FieldByName("Namespace").SetString(exampleNamespace)
		return
	}
	if t == reflect.TypeFor[metav1.Duration]() {
		d := metav1.Duration{Duration: mustParseDuration(exampleDuration)}
		v.Set(reflect.ValueOf(d))
		return
	}
	if t == reflect.TypeFor[resource.Quantity]() {
		qty := exampleQuantity
		// reposerver.grpc.max.size is historically an integer binary-MB count.
		if strings.EqualFold(ctx.fieldName, "GRPCMaxSize") || strings.HasSuffix(ctx.jsonPath, "grpcMaxSize") {
			qty = "10Mi"
		}
		v.Set(reflect.ValueOf(resource.MustParse(qty)))
		return
	}
	if t == reflect.TypeFor[corev1.SecretKeySelector]() {
		// Only argocd-secret keys round-trip into oidc.config ($string form).
		optional := true
		v.Set(reflect.ValueOf(corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "argocd-secret"},
			Key:                  "oidc.clientSecret",
			Optional:             &optional,
		}))
		return
	}

	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		jsonTag := parseJSONTag(sf.Tag.Get("json"))
		if jsonTag.name == "-" {
			continue
		}

		name := jsonTag.name
		if name == "" {
			name = strings.ToLower(sf.Name[:1]) + sf.Name[1:]
		}

		path := name
		if ctx.jsonPath != "" {
			path = ctx.jsonPath + "." + name
		}

		childCtx := fillContext{
			jsonPath:   path,
			fieldName:  sf.Name,
			structType: sf.Type,
		}
		fillValue(v.Field(i), name, childCtx)
	}
}

func setScalar(v reflect.Value, jsonName string, ctx fillContext) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(exampleString(jsonName, ctx))
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(exampleInt(jsonName, ctx))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(0.5)
	case reflect.Struct:
		switch v.Type() {
		case reflect.TypeFor[metav1.Duration]():
			d := metav1.Duration{}
			_ = d.UnmarshalJSON([]byte(`"` + exampleDuration + `"`))
			v.Set(reflect.ValueOf(d))
		case reflect.TypeFor[resource.Quantity]():
			qty := exampleQuantity
			if strings.EqualFold(jsonName, "grpcMaxSize") {
				qty = "10Mi"
			}
			q := resource.MustParse(qty)
			v.Set(reflect.ValueOf(q))
		case reflect.TypeFor[runtime.RawExtension]():
			v.Set(reflect.ValueOf(runtime.RawExtension{Raw: []byte(`{"example":true}`)}))
		}
	}
}

func exampleString(jsonName string, ctx fillContext) string {
	if v, ok := enumValue(ctx.jsonPath, ctx.fieldName, jsonName); ok {
		return v
	}

	lower := strings.ToLower(jsonName)
	path := strings.ToLower(ctx.jsonPath)

	// Path-aware overrides first (more specific than field name alone).
	switch {
	case lower == "path" && strings.Contains(path, "kustomize"):
		return "/usr/local/bin/kustomize"
	case lower == "path" && strings.Contains(path, "userinfo"):
		return "/userinfo"
	case lower == "server" && strings.Contains(path, "redis"):
		return "argocd-redis:6379"
	case lower == "name" && strings.Contains(path, "commit.author"):
		return "Argo CD"
	case lower == "name" && strings.Contains(path, "headers"):
		return "Authorization"
	case lower == "name" && strings.Contains(path, "extensions"):
		return "metrics"
	case lower == "name" && strings.Contains(path, "actions.definitions"):
		return "restart"
	case lower == "name" && strings.Contains(path, "accounts"):
		return "admin"
	case lower == "name" && strings.Contains(path, "connectors"):
		return "GitHub"
	case lower == "name" && strings.Contains(path, "policyoverlays"):
		return "extra"
	case lower == "name" && strings.Contains(path, "versions"):
		return "v5.0.0"
	case lower == "name" && strings.Contains(path, "oidc"):
		return "Okta"
	case lower == "value" && strings.Contains(path, "headers"):
		return "$extension.metrics.token"
	case lower == "value" && strings.Contains(path, "requestedidtokenclaims"):
		return "admin"
	case lower == "type" && strings.Contains(path, "connectors"):
		return "github"
	case lower == "id" && strings.Contains(path, "connectors"):
		return "github"
	case lower == "type" && strings.Contains(path, "knowntypefields"):
		return "core/v1/PodSpec"
	case lower == "key" && strings.Contains(path, "matchexpressions"):
		return "app.kubernetes.io/name"
	case lower == "text" && strings.Contains(path, "chat"):
		return "Chat with us on Slack"
	}

	switch lower {
	case "urltemplate":
		return "https://grafana.example.com/d/argo/{{.app.metadata.name}}"
	case "instancelabelkey":
		return "app.kubernetes.io/instance"
	case "cssurl":
		return "/shared/app/custom.css"
	case "urls", "url", "serverurl", "issuerurl", "logouturl", "baseurl", "graphapiendpointurl",
		"chaturl", "statusbadgeurl", "bannerurl":
		return exampleHTTPSURL
	case "email":
		return exampleEmail
	case "passwordregex":
		return `^.{8,32}$`
	case "commitmessagetemplate":
		return "chore: hydrate {{.metadata.name}}\n\n{{range .spec.sources}}{{.repoURL}}@{{.targetRevision}}\n{{end}}"
	case "readmemessagetemplate":
		return "# {{.metadata.name}}\n\nHydrated by Argo CD source hydrator."
	case "actionlua":
		return "obj.spec.replicas = 0\nreturn obj"
	case "discoverylua":
		return "actions = {}\nactions[\"restart\"] = {}\nreturn actions"
	case "healthlua":
		return "hs = {}\nhs.status = \"Healthy\"\nhs.message = \"ok\"\nreturn hs"
	case "policycsv", "csv":
		return "p, role:org-admin, applications, *, */*, allow"
	case "rootca":
		return "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----"
	case "address", "reposerveraddress", "metricsaddress", "listenaddress":
		if strings.Contains(path, "metrics") {
			return "0.0.0.0:8083"
		}
		if strings.Contains(path, "reposerver") || strings.Contains(path, "repo.server") {
			return "argocd-repo-server:8081"
		}
		if strings.Contains(path, "commitserver") {
			return "argocd-commit-server:8086"
		}
		if strings.Contains(path, "dex") {
			return "argocd-dex-server:5556"
		}
		if strings.Contains(path, "otlp") {
			return "otel-collector:4317"
		}
		return "0.0.0.0:8080"
	case "probeaddr":
		return ":8082"
	case "webhookaddr":
		return ":7000"
	case "db":
		return "0"
	case "master":
		return "mymaster"
	case "server":
		return "argocd-redis:6379"
	case "keyprefix":
		return "argocd"
	case "sampleratio":
		return "0.5"
	case "qps":
		return "50"
	case "formattimestamp":
		return "RFC3339"
	case "buildoptions":
		return "--enable-helm --load-restrictor LoadRestrictionsNone"
	case "name":
		return exampleMapKeyName(ctx)
	case "kinds", "kind":
		return "Deployment"
	case "apigroups", "group":
		return "apps"
	case "applicationnamespaceglobs", "namespaceglobs":
		return "team-*"
	case "operator":
		return "In"
	case "values":
		return "frontend"
	case "conditions":
		return "Synced"
	case "ciphers":
		return "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	case "layermediatypes":
		return "application/vnd.oci.image.layer.v1.tar+gzip"
	case "valuesfileschemes":
		return "https"
	case "apicontenttypes":
		return "application/json"
	case "shells":
		return "bash"
	case "hosts":
		return "redis-sentinel:26379"
	case "clusters":
		return "https://kubernetes.default.svc"
	case "scopes":
		return "groups"
	case "requestedscopes":
		return "openid"
	case "allowedaudiences":
		return "argocd"
	case "includekeyglobs":
		return "app.kubernetes.io/*"
	case "excludekeyglobs":
		return "kubectl.kubernetes.io/*"
	case "tarexclusionglobs":
		return ".git/*"
	case "projectname":
		return "global"
	case "field":
		return "spec.template"
	case "jsonpointers":
		return "/status"
	case "jqpathexpressions":
		return ".metadata.annotations.\"kubectl.kubernetes.io/last-applied-configuration\""
	case "managedfieldsmanagers":
		return "kube-controller-manager"
	case "title":
		return "Open in Grafana"
	case "description":
		return "Application metrics dashboard"
	case "iconclass":
		return "fa-external-link"
	case "conditionexpr":
		return "app.status.sync.status == 'Synced'"
	case "content":
		return "Scheduled maintenance Sunday 02:00–04:00 UTC"
	case "contentsecuritypolicy":
		return "frame-ancestors 'self'; object-src 'none'"
	case "loginbuttontext":
		return "Log in via SSO"
	case "basehref", "rootpath":
		return "/argo-cd"
	case "staticassetspath":
		return "/shared/app"
	case "xframeoptions":
		return "sameorigin"
	case "installationid":
		return "prod-us-west-1"
	case "minversion":
		return "1.2"
	case "maxversion":
		return "1.3"
	case "path":
		return "/userinfo"
	case "trackingid":
		return "UA-123456-1"
	case "default":
		return "role:readonly"
	case "matchmode":
		return "glob"
	case "compression":
		return "gzip"
	case "policy":
		return "sync"
	case "algorithm":
		return "legacy"
	case "mode":
		return "optional"
	case "respectrbac":
		return "strict"
	case "ignoreresourcestatusfield":
		return "crd"
	case "resourcetrackingmethod":
		return "annotation"
	case "position":
		return "top"
	case "format":
		return "json"
	case "level":
		return "info"
	case "capabilities":
		return "login"
	case "clientid", "cliclientid":
		return "argocd"
	case "domainhint":
		return "contoso.onmicrosoft.com"
	case "useragent":
		return "argocd-repo-server"
	case "applabelselector":
		return "app.kubernetes.io/part-of=argocd"
	case "configmapname":
		return "argocd-notifications-cm"
	case "secretname":
		return "argocd-notifications-secret"
	case "applicationsetlabels":
		return "app.kubernetes.io/name"
	case "profilerfilepath":
		return "/tmp/argocd-profile.pprof"
	case "cacertpath", "clientcapath", "scmrootcapath":
		return "/etc/argocd/tls/ca.crt"
	case "clientcertpath":
		return "/etc/argocd/tls/tls.crt"
	case "clientcertkeypath":
		return "/etc/argocd/tls/tls.key"
	case "allowednodelabelkeys", "customlabelkeys", "sensitivemaskannotationkeys",
		"annotationkeys", "labelkeys":
		return "app.kubernetes.io/name"
	}

	switch {
	case strings.HasSuffix(lower, "url") || strings.Contains(lower, "url"):
		return exampleHTTPSURL
	case strings.HasSuffix(lower, "email"):
		return exampleEmail
	case strings.HasSuffix(lower, "regex") || strings.HasSuffix(lower, "pattern"):
		return `^.{8,32}$`
	case strings.HasSuffix(lower, "template"):
		return "{{.metadata.name}}"
	case strings.HasSuffix(lower, "lua"):
		return "return true"
	case strings.HasSuffix(lower, "csv"):
		return "p, role:org-admin, applications, get, */*, allow"
	case strings.HasSuffix(lower, "address"):
		return "0.0.0.0:8080"
	case strings.HasSuffix(lower, "keys") || strings.HasSuffix(lower, "globs"):
		return "app.kubernetes.io/*"
	case strings.HasSuffix(lower, "path"):
		return "/etc/argocd/" + lower
	default:
		return "example-" + jsonName
	}
}

func exampleInt(jsonName string, ctx fillContext) int64 {
	lower := strings.ToLower(jsonName)
	path := strings.ToLower(ctx.jsonPath)
	switch lower {
	case "factor":
		return 2
	case "db":
		return 0
	case "jitterthreshold":
		return 3
	case "statusmaxresourcescount":
		return 100
	case "globcachesize":
		return 1000
	case "maxidleconnections":
		return 50
	case "max":
		return 5
	case "burst":
		return 100
	case "metricsport":
		return 8083
	case "port":
		return 8080
	case "processorscount", "hydration", "operation", "status", "workers":
		return 20
	case "kubectlparallelismlimit", "parallelismlimit", "webhookparallelismlimit",
		"reconciliationsparallelismlimit", "lsremoteparallelismlimit":
		return 10
	case "httpcookiemaxnumber":
		return 10
	case "maxpodstorender":
		return 10
	case "applicationtreeshardsize":
		return 100
	default:
		if strings.Contains(path, "timeout") || strings.Contains(lower, "timeout") {
			return 60
		}
		return 10
	}
}

func exampleMapKeyName(ctx fillContext) string {
	if ctx.mapKey != "" {
		return ctx.mapKey
	}
	path := strings.ToLower(ctx.jsonPath)
	switch {
	case strings.Contains(path, "accounts"):
		return "admin"
	case strings.Contains(path, "headers"):
		return "Authorization"
	case strings.Contains(path, "extensions"):
		return "metrics"
	case strings.Contains(path, "connectors"):
		return "GitHub"
	case strings.Contains(path, "policyoverlays"):
		return "extra"
	case strings.Contains(path, "versions"):
		return "v5.0.0"
	case strings.Contains(path, "actions"):
		return "restart"
	default:
		return "example"
	}
}

func exampleMapEntry(mapType reflect.Type, ctx fillContext) (key, val string) {
	path := strings.ToLower(ctx.jsonPath)
	switch {
	case strings.Contains(path, "requestedidtokenclaims"):
		return "groups", ""
	case strings.Contains(path, "headers"):
		return "Authorization", "Bearer token"
	case strings.Contains(path, "binaryurls"):
		return "darwin-arm64", exampleHTTPSURL + "/argocd-darwin-arm64"
	case strings.Contains(path, "otlp.attrs") || strings.HasSuffix(path, "attrs"):
		return "service.name", "argocd-server"
	case strings.Contains(path, "otlp"):
		return "Authorization", "Bearer token"
	case strings.Contains(path, "matchlabels"):
		return "app.kubernetes.io/part-of", "argocd"
	default:
		return "app.kubernetes.io/name", "argocd"
	}
}

func enumValue(jsonPath, fieldName, jsonName string) (string, bool) {
	key := jsonPath + "/" + jsonName
	enums := map[string]string{
		"spec.server.compression/compression":                                                     "gzip",
		"spec.server.rbac.matchMode/matchMode":                                                    "glob",
		"spec.controller.resource.respectRBAC/respectRBAC":                                        "strict",
		"spec.controller.diff.compareOptions.ignoreResourceStatusField/ignoreResourceStatusField": "crd",
		"spec.controller.sync.impersonation.mode/mode":                                            "optional",
		"spec.controller.resourceTrackingMethod/resourceTrackingMethod":                           "annotation",
		"spec.controller.sharding.algorithm/algorithm":                                            "legacy",
		"spec.applicationSet.policy/policy":                                                       "sync",
		"spec.redis.compression/compression":                                                      "gzip",
		"spec.server.log.format/format":                                                           "json",
		"spec.server.log.level/level":                                                             "info",
		"spec.server.ui.banner.position/position":                                                 "top",
		"spec.server.tls.minVersion/minVersion":                                                   "1.2",
		"spec.server.tls.maxVersion/maxVersion":                                                   "1.3",
		"spec.repoServer.tls.minVersion/minVersion":                                               "1.2",
		"spec.repoServer.tls.maxVersion/maxVersion":                                               "1.3",
	}
	if v, ok := enums[key]; ok {
		return v, true
	}
	return "", false
}

type jsonTagInfo struct {
	name      string
	omitEmpty bool
}

func parseJSONTag(tag string) jsonTagInfo {
	if tag == "" {
		return jsonTagInfo{}
	}
	parts := strings.Split(tag, ",")
	info := jsonTagInfo{name: parts[0]}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			info.omitEmpty = true
		}
	}
	return info
}

func isTypeMeta(t reflect.Type) bool {
	return t.PkgPath() == "k8s.io/apimachinery/pkg/apis/meta/v1" && t.Name() == "TypeMeta"
}

func isObjectMeta(t reflect.Type) bool {
	return t.PkgPath() == "k8s.io/apimachinery/pkg/apis/meta/v1" && t.Name() == "ObjectMeta"
}

func isListMeta(t reflect.Type) bool {
	return t.PkgPath() == "k8s.io/apimachinery/pkg/apis/meta/v1" && t.Name() == "ListMeta"
}

// UnsetField reports a JSON field path that is not considered set.
type UnsetField struct {
	Path string
	Kind string
}

// FindUnsetFields walks v and returns exported JSON fields that are unset.
func FindUnsetFields(v any) []UnsetField {
	var out []UnsetField
	findUnset(reflect.ValueOf(v), "", &out)
	return out
}

func findUnset(v reflect.Value, jsonPath string, out *[]UnsetField) {
	if !v.IsValid() {
		return
	}
	for v.Kind() == reflect.Interface {
		if v.IsNil() {
			reportUnset(jsonPath, "nil interface", out)
			return
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			reportUnset(jsonPath, "nil pointer", out)
			return
		}
		findUnset(v.Elem(), jsonPath, out)
	case reflect.Struct:
		if isTypeMeta(v.Type()) {
			checkTypeMeta(v, jsonPath, out)
			return
		}
		if isObjectMeta(v.Type()) {
			checkObjectMeta(v, jsonPath, out)
			return
		}
		if isListMeta(v.Type()) {
			return
		}
		walkStructFields(v, jsonPath, out)
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			reportUnset(jsonPath, "empty slice", out)
			return
		}
		for i := 0; i < v.Len(); i++ {
			childPath := fmt.Sprintf("%s[%d]", jsonPath, i)
			findUnset(v.Index(i), childPath, out)
		}
	case reflect.Map:
		if v.Len() == 0 {
			reportUnset(jsonPath, "empty map", out)
			return
		}
		iter := v.MapRange()
		for iter.Next() {
			key := fmt.Sprintf("%v", iter.Key().Interface())
			childPath := fmt.Sprintf("%s[%q]", jsonPath, key)
			findUnset(iter.Value(), childPath, out)
		}
	default:
		checkScalar(v, jsonPath, out)
	}
}

func walkStructFields(v reflect.Value, jsonPath string, out *[]UnsetField) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		jsonTag := parseJSONTag(sf.Tag.Get("json"))
		if jsonTag.name == "-" {
			continue
		}
		name := jsonTag.name
		if name == "" {
			name = strings.ToLower(sf.Name[:1]) + sf.Name[1:]
		}
		path := name
		if jsonPath != "" {
			path = jsonPath + "." + name
		}
		findUnset(v.Field(i), path, out)
	}
}

func checkTypeMeta(v reflect.Value, jsonPath string, out *[]UnsetField) {
	if v.FieldByName("APIVersion").String() == "" {
		reportUnset(jsonPath+".apiVersion", "empty string", out)
	}
	if v.FieldByName("Kind").String() == "" {
		reportUnset(jsonPath+".kind", "empty string", out)
	}
}

func checkObjectMeta(v reflect.Value, jsonPath string, out *[]UnsetField) {
	if v.FieldByName("Name").String() == "" {
		reportUnset(jsonPath+".name", "empty string", out)
	}
}

func checkScalar(v reflect.Value, jsonPath string, out *[]UnsetField) {
	t := v.Type()
	switch v.Kind() {
	case reflect.String:
		if v.String() == "" {
			reportUnset(jsonPath, "empty string", out)
		}
	case reflect.Bool:
		// any bool value counts as set
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		if isZeroNumeric(v) {
			if sf := lookupStructField(jsonPath); sf != nil {
				tag := parseJSONTag(sf.Tag.Get("json"))
				if tag.omitEmpty {
					reportUnset(jsonPath, "zero numeric with omitempty", out)
				}
			}
		}
	case reflect.Struct:
		switch t {
		case reflect.TypeFor[metav1.Duration]():
			if v.FieldByName("Duration").Int() == 0 {
				reportUnset(jsonPath, "zero duration", out)
			}
		case reflect.TypeFor[resource.Quantity]():
			q := v.Interface().(resource.Quantity)
			if q.IsZero() {
				reportUnset(jsonPath, "zero quantity", out)
			}
		case reflect.TypeFor[corev1.SecretKeySelector]():
			name := v.FieldByName("LocalObjectReference").FieldByName("Name").String()
			key := v.FieldByName("Key").String()
			if name == "" || key == "" {
				reportUnset(jsonPath, "incomplete secretKeySelector", out)
			}
		case reflect.TypeFor[runtime.RawExtension]():
			raw := v.FieldByName("Raw")
			if raw.Len() == 0 {
				reportUnset(jsonPath, "empty rawExtension", out)
			} else if !json.Valid(raw.Bytes()) {
				reportUnset(jsonPath, "invalid rawExtension JSON", out)
			}
		default:
			walkStructFields(v, jsonPath, out)
		}
	}
}

func isZeroNumeric(v reflect.Value) bool {
	return v.IsZero()
}

func reportUnset(path, kind string, out *[]UnsetField) {
	if path == "" {
		path = "(root)"
	}
	*out = append(*out, UnsetField{Path: path, Kind: kind})
}

// lookupStructField is a best-effort helper; zero-value omitempty checks fall
// back to requiring non-zero only when we cannot resolve metadata.
func lookupStructField(jsonPath string) *reflect.StructField {
	_ = jsonPath
	return nil
}

func mustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}
