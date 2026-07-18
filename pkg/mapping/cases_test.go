package mapping_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

// updateGoldens regenerates expected/ files under testdata/cases.
// Use only for regression guards (behavior already correct), never for bug repros.
var updateGoldens = flag.Bool("update", false, "update golden expected/ files under testdata/cases")

// caseManifest is the per-case metadata file (case.yaml).
type caseManifest struct {
	Description string `json:"description" yaml:"description"`
	Issue       string `json:"issue,omitempty" yaml:"issue,omitempty"`
	Direction   string `json:"direction" yaml:"direction"` // from | to | roundtrip
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace   string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Strict      bool   `json:"strict,omitempty" yaml:"strict,omitempty"`
}

func TestCases(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "cases")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("testdata/cases not present yet")
	}

	var cases []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "case.yaml" {
			return nil
		}
		cases = append(cases, filepath.Dir(path))
		return nil
	})
	if err != nil {
		t.Fatalf("walk cases: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases found under testdata/cases (expected at least one case.yaml)")
	}

	for _, dir := range cases {
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(rel, func(t *testing.T) {
			runCase(t, dir, rel)
		})
	}
}

func runCase(t *testing.T, dir, rel string) {
	t.Helper()
	manifest, err := loadManifest(filepath.Join(dir, "case.yaml"))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	if manifest.Description == "" {
		t.Fatalf("%s: case.yaml missing required field 'description'", rel)
	}
	switch manifest.Direction {
	case "from", "to", "roundtrip":
	default:
		t.Fatalf("%s: case.yaml direction must be from|to|roundtrip, got %q", rel, manifest.Direction)
	}
	name := manifest.Name
	if name == "" {
		name = mapping.DefaultConfigurationName
	}
	ns := manifest.Namespace
	if ns == "" {
		ns = "argocd"
	}

	expectedDir := filepath.Join(dir, "expected")
	if !*updateGoldens {
		if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
			t.Fatalf("%s: missing expected/ directory (add goldens or run with -update)", rel)
		}
	}

	switch manifest.Direction {
	case "from":
		runFromCase(t, dir, rel, name, ns, manifest)
	case "to":
		runToCase(t, dir, rel, ns, manifest)
	case "roundtrip":
		runRoundtripCase(t, dir, rel, name, ns, manifest)
	}
}

func runFromCase(t *testing.T, dir, rel, name, ns string, m caseManifest) {
	t.Helper()
	cms, err := loadCaseConfigMaps(filepath.Join(dir, "input"))
	if err != nil {
		t.Fatalf("%s: load input: %v\n%s", rel, err, m.Description)
	}
	cfg, diag, err := mapping.FromConfigMaps(cms, name, ns)
	if err != nil {
		t.Fatalf("%s: FromConfigMaps: %v\n%s", rel, err, m.Description)
	}
	if m.Strict && diag.HasWarnings() {
		t.Fatalf("%s: strict=true but got warnings:\n%s\n%s", rel, formatDiag(diag), m.Description)
	}

	wantCfgPath := filepath.Join(dir, "expected", "configuration.yaml")
	wantDiagPath := filepath.Join(dir, "expected", "diagnostics.yaml")
	if *updateGoldens {
		if err := writeYAMLFile(wantCfgPath, cfg); err != nil {
			t.Fatal(err)
		}
		if err := writeYAMLFile(wantDiagPath, diag.Items()); err != nil {
			t.Fatal(err)
		}
		return
	}
	assertYAMLEqual(t, rel, m.Description, wantCfgPath, cfg)
	assertYAMLEqual(t, rel, m.Description, wantDiagPath, diag.Items())
}

func runToCase(t *testing.T, dir, rel, ns string, m caseManifest) {
	t.Helper()
	cfg, err := loadCaseConfiguration(filepath.Join(dir, "input", "configuration.yaml"))
	if err != nil {
		t.Fatalf("%s: load input configuration: %v\n%s", rel, err, m.Description)
	}
	out, diag, err := mapping.ToConfigMaps(cfg, ns)
	if err != nil {
		t.Fatalf("%s: ToConfigMaps: %v\n%s", rel, err, m.Description)
	}

	wantCMDir := filepath.Join(dir, "expected", "configmaps")
	wantDiagPath := filepath.Join(dir, "expected", "diagnostics.yaml")
	if *updateGoldens {
		if err := writeConfigMapsDir(wantCMDir, out); err != nil {
			t.Fatal(err)
		}
		if err := writeYAMLFile(wantDiagPath, diag.Items()); err != nil {
			t.Fatal(err)
		}
		return
	}
	assertConfigMapsEqual(t, rel, m.Description, wantCMDir, out)
	assertYAMLEqual(t, rel, m.Description, wantDiagPath, diag.Items())
}

func runRoundtripCase(t *testing.T, dir, rel, name, ns string, m caseManifest) {
	t.Helper()
	cms, err := loadCaseConfigMaps(filepath.Join(dir, "input"))
	if err != nil {
		t.Fatalf("%s: load input: %v\n%s", rel, err, m.Description)
	}
	cfg, diag1, err := mapping.FromConfigMaps(cms, name, ns)
	if err != nil {
		t.Fatalf("%s: FromConfigMaps: %v\n%s", rel, err, m.Description)
	}
	out, diag2, err := mapping.ToConfigMapsWithSource(cfg, ns, cms)
	if err != nil {
		t.Fatalf("%s: ToConfigMaps: %v\n%s", rel, err, m.Description)
	}
	diag := &mapping.Diagnostics{}
	diag.Merge(diag1)
	diag.Merge(diag2)

	wantCfgPath := filepath.Join(dir, "expected", "configuration.yaml")
	wantCMDir := filepath.Join(dir, "expected", "configmaps")
	wantDiagPath := filepath.Join(dir, "expected", "diagnostics.yaml")
	if *updateGoldens {
		if err := writeYAMLFile(wantCfgPath, cfg); err != nil {
			t.Fatal(err)
		}
		if err := writeConfigMapsDir(wantCMDir, out); err != nil {
			t.Fatal(err)
		}
		if err := writeYAMLFile(wantDiagPath, diag.Items()); err != nil {
			t.Fatal(err)
		}
		return
	}
	assertYAMLEqual(t, rel, m.Description, wantCfgPath, cfg)
	assertConfigMapsEqual(t, rel, m.Description, wantCMDir, out)
	assertYAMLEqual(t, rel, m.Description, wantDiagPath, diag.Items())
}

