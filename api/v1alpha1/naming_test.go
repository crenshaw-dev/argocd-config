package v1alpha1

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
)

// commonInitialisms follows Go CodeReviewComments (and Kubernetes API conventions
// where they agree). JSON field names use these with a CRD-user-facing rule:
// leading initialisms are fully lowercased (grpcMaxSize, not gRPCMaxSize);
// non-leading initialisms stay all-caps (issuerURL, grpcTXTServiceConfigEnabled).
//
// When adding a field whose name contains a new acronym, add it here so the
// casing check catches regressions.
//
// See: https://go.dev/wiki/CodeReviewComments#initialisms
var commonInitialisms = map[string]bool{
	"API":   true,
	"DB":    true,
	"GRPC":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"JQ":    true,
	"OCI":   true,
	"OIDC":  true,
	"OTLP":  true,
	"PKCE":  true,
	"QPS":   true,
	"RBAC":  true,
	"SCM":   true,
	"SSA":   true,
	"SSL":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"TXT":   true,
	"UID":   true,
	"URI":   true,
	"URL":   true,
	"UUID":  true,
}

func TestExportedFieldInitialisms(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var bad []string
	checkType(t, reflect.TypeOf(ArgoCDConfiguration{}), "", seen, &bad)
	if len(bad) > 0 {
		t.Fatalf("JSON field initialism casing violations (CRD/YAML names — see CONTRIBUTING.md):\n  %s",
			strings.Join(bad, "\n  "))
	}
}

func checkType(t *testing.T, typ reflect.Type, jsonPath string, seen map[reflect.Type]bool, bad *[]string) {
	t.Helper()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	if seen[typ] {
		return
	}
	seen[typ] = true

	for i := 0; i < typ.NumField(); i++ {
		sf := typ.Field(i)
		if sf.PkgPath != "" {
			continue // unexported
		}
		jsonName, skip := jsonFieldName(sf)
		if skip {
			continue
		}
		path := jsonName
		if jsonPath != "" {
			path = jsonPath + "." + jsonName
		}
		if sugg := suggestJSONInitialismFix(jsonName); sugg != "" && sugg != jsonName {
			*bad = append(*bad, path+" → want "+sugg)
		}
		ft := sf.Type
		for ft.Kind() == reflect.Pointer || ft.Kind() == reflect.Slice || ft.Kind() == reflect.Array || ft.Kind() == reflect.Map {
			if ft.Kind() == reflect.Map {
				checkType(t, ft.Key(), path, seen, bad)
			}
			ft = ft.Elem()
		}
		if ft.PkgPath() == typ.PkgPath() || strings.HasPrefix(ft.PkgPath(), "github.com/crenshaw-dev/argocd-config/api/") {
			checkType(t, ft, path, seen, bad)
		}
	}
}

func jsonFieldName(sf reflect.StructField) (name string, skip bool) {
	tag := sf.Tag.Get("json")
	if tag == "-" {
		return "", true
	}
	if tag == "" {
		// Inline / no tag: not a serialized CRD field name.
		if sf.Anonymous {
			return "", true
		}
		return strings.ToLower(sf.Name[:1]) + sf.Name[1:], false
	}
	name = strings.Split(tag, ",")[0]
	if name == "" || name == "-" {
		return "", true
	}
	return name, false
}

// suggestJSONInitialismFix returns the correctly-cased JSON name when an
// initialism appears with wrong casing, or "" if the name is fine.
func suggestJSONInitialismFix(name string) string {
	words := splitJSONCamel(name)
	changed := false
	for i, w := range words {
		upper := strings.ToUpper(w)
		if !commonInitialisms[upper] {
			continue
		}
		var want string
		if i == 0 {
			// Leading initialism: fully lowercase in JSON (grpcMaxSize, tlsEnabled).
			want = strings.ToLower(upper)
		} else {
			// Non-leading: all-caps (issuerURL, grpcTXTServiceConfigEnabled).
			want = upper
		}
		if w != want {
			words[i] = want
			changed = true
		}
	}
	if !changed {
		return ""
	}
	return strings.Join(words, "")
}

// splitJSONCamel splits a JSON camelCase name into words. Leading runes may be
// lowercase (grpcMaxSize → ["grpc","Max","Size"]).
func splitJSONCamel(name string) []string {
	if name == "" {
		return nil
	}
	var words []string
	runes := []rune(name)
	start := 0
	for i := 1; i < len(runes); i++ {
		prev, cur := runes[i-1], runes[i]
		if unicode.IsUpper(cur) {
			if !unicode.IsUpper(prev) {
				words = append(words, string(runes[start:i]))
				start = i
			} else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				words = append(words, string(runes[start:i]))
				start = i
			}
		}
	}
	words = append(words, string(runes[start:]))
	return words
}
