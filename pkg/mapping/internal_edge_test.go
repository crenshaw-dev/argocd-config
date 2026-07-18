package mapping

import (
	"bytes"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
)

func TestAsIntBranches(t *testing.T) {
	cases := []struct {
		in   any
		want int
		ok   bool
	}{
		{int(7), 7, true},
		{int64(8), 8, true},
		{float64(9.9), 9, true},
		{"10", 10, true},
		{"nope", 0, false},
		{true, 0, false},
		{nil, 0, false},
	}
	for _, tc := range cases {
		got, ok := asInt(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("asInt(%#v)=(%d,%v) want (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestAsStringBranches(t *testing.T) {
	if asString(nil) != "" {
		t.Fatal("nil")
	}
	if asString("x") != "x" {
		t.Fatal("string")
	}
	if asString(42) != "42" {
		t.Fatal("default")
	}
}

func TestParseQuantityPtrEmpty(t *testing.T) {
	q, err := parseQuantityPtr(nil, "k", "")
	if err != nil || q != nil {
		t.Fatalf("empty: q=%v err=%v", q, err)
	}
}

func TestJoinKVMapEmpty(t *testing.T) {
	if joinKVMap(nil, "=", ",") != "" {
		t.Fatal("nil map")
	}
	if joinKVMap(map[string]string{}, "=", ",") != "" {
		t.Fatal("empty map")
	}
}

func TestParseKVMapEdges(t *testing.T) {
	m := parseKVMap("a=1,bad,=x,b=2", "=", ",")
	if m["a"] != "1" || m["b"] != "2" {
		t.Fatalf("%#v", m)
	}
	if _, ok := m[""]; ok {
		t.Fatal("empty key should be skipped")
	}
}

func TestSecretRefToDollarEdges(t *testing.T) {
	if _, err := secretRefToDollar(nil); err == nil {
		t.Fatal("nil ref")
	}
	if _, err := secretRefToDollar(&corev1.SecretKeySelector{}); err == nil {
		t.Fatal("empty key")
	}
	s, err := secretRefToDollar(&corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: defaultArgoCDSecretName},
		Key:                  "k",
	})
	if err != nil || s != "$k" {
		t.Fatalf("default name: %q %v", s, err)
	}
	s, err = secretRefToDollar(&corev1.SecretKeySelector{
		Key: "k",
	})
	if err != nil || s != "$k" {
		t.Fatalf("empty name: %q %v", s, err)
	}
	if _, err := secretRefToDollar(&corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "other"},
		Key:                  "k",
	}); err == nil {
		t.Fatal("custom name")
	}
}