func loadManifest(path string) (caseManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return caseManifest{}, err
	}
	var m caseManifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return caseManifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

func loadCaseConfigMaps(dir string) (mapping.ConfigMaps, error) {
	cms := mapping.ConfigMaps{}
	var err error
	cms.CM, err = readOptionalCM(filepath.Join(dir, "argocd-cm.yaml"))
	if err != nil {
		return cms, err
	}
	cms.CmdParams, err = readOptionalCM(filepath.Join(dir, "argocd-cmd-params-cm.yaml"))
	if err != nil {
		return cms, err
	}
	cms.RBAC, err = readOptionalCM(filepath.Join(dir, "argocd-rbac-cm.yaml"))
	if err != nil {
		return cms, err
	}
	if cms.CM == nil && cms.CmdParams == nil && cms.RBAC == nil {
		return cms, fmt.Errorf("no ConfigMaps in %s", dir)
	}
	return cms, nil
}

func readOptionalCM(path string) (*corev1.ConfigMap, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cm := &corev1.ConfigMap{}
	if err := yaml.Unmarshal(b, cm); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	return cm, nil
}

func loadCaseConfiguration(path string) (*argov1alpha1.ArgoCDConfiguration, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &argov1alpha1.ArgoCDConfiguration{}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func writeYAMLFile(path string, obj any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func writeConfigMapsDir(dir string, cms mapping.ConfigMaps) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if cms.CM != nil {
		if err := writeYAMLFile(filepath.Join(dir, "argocd-cm.yaml"), cms.CM); err != nil {
			return err
		}
	}
	if cms.CmdParams != nil {
		if err := writeYAMLFile(filepath.Join(dir, "argocd-cmd-params-cm.yaml"), cms.CmdParams); err != nil {
			return err
		}
	}
	if cms.RBAC != nil {
		if err := writeYAMLFile(filepath.Join(dir, "argocd-rbac-cm.yaml"), cms.RBAC); err != nil {
			return err
		}
	}
	return nil
}

func assertYAMLEqual(t *testing.T, rel, desc, wantPath string, got any) {
	t.Helper()
	wantBytes, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("%s: read %s: %v\n%s", rel, wantPath, err, desc)
	}
	gotBytes, err := yaml.Marshal(got)
	if err != nil {
		t.Fatalf("%s: marshal got: %v\n%s", rel, err, desc)
	}
	// Normalize via JSON round-trip for stable comparison of maps/lists.
	wantNorm, err := normalizeYAML(wantBytes)
	if err != nil {
		t.Fatalf("%s: normalize want: %v\n%s", rel, err, desc)
	}
	gotNorm, err := normalizeYAML(gotBytes)
	if err != nil {
		t.Fatalf("%s: normalize got: %v\n%s", rel, err, desc)
	}
	if wantNorm == gotNorm {
		return
	}
	t.Fatalf("%s: mismatch for %s\n%s\n%s", rel, filepath.Base(wantPath), desc, lineDiff(wantNorm, gotNorm))
}

func assertConfigMapsEqual(t *testing.T, rel, desc, wantDir string, got mapping.ConfigMaps) {
	t.Helper()
	check := func(name string, cm *corev1.ConfigMap) {
		path := filepath.Join(wantDir, name)
		if cm == nil {
			if _, err := os.Stat(path); err == nil {
				t.Fatalf("%s: expected %s but got nil\n%s", rel, name, desc)
			}
			return
		}
		assertYAMLEqual(t, rel, desc, path, cm)
	}
	check("argocd-cm.yaml", got.CM)
	check("argocd-cmd-params-cm.yaml", got.CmdParams)
	check("argocd-rbac-cm.yaml", got.RBAC)
}

func normalizeYAML(b []byte) (string, error) {
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return "", err
	}
	jb, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jb) + "\n", nil
}

func lineDiff(want, got string) string {
	wLines := strings.Split(want, "\n")
	gLines := strings.Split(got, "\n")
	var buf bytes.Buffer
	buf.WriteString("--- want\n+++ got\n")
	max := len(wLines)
	if len(gLines) > max {
		max = len(gLines)
	}
	shown := 0
	const limit = 40
	for i := 0; i < max && shown < limit; i++ {
		var w, g string
		if i < len(wLines) {
			w = wLines[i]
		}
		if i < len(gLines) {
			g = gLines[i]
		}
		if w == g {
			continue
		}
		if w != "" {
			fmt.Fprintf(&buf, "-%s\n", w)
			shown++
		}
		if g != "" && shown < limit {
			fmt.Fprintf(&buf, "+%s\n", g)
			shown++
		}
	}
	if shown == 0 {
		buf.WriteString("(no line-level differences found after normalize; raw lengths differ)\n")
	} else if shown >= limit {
		buf.WriteString("... (diff truncated)\n")
	}
	return buf.String()
}

func formatDiag(d *mapping.Diagnostics) string {
	var b bytes.Buffer
	_ = d.WriteHuman(&b)
	return b.String()
}
