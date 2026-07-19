package mapping_test

import (
	"testing"

	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

func TestDurationValueEqual(t *testing.T) {
	d := mapping.DiffConfigMapDataNormalized(
		map[string]string{"timeout.reconciliation": "180s", "other": "x"},
		map[string]string{"timeout.reconciliation": "3m0s", "other": "x"},
	)
	if len(d.Changed) != 0 || len(d.Missing) != 0 || len(d.Extra) != 0 {
		t.Fatalf("expected no diffs, got %#v", d)
	}
}

func TestYAMLSemanticEqual(t *testing.T) {
	orig := "connectors:\n  - type: github\n    id: github\n"
	round := "connectors:\n- id: github\n  type: github\n"
	d := mapping.DiffConfigMapDataNormalized(
		map[string]string{"dex.config": orig},
		map[string]string{"dex.config": round},
	)
	if len(d.Changed) != 0 {
		t.Fatalf("expected YAML semantic equal, got changed=%v", d.Changed)
	}
}

func TestScopesYAMLListVsFlow(t *testing.T) {
	d := mapping.DiffConfigMapDataNormalized(
		map[string]string{"scopes": "[groups]"},
		map[string]string{"scopes": "- groups\n"},
	)
	if len(d.Changed) != 0 {
		t.Fatalf("expected scopes formats equal, got changed=%v", d.Changed)
	}
}

func TestRealDiffStillReported(t *testing.T) {
	d := mapping.DiffConfigMapDataNormalized(
		map[string]string{"url": "https://a.example.com"},
		map[string]string{"url": "https://b.example.com"},
	)
	if len(d.Changed) != 1 || d.Changed[0] != "url" {
		t.Fatalf("expected url changed, got %#v", d)
	}
}

func TestImpersonationAbsentEnforcedRequired(t *testing.T) {
	// Argo CD defaults enforced=true when the key is absent.
	d := mapping.DiffConfigMapDataNormalized(
		map[string]string{"application.sync.impersonation.enabled": "true"},
		map[string]string{
			"application.sync.impersonation.enabled":  "true",
			"application.sync.impersonation.enforced": "true",
		},
	)
	if len(d.Extra) != 0 || len(d.Changed) != 0 || len(d.Missing) != 0 {
		t.Fatalf("expected impersonation modes equal (required), got %#v", d)
	}
}

func TestMissingAndExtraKeys(t *testing.T) {
	d := mapping.DiffConfigMapDataNormalized(
		map[string]string{"a": "1", "b": "2"},
		map[string]string{"b": "2", "c": "3"},
	)
	if len(d.Missing) != 1 || d.Missing[0] != "a" {
		t.Fatalf("missing: %#v", d.Missing)
	}
	if len(d.Extra) != 1 || d.Extra[0] != "c" {
		t.Fatalf("extra: %#v", d.Extra)
	}
}

func TestImpersonationModeMismatchStillReported(t *testing.T) {
	d := mapping.DiffConfigMapDataNormalized(
		map[string]string{"application.sync.impersonation.enabled": "true"},
		map[string]string{
			"application.sync.impersonation.enabled":  "true",
			"application.sync.impersonation.enforced": "false",
		},
	)
	if len(d.Extra) != 1 || d.Extra[0] != "application.sync.impersonation.enforced" {
		t.Fatalf("expected enforced extra on required vs optional, got %#v", d)
	}
}