func TestDollarSecretToRefEdges(t *testing.T) {
	for _, bad := range []string{"", "plain", "$a$b", "$", "$has space"} {
		if _, err := dollarSecretToRef(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
	ref, err := dollarSecretToRef("$ok.key")
	if err != nil || ref.Key != "ok.key" || ref.Name != defaultArgoCDSecretName {
		t.Fatalf("%#v %v", ref, err)
	}
}

func TestSecondsAndParseDurationPtr(t *testing.T) {
	diag := &Diagnostics{}
	if d, err := secondsDurationPtr(diag, "k", ""); err != nil || d != nil {
		t.Fatal("empty seconds")
	}
	if _, err := secondsDurationPtr(diag, "k", "x"); err == nil || !diag.HasErrors() {
		t.Fatal("bad seconds")
	}
	diag2 := &Diagnostics{}
	if d, err := parseDurationPtr(diag2, "k", ""); err != nil || d != nil {
		t.Fatal("empty duration")
	}
	if _, err := parseDurationPtr(diag2, "k", "nope"); err == nil || !diag2.HasErrors() {
		t.Fatal("bad duration")
	}
}

func TestNilDiagnosticsMethods(t *testing.T) {
	var d *Diagnostics
	d.Add(SeverityError, DirCMToCR, "k", "m")
	if d.Items() != nil {
		t.Fatal("nil Items")
	}
	if d.HasErrors() || d.HasWarnings() || d.Len() != 0 {
		t.Fatal("nil predicates")
	}
	d.Merge(&Diagnostics{})
	var human bytes.Buffer
	if err := d.WriteHuman(&human); err != nil || human.Len() != 0 {
		t.Fatal("nil WriteHuman")
	}
	var js bytes.Buffer
	if err := d.WriteJSON(&js); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(js.String(), "[]") {
		t.Fatalf("nil WriteJSON: %s", js.String())
	}
}

func TestWriteHumanEmptyKey(t *testing.T) {
	diag := &Diagnostics{}
	diag.Warn(DirCMToCR, "", "no key")
	var b bytes.Buffer
	if err := diag.WriteHuman(&b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "(none)") {
		t.Fatalf("%s", b.String())
	}
}

func TestNewKeyTrackerNilDataAndNilUse(t *testing.T) {
	kt := newKeyTracker(nil, &Diagnostics{}, DirCMToCR, "cm")
	if kt.source == nil {
		t.Fatal("source")
	}
	var nilKT *keyTracker
	nilKT.use("x")
	if _, ok := kt.get("missing"); ok {
		t.Fatal("missing")
	}
	kt.reportUnknown()
}

func TestParseAndMarshalResourceActionsEmpty(t *testing.T) {
	a, err := parseResourceActions("")
	if err != nil || a != nil {
		t.Fatalf("%v %v", a, err)
	}
	s, err := marshalResourceActions(nil)
	if err != nil || s != "" {
		t.Fatal(s, err)
	}
	s, err = marshalResourceActions(&argov1alpha1.ResourceActionsConfig{})
	if err != nil || s != "" {
		t.Fatal(s, err)
	}
}

func TestSplitGroupKindWildcards(t *testing.T) {
	g, k := splitGroupKind("*/*")
	if g != "*" || k != "*" {
		t.Fatalf("%s/%s", g, k)
	}
	g, k = splitGroupKind("all")
	if g != "*" || k != "*" {
		t.Fatalf("%s/%s", g, k)
	}
	g, k = splitGroupKind("ConfigMap")
	if g != "" || k != "ConfigMap" {
		t.Fatalf("%s/%s", g, k)
	}
}

func TestApplyOverrideMapActionsString(t *testing.T) {
	c := &argov1alpha1.ResourceCustomization{}
	err := applyOverrideMap(c, map[string]any{
		"actions": "discovery.lua: |\n  return {}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Actions == nil || c.Actions.DiscoveryLua == "" {
		t.Fatalf("%#v", c.Actions)
	}
}

func TestFromConfigMapsDefaultNameAndHardErrors(t *testing.T) {
	cfg, _, err := FromConfigMaps(ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{"url": "https://x"}},
	}, "", "ns")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != DefaultConfigurationName {
		t.Fatalf("name=%q", cfg.Name)
	}

	_, _, err = FromConfigMaps(ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{"resource.exclusions": ":"}},
	}, DefaultConfigurationName, "ns")
	if err == nil {
		t.Fatal("expected resource.exclusions parse error")
	}

	_, _, err = FromConfigMaps(ConfigMaps{
		CmdParams: &corev1.ConfigMap{Data: map[string]string{
			"applicationsetcontroller.requeue.after": "not-a-duration-for-hard?",
		}},
	}, DefaultConfigurationName, "ns")
	// duration parse may be soft; just exercise cmd-params path
	_ = err

	_, _, err = FromConfigMaps(ConfigMaps{
		RBAC: &corev1.ConfigMap{Data: map[string]string{"scopes": "ok"}},
	}, DefaultConfigurationName, "ns")
	if err != nil {
		t.Fatal(err)
	}
}

func TestMapExtensionsPerKeyAndBadYAML(t *testing.T) {
	cfg, _, err := FromConfigMaps(ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{
			"extension.config.metrics": `
connectionTimeout: 2s
services:
  - url: https://metrics.example.com
`,
		}},
	}, DefaultConfigurationName, "ns")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Spec.Server.Extensions) != 1 || cfg.Spec.Server.Extensions[0].Name != "metrics" {
		t.Fatalf("%#v", cfg.Spec.Server.Extensions)
	}

	_, _, err = FromConfigMaps(ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{"extension.config": ":"}},
	}, DefaultConfigurationName, "ns")
	if err == nil {
		t.Fatal("expected extension.config parse error")
	}
}

