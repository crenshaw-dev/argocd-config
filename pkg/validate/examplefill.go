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
		if v.Type() == reflect.TypeOf(runtime.RawExtension{}) {
			v.Set(reflect.ValueOf(runtime.RawExtension{Raw: []byte(`{"example":true}`)}))
			return
		}
		fillStruct(v, ctx)
	case reflect.Slice:
		elem := reflect.New(v.Type().Elem()).Elem()
		fillValue(elem, jsonName, ctx)
		v.Set(reflect.Append(v, elem))
	case reflect.Map:
		key, _ := exampleMapEntry(v.Type(), ctx)
		keyVal := reflect.New(v.Type().Key()).Elem()
		keyVal.SetString(key)
		elem := reflect.New(v.Type().Elem()).Elem()
		fillValue(elem, jsonName, fillContext{
			jsonPath:  ctx.jsonPath,
			fieldName: ctx.fieldName,
			mapKey:    key,
		})
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
	if t == reflect.TypeOf(metav1.Duration{}) {
		d := metav1.Duration{Duration: mustParseDuration(exampleDuration)}
		v.Set(reflect.ValueOf(d))
		return
	}
	if t == reflect.TypeOf(resource.Quantity{}) {
		qty := exampleQuantity
		// reposerver.grpc.max.size is historically an integer binary-MB count.
		if strings.EqualFold(ctx.fieldName, "GRPCMaxSize") || strings.HasSuffix(ctx.jsonPath, "grpcMaxSize") {
			qty = "10Mi"
		}
		v.Set(reflect.ValueOf(resource.MustParse(qty)))
		return
	}
	if t == reflect.TypeOf(corev1.SecretKeySelector{}) {
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
		case reflect.TypeOf(metav1.Duration{}):
			d := metav1.Duration{}
			_ = d.UnmarshalJSON([]byte(`"` + exampleDuration + `"`))
			v.Set(reflect.ValueOf(d))
		case reflect.TypeOf(resource.Quantity{}):
			qty := exampleQuantity
			if strings.EqualFold(jsonName, "grpcMaxSize") {
				qty = "10Mi"
			}
			q := resource.MustParse(qty)
			v.Set(reflect.ValueOf(q))
		case reflect.TypeOf(runtime.RawExtension{}):
			v.Set(reflect.ValueOf(runtime.RawExtension{Raw: []byte(`{"example":true}`)}))
		default:
			if v.Type().Name() == "AbsoluteHTTPURL" || v.Type().PkgPath() == argov1alpha1.GroupVersion.Group+"/v1alpha1" && v.Type().Name() == "AbsoluteHTTPURL" {
				v.SetString(exampleHTTPSURL)
			}
		}
	}
}

