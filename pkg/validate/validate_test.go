package validate_test

import (
	"os"
	"path/filepath"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
	"github.com/crenshaw-dev/argocd-config/pkg/validate"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
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
			Server: &argov1alpha1.ServerConfig{URL: "ftp://bad.example.com"},
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
			Server: &argov1alpha1.ServerConfig{URL: "https://argocd.example.com"},
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

func TestValidateAdditionalURLs(t *testing.T) {
	cfg := &argov1alpha1.ArgoCDConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: mapping.DefaultConfigurationName},
		Spec: argov1alpha1.ArgoCDConfigurationSpec{
			Server: &argov1alpha1.ServerConfig{
				URL:            "https://argocd.example.com",
				AdditionalURLs: []string{"ftp://bad.example.com", "https://extra.example.com"},
			},
		},
	}
	diag := validate.Validate(cfg)
	if !diag.HasErrors() {
		t.Fatal("expected additionalURLs scheme error")
	}
}

func TestValidateURLMissingHost(t *testing.T) {
	cfg := &argov1alpha1.ArgoCDConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: mapping.DefaultConfigurationName},
		Spec: argov1alpha1.ArgoCDConfigurationSpec{
			Server: &argov1alpha1.ServerConfig{URL: "https://"},
		},
	}
	diag := validate.Validate(cfg)
	if !diag.HasErrors() {
		t.Fatal("expected missing host error")
	}
}

func TestValidateAgainstCRDFromModuleRoot(t *testing.T) {
	root := moduleRoot(t)
	t.Chdir(root)

	diag := validate.ValidateAgainstCRD()
	if diag.HasWarnings() {
		t.Fatalf("unexpected CRD warnings from module root: %v", diag)
	}
}

func TestValidateAgainstCRDFromTempDirWarns(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	diag := validate.ValidateAgainstCRD()
	if !diag.HasWarnings() {
		t.Fatal("expected CRD warning when run outside module root")
	}
}