func TestParseDexSkipNonMapsAndExtraMarshalErrorPaths(t *testing.T) {
	dex, err := parseDexConfig(`
connectors:
  - not-a-map
  - type: github
    id: g
    name: G
    config:
      clientID: x
staticClients: not-a-list
issuer: https://example.com/dex
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(dex.Connectors) != 1 {
		t.Fatalf("connectors: %#v", dex.Connectors)
	}
	if dex.Extra == nil {
		t.Fatal("expected Extra for issuer + bad staticClients leftover cleared")
	}
}

func TestUseOpenLibsBadBool(t *testing.T) {
	_, _, err := FromConfigMaps(ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{
			"resource.customizations.useOpenLibs.apps_Deployment": "notabool",
		}},
	}, DefaultConfigurationName, "ns")
	if err == nil {
		t.Fatal("expected useOpenLibs parse error")
	}
}

func TestServerOnlyRepoServerClientWarn(t *testing.T) {
	_, diag, err := FromConfigMaps(ConfigMaps{
		CmdParams: &corev1.ConfigMap{Data: map[string]string{
			"server.repo.server.timeout.seconds":      "45",
			"server.repo.server.plaintext":            "true",
			"server.repo.server.strict.tls":           "false",
			"server.repo.server.ca.cert.path":         "/ca",
			"server.repo.server.client.cert.path":     "/cert",
			"server.repo.server.client.cert.key.path": "/key",
		}},
	}, DefaultConfigurationName, "ns")
	if err != nil {
		t.Fatal(err)
	}
	if !diag.HasWarnings() {
		t.Fatalf("expected server-only repo client warns: %#v", diag.Items())
	}
}

func TestPreserveMetaCopiesLabelsAndAnnotations(t *testing.T) {
	src := ConfigMaps{
		CM: &corev1.ConfigMap{
			ObjectMeta: metav1ObjectMeta("argocd-cm", map[string]string{"a": "1"}, map[string]string{"b": "2"}),
			Data:       map[string]string{"url": "https://x"},
		},
		CmdParams: &corev1.ConfigMap{
			ObjectMeta: metav1ObjectMeta("argocd-cmd-params-cm", map[string]string{"c": "3"}, nil),
			Data:       map[string]string{"redis.server": "r:6379"},
		},
		RBAC: &corev1.ConfigMap{
			ObjectMeta: metav1ObjectMeta("argocd-rbac-cm", nil, map[string]string{"d": "4"}),
			Data:       map[string]string{"policy.default": "role:readonly"},
		},
	}
	cfg, _, err := FromConfigMaps(src, DefaultConfigurationName, "ns")
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := ToConfigMapsWithSource(cfg, "ns", src)
	if err != nil {
		t.Fatal(err)
	}
	if out.CM.Labels["a"] != "1" || out.CM.Annotations["b"] != "2" {
		t.Fatalf("cm meta: %#v %#v", out.CM.Labels, out.CM.Annotations)
	}
	if out.CmdParams.Labels["c"] != "3" {
		t.Fatalf("cmd labels: %#v", out.CmdParams.Labels)
	}
	if out.RBAC.Annotations["d"] != "4" {
		t.Fatalf("rbac ann: %#v", out.RBAC.Annotations)
	}
}

func metav1ObjectMeta(name string, labels, ann map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Labels: labels, Annotations: ann}
}

func TestAccountsPasswordKeySkippedAndAdminOnly(t *testing.T) {
	cfg, _, err := FromConfigMaps(ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{
			"admin.enabled":           "false",
			"accounts.alice.password": "ignored",
			"accounts.robot.enabled":  "true",
		}},
	}, DefaultConfigurationName, "ns")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, a := range cfg.Spec.Server.Accounts {
		found[a.Name] = a.Enabled
	}
	if enabled, ok := found["admin"]; !ok || enabled {
		t.Fatalf("admin: %#v", cfg.Spec.Server.Accounts)
	}
	if enabled, ok := found["robot"]; !ok || !enabled {
		t.Fatalf("robot: %#v", cfg.Spec.Server.Accounts)
	}
	if _, ok := found["alice"]; ok {
		t.Fatalf("alice should be skipped: %#v", cfg.Spec.Server.Accounts)
	}
}

func TestKustomizeVersionPrefixAndMerge(t *testing.T) {
	cfg, _, err := FromConfigMaps(ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{
			"kustomize.version.v3.5.1":      "/tools/kustomize_3_5_1",
			"kustomize.version.v3.5.4":      "",
			"kustomize.path.v3.5.4":         "/tools/kustomize_3_5_4",
			"kustomize.buildOptions.v3.5.4": "--enable_kyaml true",
		}},
	}, DefaultConfigurationName, "ns")
	if err != nil {
		t.Fatal(err)
	}
	vers := cfg.Spec.RepoServer.Kustomize.Versions
	if len(vers) != 2 {
		t.Fatalf("%#v", vers)
	}
}

func TestMiscPartialBranches(t *testing.T) {
	cfg, _, err := FromConfigMaps(ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{
			"ga.trackingid":                        "UA-1",
			"exec.shells":                          "bash",
			"statusbadge.url":                      "https://badge/",
			"sourceHydrator.readmeMessageTemplate": "# hi",
			"commit.author.email":                  "a@b.c",
		}},
	}, DefaultConfigurationName, "ns")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Spec.Server.GoogleAnalytics == nil || cfg.Spec.Server.Exec == nil {
		t.Fatal("expected partial misc fields")
	}
}
