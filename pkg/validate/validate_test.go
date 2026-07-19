package validate_test

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
	"github.com/crenshaw-dev/argocd-config/pkg/validate"
)

func TestValidateAccountCapabilitiesUnique(t *testing.T) {
	cfg := &argov1alpha1.ArgoCDConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: mapping.DefaultConfigurationName},
		Spec: argov1alpha1.ArgoCDConfigurationSpec{
			Server: &argov1alpha1.ServerConfig{
				Accounts: []argov1alpha1.AccountConfig{{
					Name:         "alice",
					Capabilities: []string{"login", "apiKey", "login"},
				}},
			},
		},
	}
	diag := validate.Validate(cfg)
	if !diag.HasErrors() {
		t.Fatal("expected duplicate capability error")
	}
	found := false
	for _, d := range diag.Items() {
		if strings.Contains(d.Key, "capabilities") && strings.Contains(strings.ToLower(d.Message), "duplicate") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected capabilities duplicate diagnostic, got %#v", diag.Items())
	}
}

func TestValidateName(t *testing.T) {
	cfg := &argov1alpha1.ArgoCDConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "wrong-name"},
	}
	diag := validate.Validate(cfg)
	if !diag.HasErrors() {
		t.Fatal("expected name error")
	}
}

func TestValidateURL(t *testing.T) {
	cfg := &argov1alpha1.ArgoCDConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: mapping.DefaultConfigurationName},
		Spec: argov1alpha1.ArgoCDConfigurationSpec{
			Server: &argov1alpha1.ServerConfig{URLs: []string{"ftp://bad.example.com"}},
		},
	}
	diag := validate.Validate(cfg)
	if !diag.HasErrors() {
		t.Fatal("expected URL scheme error")
	}
}

func TestValidateGoodURL(t *testing.T) {
	cfg := &argov1alpha1.ArgoCDConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: mapping.DefaultConfigurationName},
		Spec: argov1alpha1.ArgoCDConfigurationSpec{
			Server: &argov1alpha1.ServerConfig{URLs: []string{"https://argocd.example.com"}},
		},
	}
	diag := validate.Validate(cfg)
	if diag.HasErrors() {
		t.Fatalf("unexpected errors: %v", diag)
	}
}

func TestValidateNilConfig(t *testing.T) {
	diag := validate.Validate(nil)
	if !diag.HasErrors() {
		t.Fatal("expected error for nil configuration")
	}
}

func TestValidateWrongKind(t *testing.T) {
	cfg := &argov1alpha1.ArgoCDConfiguration{
		TypeMeta:   metav1.TypeMeta{Kind: "NotArgoCDConfiguration"},
		ObjectMeta: metav1.ObjectMeta{Name: mapping.DefaultConfigurationName},
	}
	diag := validate.Validate(cfg)
	if !diag.HasErrors() {
		t.Fatal("expected kind error")
	}
}

func TestValidateUnexpectedAPIVersionWarns(t *testing.T) {
	cfg := &argov1alpha1.ArgoCDConfiguration{
		TypeMeta:   metav1.TypeMeta{APIVersion: "argo.crenshaw.dev/v99"},
		ObjectMeta: metav1.ObjectMeta{Name: mapping.DefaultConfigurationName},
	}
	diag := validate.Validate(cfg)
	if !diag.HasWarnings() {
		t.Fatal("expected apiVersion warning")
	}
	if diag.HasErrors() {
		t.Fatalf("unexpected errors: %v", diag)
	}
}

func TestValidateURLs(t *testing.T) {
	cfg := &argov1alpha1.ArgoCDConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: mapping.DefaultConfigurationName},
		Spec: argov1alpha1.ArgoCDConfigurationSpec{
			Server: &argov1alpha1.ServerConfig{
				URLs: []string{"https://argocd.example.com", "ftp://bad.example.com", "https://extra.example.com"},
			},
		},
	}
	diag := validate.Validate(cfg)
	if !diag.HasErrors() {
		t.Fatal("expected urls scheme error")
	}
}

func TestValidateAgainstCRD(t *testing.T) {
	diag := validate.ValidateAgainstCRD()
	if diag.HasErrors() || diag.HasWarnings() {
		t.Fatalf("unexpected CRD diagnostics: %v", diag)
	}
}