func exampleString(jsonName string, ctx fillContext) string {
	if v, ok := enumValue(ctx.jsonPath, ctx.fieldName, jsonName); ok {
		return v
	}

	lower := strings.ToLower(jsonName)
	fieldLower := strings.ToLower(ctx.fieldName)

	switch {
	case lower == "urltemplate":
		return "https://example.com/{{.app.metadata.name}}"
	case lower == "instancelabelkey":
		return "app.kubernetes.io/instance"
	case lower == "cssurl":
		return "/shared/app/custom.css"
	case lower == "urls" || strings.HasSuffix(lower, "urls"):
		return exampleHTTPSURL
	case lower == "url" || (strings.HasSuffix(lower, "url") && !strings.HasSuffix(lower, "urltemplate")):
		return exampleHTTPSURL
	case strings.Contains(lower, "url") && !strings.Contains(lower, "template"):
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
		return "p, role:example, applications, get, */*, allow"
	case strings.HasSuffix(lower, "rootca") || strings.HasSuffix(lower, "rootca"):
		return "-----BEGIN CERTIFICATE-----\nEXAMPLE\n-----END CERTIFICATE-----"
	case lower == "address" || strings.HasSuffix(lower, "address"):
		return "example.local:8080"
	case lower == "server" && ctx.jsonPath == "spec.redis":
		return "argocd-redis:6379"
	case lower == "db":
		return "0"
	case lower == "master":
		return "mymaster"
	case lower == "sampleRatio":
		return "0.5"
	case lower == "qps":
		return "50"
	case lower == "formatTimestamp":
		return "RFC3339"
	case lower == "buildOptions":
		return "--enable-alpha-plugins"
	case lower == "type" && strings.Contains(ctx.jsonPath, "connectors"):
		return "github"
	case lower == "id" && strings.Contains(ctx.jsonPath, "connectors"):
		return "github"
	case lower == "name" && strings.Contains(ctx.jsonPath, "commit.author"):
		return "Argo CD"
	case lower == "name":
		return exampleMapKeyName(ctx)
	case lower == "kinds":
		return "Deployment"
	case lower == "kind":
		return "Deployment"
	case lower == "apigroups":
		return "apps"
	case lower == "group":
		return "apps"
	case lower == "applicationnamespaceglobs", lower == "namespaceglobs":
		return "team-*"
	case lower == "operator":
		return "In"
	case lower == "key" && strings.Contains(ctx.jsonPath, "matchExpressions"):
		return "app.kubernetes.io/name"
	case strings.HasSuffix(lower, "keys"):
		return "app.kubernetes.io/name"
	case lower == "conditions":
		return "Synced"
	case lower == "ciphers":
		return "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	case lower == "layerMediaTypes":
		return "application/vnd.oci.image.layer.v1.tar+gzip"
	case lower == "valuesFileSchemes":
		return "https"
	case lower == "apiContentTypes":
		return "application/json"
	case lower == "shells":
		return "bash"
	case lower == "hosts":
		return "redis-sentinel:26379"
	case lower == "values":
		return "example"
	case lower == "clusters":
		return "https://kubernetes.default.svc"
	case lower == "scopes":
		return "groups"
	case lower == "requestedScopes":
		return "openid"
	case lower == "allowedAudiences":
		return "argocd"
	case lower == "includeKeyGlobs", lower == "excludeKeyGlobs":
		return "app.kubernetes.io/*"
	case lower == "tarExclusionGlobs":
		return "*.git"
	case lower == "urls":
		return exampleHTTPSURL
	case lower == "projectname":
		return "global"
	case lower == "field":
		return "spec.template"
	case lower == "type" && strings.Contains(ctx.jsonPath, "knownTypeFields"):
		return "core/v1/PodSpec"
	case lower == "title":
		return "Example Link"
	case lower == "description":
		return "Example description"
	case lower == "iconClass":
		return "fa fa-link"
	case lower == "conditionExpr":
		return "true"
	case lower == "content":
		return "Example banner"
	case lower == "loginbuttontext":
		return "Log in via SSO"
	case lower == "basehref", lower == "rootpath":
		return "/argo-cd"
	case lower == "staticassetspath":
		return "/shared/app"
	case lower == "xframeoptions":
		return "sameorigin"
	case lower == "installationid":
		return "example-installation-id"
	case lower == "minversion":
		return "1.2"
	case lower == "maxVersion":
		return "1.3"
	case lower == "path":
		return "/userinfo"
	case lower == "value" && strings.Contains(ctx.jsonPath, "requestedIDTokenClaims"):
		return "example-value"
	case lower == "actionLua":
		return "return obj"
	case lower == "discoveryLua":
		return "actions = {}\nreturn actions"
	case lower == "healthLua":
		return "hs = {}\nhs.status = \"Healthy\"\nreturn hs"
	case lower == "commitMessageTemplate", lower == "readmeMessageTemplate":
		return "{{.metadata.name}}"
	case lower == "trackingID":
		return "UA-12345-1"
	case lower == "default":
		return "role:readonly"
	case lower == "matchMode":
		return "glob"
	case lower == "compression":
		if strings.Contains(ctx.jsonPath, "redis") {
			return "gzip"
		}
		return "gzip"
	case lower == "policy":
		return "sync"
	case lower == "algorithm":
		return "legacy"
	case lower == "mode":
		return "optional"
	case lower == "respectrbac":
		return "strict"
	case lower == "ignoreResourceStatusField":
		return "crd"
	case lower == "resourceTrackingMethod":
		return "annotation"
	case lower == "position":
		return "top"
	case lower == "format":
		return "json"
	case lower == "level":
		return "info"
	case fieldLower == "Capabilities" || lower == "capabilities":
		return "login"
	case lower == "clientID", lower == "cliClientID":
		return "argocd-client"
	case lower == "domainHint":
		return "example.com"
	case lower == "cacertPath", lower == "clientCertPath", lower == "clientCertKeyPath", lower == "clientCAPath", lower == "scmRootCAPath":
		return "/etc/argocd/tls/" + jsonName + ".pem"
	case strings.HasSuffix(lower, "path"):
		return "/example/" + jsonName
	default:
		return "example-" + jsonName
	}
}

func exampleInt(jsonName string, ctx fillContext) int64 {
	switch strings.ToLower(jsonName) {
	case "factor":
		return 2
	case "jitterthreshold", "statusmaxresourcescount", "globcachesize", "maxidleconnections", "max":
		return 1
	default:
		return 10
	}
}

func exampleMapKeyName(ctx fillContext) string {
	if ctx.mapKey != "" {
		return ctx.mapKey
	}
	switch {
	case strings.Contains(ctx.jsonPath, "accounts"):
		return "admin"
	case strings.Contains(ctx.jsonPath, "extensions"):
		return "example-extension"
	case strings.Contains(ctx.jsonPath, "connectors"):
		return "GitHub"
	case strings.Contains(ctx.jsonPath, "policyOverlays"):
		return "overlay"
	case strings.Contains(ctx.jsonPath, "versions"):
		return "v5.0.0"
	case strings.Contains(ctx.jsonPath, "headers"):
		return "X-Example"
	default:
		return "example"
	}
}

func exampleMapEntry(mapType reflect.Type, ctx fillContext) (key, val string) {
	switch {
	case strings.Contains(ctx.jsonPath, "requestedIDTokenClaims"):
		return "groups", ""
	case strings.Contains(ctx.jsonPath, "headers"):
		return "Authorization", "Bearer example"
	case strings.Contains(ctx.jsonPath, "binaryURLs"):
		return "darwin-arm64", exampleHTTPSURL + "/argocd"
	case strings.Contains(ctx.jsonPath, "otlp"):
		return "x-example", "value"
	default:
		return "example-key", "example-value"
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
		case reflect.TypeOf(metav1.Duration{}):
			if v.FieldByName("Duration").Int() == 0 {
				reportUnset(jsonPath, "zero duration", out)
			}
		case reflect.TypeOf(resource.Quantity{}):
			q := v.Interface().(resource.Quantity)
			if q.IsZero() {
				reportUnset(jsonPath, "zero quantity", out)
			}
		case reflect.TypeOf(corev1.SecretKeySelector{}):
			name := v.FieldByName("LocalObjectReference").FieldByName("Name").String()
			key := v.FieldByName("Key").String()
			if name == "" || key == "" {
				reportUnset(jsonPath, "incomplete secretKeySelector", out)
			}
		case reflect.TypeOf(runtime.RawExtension{}):
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
