package mapping_test

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

func TestMappingTable(t *testing.T) {
	tests := []struct {
		name    string
		cms     mapping.ConfigMaps
		wantErr bool
		check   func(t *testing.T, cfg *argov1alpha1.ArgoCDConfiguration, diag *mapping.Diagnostics)
	}{
		{
			name: "OIDC dollar string clientSecret to SecretKeySelector",
			cms: mapping.ConfigMaps{
				CM: cmWithData(&corev1.ConfigMap{}, map[string]string{
					"oidc.config": "name: Test\nissuer: https://issuer.example.com\nclientID: app\nclientSecret: $oidc.clientSecret\n",
				}),
			},
			check: func(t *testing.T, cfg *argov1alpha1.ArgoCDConfiguration, diag *mapping.Diagnostics) {
				t.Helper()
				ref := cfg.Spec.Server.OIDC.ClientSecretRef
				if ref == nil || ref.Name != "argocd-secret" || ref.Key != "oidc.clientSecret" {
					t.Fatalf("ClientSecretRef = %#v", ref)
				}
			},
		},
		{
			name: "Dex connector envelope",
			cms: mapping.ConfigMaps{
				CM: cmWithData(&corev1.ConfigMap{}, map[string]string{
					"dex.config": "connectors:\n  - type: github\n    id: github\n    name: GitHub\n    config:\n      clientID: $dex.github.clientId\n",
				}),
			},
			check: func(t *testing.T, cfg *argov1alpha1.ArgoCDConfiguration, diag *mapping.Diagnostics) {
				t.Helper()
				dex := cfg.Spec.Server.Dex
				if dex == nil || len(dex.Connectors) != 1 {
					t.Fatalf("Dex = %#v", dex)
				}
				c := dex.Connectors[0]
				if c.Type != "github" || c.ID != "github" || c.Name != "GitHub" {
					t.Fatalf("connector = %#v", c)
				}
				if len(c.Config.Raw) == 0 || !strings.Contains(string(c.Config.Raw), "clientID") {
					t.Fatalf("connector config not preserved: %s", c.Config.Raw)
				}
			},
		},
		{
			name: "RBAC policy overlays",
			cms: mapping.ConfigMaps{
				RBAC: cmWithData(&corev1.ConfigMap{}, map[string]string{
					"policy.default":   "role:readonly",
					"policy.csv":       "p, role:admin, applications, *, */*, allow\n",
					"policy.extra.csv": "p, role:extra, applications, get, */*, allow\n",
				}),
			},
			check: func(t *testing.T, cfg *argov1alpha1.ArgoCDConfiguration, diag *mapping.Diagnostics) {
				t.Helper()
				rbac := cfg.Spec.Server.RBAC
				if rbac == nil || rbac.Default != "role:readonly" {
					t.Fatalf("RBAC default = %#v", rbac)
				}
				if len(rbac.PolicyOverlays) != 1 || rbac.PolicyOverlays[0].Name != "extra" {
					t.Fatalf("PolicyOverlays = %#v", rbac.PolicyOverlays)
				}
			},
		},
		{
			name: "plugin tar exclusions semicolon separator",
			cms: mapping.ConfigMaps{
				CmdParams: cmWithData(&corev1.ConfigMap{}, map[string]string{
					"reposerver.plugin.tar.exclusions": "*.git;*.tmp;node_modules",
				}),
			},
			check: func(t *testing.T, cfg *argov1alpha1.ArgoCDConfiguration, diag *mapping.Diagnostics) {
				t.Helper()
				globs := cfg.Spec.RepoServer.Plugin.TarExclusionGlobs
				if len(globs) != 3 || globs[0] != "*.git" || globs[1] != "*.tmp" || globs[2] != "node_modules" {
					t.Fatalf("PluginTarExclusionGlobs = %#v", globs)
				}
			},
		},
		{
			name: "invalid duration parse error",
			cms: mapping.ConfigMaps{
				CM: cmWithData(&corev1.ConfigMap{}, map[string]string{
					"users.session.duration": "not-a-duration",
				}),
			},
			check: func(t *testing.T, cfg *argov1alpha1.ArgoCDConfiguration, diag *mapping.Diagnostics) {
				t.Helper()
				if !diag.HasErrors() {
					t.Fatalf("expected duration error diagnostic, got %#v", diag.Items())
				}
				if !hasDiagnostic(diag.Items(), mapping.SeverityError, "users.session.duration", "invalid duration") {
					t.Fatalf("missing duration error, got %#v", diag.Items())
				}
			},
		},
		{
			name: "invalid quantity parse error",
			cms: mapping.ConfigMaps{
				CM: cmWithData(&corev1.ConfigMap{}, map[string]string{
					"webhook.maxPayloadSizeMB": "bad-quantity",
				}),
			},
			check: func(t *testing.T, cfg *argov1alpha1.ArgoCDConfiguration, diag *mapping.Diagnostics) {
				t.Helper()
				if !diag.HasErrors() {
					t.Fatalf("expected quantity error diagnostic, got %#v", diag.Items())
				}
				if !hasDiagnostic(diag.Items(), mapping.SeverityError, "webhook.maxPayloadSizeMB", "invalid quantity") {
					t.Fatalf("missing quantity error, got %#v", diag.Items())
				}
			},
		},
		{
			name: "empty ConfigMaps",
			cms:  mapping.ConfigMaps{},
			check: func(t *testing.T, cfg *argov1alpha1.ArgoCDConfiguration, diag *mapping.Diagnostics) {
				t.Helper()
				if cfg.Name != mapping.DefaultConfigurationName {
					t.Fatalf("name = %q", cfg.Name)
				}
				if cfg.Spec.Server != nil || cfg.Spec.Controller != nil || cfg.Spec.RepoServer != nil {
					t.Fatalf("expected empty spec, got %#v", cfg.Spec)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, diag, err := mapping.FromConfigMaps(tt.cms, mapping.DefaultConfigurationName, "argocd")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("FromConfigMaps: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg, diag)
			}
		})
	}
}
