package mapping_test

import (
	"testing"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

// TestRoundTripSemanticEquality is the sole property-style invariant:
// CM -> CR -> CM -> CR must yield a semantically equal configuration.
// Value-level ConfigMap golden expectations live under testdata/cases/.
func TestRoundTripSemanticEquality(t *testing.T) {
	cms := loadSampleCMS(t)
	cfg1, diag1, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("FromConfigMaps: %v", err)
	}
	if diag1.HasErrors() {
		t.Fatalf("unexpected errors: %+v", diag1.Items())
	}
	out, _, err := mapping.ToConfigMapsWithSource(cfg1, "argocd", cms)
	if err != nil {
		t.Fatalf("ToConfigMaps: %v", err)
	}
	cfg2, diag2, err := mapping.FromConfigMaps(out, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("second FromConfigMaps: %v", err)
	}
	if diag2.HasErrors() {
		t.Fatalf("unexpected errors on second pass: %+v", diag2.Items())
	}
	assertSemanticEqual(t, cfg1, cfg2)
}

func assertSemanticEqual(t *testing.T, a, b *argov1alpha1.ArgoCDConfiguration) {
	t.Helper()
	if a.Spec.Server == nil || b.Spec.Server == nil {
		t.Fatal("server missing")
	}
	if len(a.Spec.Server.URLs) != len(b.Spec.Server.URLs) {
		t.Fatalf("urls len: %d vs %d", len(a.Spec.Server.URLs), len(b.Spec.Server.URLs))
	}
	for i := range a.Spec.Server.URLs {
		if a.Spec.Server.URLs[i] != b.Spec.Server.URLs[i] {
			t.Fatalf("urls[%d]: %q vs %q", i, a.Spec.Server.URLs[i], b.Spec.Server.URLs[i])
		}
	}
	if a.Spec.Server.OIDC == nil || b.Spec.Server.OIDC == nil ||
		a.Spec.Server.OIDC.ClientSecretRef == nil || b.Spec.Server.OIDC.ClientSecretRef == nil ||
		a.Spec.Server.OIDC.ClientSecretRef.Key != b.Spec.Server.OIDC.ClientSecretRef.Key {
		t.Fatalf("oidc secret ref drift: %#v vs %#v", a.Spec.Server.OIDC, b.Spec.Server.OIDC)
	}
	if a.Spec.Server.RBAC == nil || b.Spec.Server.RBAC == nil ||
		a.Spec.Server.RBAC.PolicyCSV != b.Spec.Server.RBAC.PolicyCSV {
		t.Fatalf("rbac policy drift")
	}
	if a.Spec.Server.Dex == nil || b.Spec.Server.Dex == nil ||
		len(a.Spec.Server.Dex.Connectors) == 0 || len(b.Spec.Server.Dex.Connectors) == 0 ||
		a.Spec.Server.Dex.Connectors[0].Type != b.Spec.Server.Dex.Connectors[0].Type {
		t.Fatalf("dex connector drift")
	}
	if a.Spec.InstallationID != b.Spec.InstallationID {
		t.Fatalf("installationID: %q vs %q", a.Spec.InstallationID, b.Spec.InstallationID)
	}
}
