package validate

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

const crdRelativePath = "config/crd/bases/argo.crenshaw.dev_argocdconfigurations.yaml"

// Validate performs offline structural checks on an ArgoCDConfiguration.
//
// This mirrors a subset of CRD OpenAPI/CEL rules (singleton name, http(s) URLs).
// Full CEL evaluation against the CRD schema requires a Kubernetes apiserver or
// envtest; ValidateAgainstCRD loads the embedded CRD manifest for sanity checks only.
func Validate(cfg *argov1alpha1.ArgoCDConfiguration) *mapping.Diagnostics {
	diag := &mapping.Diagnostics{}
	if cfg == nil {
		diag.Error("", "metadata", "configuration is nil")
		return diag
	}

	if cfg.Name != mapping.DefaultConfigurationName {
		diag.Error("", "metadata.name",
			fmt.Sprintf("name must be %q, got %q", mapping.DefaultConfigurationName, cfg.Name))
	}

	if cfg.Kind != "" && cfg.Kind != "ArgoCDConfiguration" {
		diag.Error("", "kind",
			fmt.Sprintf("kind must be %q, got %q", "ArgoCDConfiguration", cfg.Kind))
	}

	if cfg.APIVersion != "" &&
		cfg.APIVersion != argov1alpha1.GroupVersion.String() &&
		cfg.APIVersion != "argo.crenshaw.dev/v1beta1" {
		diag.Warn("", "apiVersion",
			fmt.Sprintf("expected apiVersion %q or v1beta1 spoke, got %q", argov1alpha1.GroupVersion.String(), cfg.APIVersion))
	}

	if s := cfg.Spec.Server; s != nil {
		validateHTTPURL(diag, "spec.server.url", s.URL)
		for i, u := range s.AdditionalURLs {
			validateHTTPURL(diag, fmt.Sprintf("spec.server.additionalURLs[%d]", i), string(u))
		}
	}

	return diag
}

// ValidateAgainstCRD verifies the CRD manifest is present and readable.
// Offline validation does not evaluate CEL rules from the CRD; use a cluster for that.
func ValidateAgainstCRD() *mapping.Diagnostics {
	diag := &mapping.Diagnostics{}
	path, err := locateCRD()
	if err != nil {
		diag.Warn("", "crd", fmt.Sprintf("CRD manifest not found: %v", err))
		return diag
	}
	info, err := os.Stat(path)
	if err != nil {
		diag.Warn("", "crd", fmt.Sprintf("cannot stat CRD at %q: %v", path, err))
		return diag
	}
	if info.Size() == 0 {
		diag.Warn("", "crd", fmt.Sprintf("CRD at %q is empty", path))
	}
	return diag
}

func validateHTTPURL(diag *mapping.Diagnostics, field, raw string) {
	if raw == "" {
		return
	}
	u, err := url.Parse(raw)
	if err != nil {
		diag.Error("", field, fmt.Sprintf("invalid URL %q: %v", raw, err))
		return
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		diag.Error("", field, fmt.Sprintf("URL must start with http:// or https://, got %q", raw))
	}
	if u.Host == "" {
		diag.Error("", field, fmt.Sprintf("URL must include a host, got %q", raw))
	}
}

func locateCRD() (string, error) {
	candidates := []string{
		crdRelativePath,
		filepath.Join("..", crdRelativePath),
		filepath.Join("..", "..", crdRelativePath),
	}
	if modRoot := moduleRoot(); modRoot != "" {
		candidates = append([]string{filepath.Join(modRoot, crdRelativePath)}, candidates...)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("could not find %s from working directory", crdRelativePath)
}

func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
