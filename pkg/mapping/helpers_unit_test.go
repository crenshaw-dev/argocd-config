package mapping_test

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

func TestParseKVMapAndJoin(t *testing.T) {
	cms := mapping.ConfigMaps{
		CmdParams: &corev1.ConfigMap{Data: map[string]string{
			"otlp.headers": "b=2,a=1",
			"otlp.attrs":   "z:9,m:3",
		}},
	}
	cfg, _, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Spec.OTLP == nil || cfg.Spec.OTLP.Headers["a"] != "1" || cfg.Spec.OTLP.Headers["b"] != "2" {
		t.Fatalf("headers: %#v", cfg.Spec.OTLP)
	}
	if cfg.Spec.OTLP.Attrs["m"] != "3" || cfg.Spec.OTLP.Attrs["z"] != "9" {
		t.Fatalf("attrs: %#v", cfg.Spec.OTLP.Attrs)
	}
	out, _, err := mapping.ToConfigMaps(cfg, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	if out.CmdParams.Data["otlp.headers"] != "a=1,b=2" {
		t.Fatalf("headers join: %q", out.CmdParams.Data["otlp.headers"])
	}
	if out.CmdParams.Data["otlp.attrs"] != "m:3,z:9" {
		t.Fatalf("attrs join: %q", out.CmdParams.Data["otlp.attrs"])
	}
}

func TestDiagnosticsMerge(t *testing.T) {
	a := &mapping.Diagnostics{}
	b := &mapping.Diagnostics{}
	a.Warn(mapping.DirCMToCR, "k1", "m1")
	b.Error(mapping.DirCRToCM, "k2", "m2")
	a.Merge(b)
	if a.Len() != 2 {
		t.Fatalf("len=%d", a.Len())
	}
	if !a.HasErrors() || !a.HasWarnings() {
		t.Fatal("expected both error and warn")
	}
	items := a.Items()
	if items[0].Severity != mapping.SeverityError {
		t.Fatalf("sorted severity: %#v", items)
	}
}

func TestDollarSecretToRefViaOIDC(t *testing.T) {
	cms := mapping.ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{
			"oidc.config": "clientSecret: $my.key\nclientID: x\nissuer: https://i.example.com\n",
		}},
	}
	cfg, _, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	ref := cfg.Spec.Server.OIDC.ClientSecretRef
	if ref == nil || ref.Key != "my.key" || ref.Name != "argocd-secret" {
		t.Fatalf("ref: %#v", ref)
	}
	out, _, err := mapping.ToConfigMaps(cfg, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.CM.Data["oidc.config"], "$my.key") {
		t.Fatalf("oidc.config: %s", out.CM.Data["oidc.config"])
	}
}

func TestQuantityParseError(t *testing.T) {
	cms := mapping.ConfigMaps{
		CmdParams: &corev1.ConfigMap{Data: map[string]string{
			"reposerver.max.combined.directory.manifests.size": "not-a-quantity",
		}},
	}
	_, diag, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	if !diag.HasErrors() {
		t.Fatalf("expected quantity parse error, got %#v", diag.Items())
	}
}

func TestDollarSecretToRefRejectsEmbeddedDollarAndSpaces(t *testing.T) {
	for _, secret := range []string{"$foo$bar", "$has space", "nosigils", "$"} {
		cms := mapping.ConfigMaps{
			CM: &corev1.ConfigMap{Data: map[string]string{
				"oidc.config": "clientSecret: " + secret + "\nclientID: x\nissuer: https://i.example.com\n",
			}},
		}
		_, _, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
		if err == nil {
			t.Fatalf("expected error for clientSecret %q", secret)
		}
	}
}

func TestSecretRefToDollarNonArgoCDSecretDiagnostic(t *testing.T) {
	cfg, _, err := mapping.FromConfigMaps(mapping.ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{
			"url": "https://argocd.example.com",
		}},
	}, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Spec.Server.OIDC = &argov1alpha1.OIDCConfig{
		ClientID:  "cid",
		IssuerURL: "https://issuer.example.com",
		ClientSecretRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "other-secret"},
			Key:                  "clientSecret",
		},
	}
	out, diag, err := mapping.ToConfigMaps(cfg, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	if !diag.HasErrors() {
		t.Fatalf("expected error diagnostic, got %#v", diag.Items())
	}
	if _, ok := out.CM.Data["oidc.config"]; ok {
		t.Fatalf("oidc.config should be omitted on secret name error: %v", out.CM.Data["oidc.config"])
	}
	if out.CM.Data["url"] != "https://argocd.example.com" {
		t.Fatalf("other keys should still unmap: %#v", out.CM.Data)
	}
}

func TestParseKVMapMalformedAndEmpty(t *testing.T) {
	cms := mapping.ConfigMaps{
		CmdParams: &corev1.ConfigMap{Data: map[string]string{
			"otlp.headers": "good=1,badentry,=noval,empty=",
			"otlp.attrs":   "",
			"otlp.address": "collector:4317",
		}},
	}
	cfg, _, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Spec.OTLP.Headers["good"] != "1" {
		t.Fatalf("headers: %#v", cfg.Spec.OTLP.Headers)
	}
	if _, ok := cfg.Spec.OTLP.Headers["badentry"]; ok {
		t.Fatalf("malformed entry should be skipped: %#v", cfg.Spec.OTLP.Headers)
	}
	if len(cfg.Spec.OTLP.Attrs) != 0 {
		t.Fatalf("empty attrs should yield empty map: %#v", cfg.Spec.OTLP.Attrs)
	}
	out, _, err := mapping.ToConfigMaps(cfg, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	if out.CmdParams.Data["otlp.headers"] != "empty=,good=1" && out.CmdParams.Data["otlp.headers"] != "good=1,empty=" {
		// empty= is kept (k nonempty); order is sorted
		if out.CmdParams.Data["otlp.headers"] != "empty=,good=1" {
			t.Fatalf("headers join: %q", out.CmdParams.Data["otlp.headers"])
		}
	}
}

func TestAsIntViaExtensionMaxIdleConnections(t *testing.T) {
	// YAML unmarshals numbers as int/int64/float64 depending on decoder;
	// string forms exercise the string branch of asInt.
	cms := mapping.ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{
			"extension.config": `
extensions:
  - name: n
    backend:
      maxIdleConnections: "42"
      services:
        - url: http://example.com
`,
		}},
	}
	cfg, _, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Spec.Server.Extensions[0].Backend.Transport == nil || cfg.Spec.Server.Extensions[0].Backend.Transport.MaxIdleConnections != 42 {
		t.Fatalf("maxIdleConnections: %#v", cfg.Spec.Server.Extensions[0].Backend)
	}
}

func TestAsStringViaDexNumericFields(t *testing.T) {
	cms := mapping.ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{
			"dex.config": `
connectors:
  - type: 123
    id: 456
    name: Named
    config:
      n: 1
`,
		}},
	}
	cfg, _, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	c := cfg.Spec.Server.Dex.Connectors[0]
	if c.Type != "123" || c.ID != "456" {
		t.Fatalf("asString numeric: %#v", c)
	}
}

func TestParseDurationPtrErrorViaOIDC(t *testing.T) {
	cms := mapping.ConfigMaps{
		CM: &corev1.ConfigMap{Data: map[string]string{
			"oidc.config": `
name: X
issuer: https://issuer.example.com
clientID: cid
clientSecret: $oidc.secret
userInfoCacheExpiration: not-a-duration
`,
		}},
	}
	_, diag, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatal(err)
	}
	if !diag.HasErrors() {
		t.Fatalf("expected duration parse error, got %#v", diag.Items())
	}
}
