package validate_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/pkg/validate"
)

var updateExample = flag.Bool("update-example", false, "regenerate EXAMPLE.yaml at the repo root")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func exampleYAMLPath(t *testing.T) string {
	t.Helper()
	root, err := moduleRoot()
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}
	return filepath.Join(root, "EXAMPLE.yaml")
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func TestGenerateExampleYAML(t *testing.T) {
	if !*updateExample {
		t.Skip("run with -update-example to regenerate EXAMPLE.yaml")
	}

	cfg := validate.FillExampleConfiguration()
	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal example: %v", err)
	}

	path := exampleYAMLPath(t)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Log("regenerated EXAMPLE.yaml; refresh round-trip goldens with: go test ./pkg/mapping -run 'TestCases/roundtrip/example-full' -update")
}

func TestExampleYAMLCompleteAndValid(t *testing.T) {
	path := exampleYAMLPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	cfg, err := decodeExampleStrict(data)
	if err != nil {
		t.Fatalf("strict decode EXAMPLE.yaml: %v", err)
	}

	unset := validate.FindUnsetFields(cfg)
	if len(unset) > 0 {
		var b strings.Builder
		for _, u := range unset {
			b.WriteString("\n  - ")
			b.WriteString(u.Path)
			b.WriteString(" (")
			b.WriteString(u.Kind)
			b.WriteString(")")
		}
		t.Fatalf("EXAMPLE.yaml has unset fields:%s", b.String())
	}

	diag := validate.Validate(cfg)
	if diag.HasErrors() {
		t.Fatalf("validate EXAMPLE.yaml: %v", diag)
	}
	if diag.HasWarnings() {
		t.Fatalf("unexpected validate warnings: %v", diag)
	}
}

func TestFillExampleConfigurationAllFieldsSet(t *testing.T) {
	cfg := validate.FillExampleConfiguration()
	unset := validate.FindUnsetFields(cfg)
	if len(unset) > 0 {
		t.Fatalf("FillExampleConfiguration left unset fields: %+v", unset)
	}
}

func TestFillExampleConfigurationValidates(t *testing.T) {
	cfg := validate.FillExampleConfiguration()
	diag := validate.Validate(cfg)
	if diag.HasErrors() {
		t.Fatalf("filled example failed validation: %v", diag)
	}
}

func decodeExampleStrict(yamlData []byte) (*argov1alpha1.ArgoCDConfiguration, error) {
	jsonData, err := yaml.YAMLToJSON(yamlData)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(jsonData))
	dec.DisallowUnknownFields()
	var cfg argov1alpha1.ArgoCDConfiguration
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
