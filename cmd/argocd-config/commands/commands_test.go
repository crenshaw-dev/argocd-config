package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	argov1beta1 "github.com/crenshaw-dev/argocd-config/api/v1beta1"
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

func testdataPath(t *testing.T, elems ...string) string {
	t.Helper()
	return filepath.Join(append([]string{moduleRoot(t), "testdata"}, elems...)...)
}

// sampleCMSFlags returns --cm/--cmd-params/--rbac pointing at testdata/sample-cms files.
func sampleCMSFlags(t *testing.T) []string {
	t.Helper()
	dir := testdataPath(t, "sample-cms")
	return []string{
		"--cm", filepath.Join(dir, "argocd-cm.yaml"),
		"--cmd-params", filepath.Join(dir, "argocd-cmd-params-cm.yaml"),
		"--rbac", filepath.Join(dir, "argocd-rbac-cm.yaml"),
	}
}

func writeMinimalCR(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "argocd-config.yaml")
	const crYAML = `apiVersion: argo.crenshaw.dev/v1alpha1
kind: ArgoCDConfiguration
metadata:
  name: argocd-config
  namespace: argocd
spec:
  server:
    urls:
    - https://argocd.example.com
`
	if err := os.WriteFile(path, []byte(crYAML), 0o644); err != nil {
		t.Fatalf("write CR: %v", err)
	}
	return path
}

func captureStdio(t *testing.T, fn func(root *cobra.Command)) (stdout, stderr string) {
	t.Helper()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout = outW
	os.Stderr = errW
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})

	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	fn(root)

	outW.Close()
	errW.Close()
	outBytes, err := io.ReadAll(outR)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	errBytes, err := io.ReadAll(errR)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(outBytes), string(errBytes)
}

func TestVersionCommandPrintsVersion(t *testing.T) {
	const wantVersion = "test-version-1.2.3"
	Version = wantVersion

	cmd := newVersionCommand()
	cmd.SetArgs(nil)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	w.Close()
	outBytes, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	out := string(outBytes)
	if !strings.Contains(out, wantVersion) {
		t.Fatalf("output %q does not contain version %q", out, wantVersion)
	}
	if !strings.Contains(out, "argocd-config") {
		t.Fatalf("output %q does not contain program name", out)
	}
}

func TestFromConfigMapsWithTestdata(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "out.yaml")

	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{
		"from-configmaps",
		"--output", outFile,
		"--no-validate",
	}, sampleCMSFlags(t)...))

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	b, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	out := string(b)
	if !strings.Contains(out, "name: argocd-config") {
		t.Fatalf("output missing argocd-config name:\n%s", out)
	}
	if !strings.Contains(out, "kind: ArgoCDConfiguration") {
		t.Fatalf("output missing ArgoCDConfiguration kind:\n%s", out)
	}
}

func TestFromConfigMapsStrictUnknownKeyExitsNonZero(t *testing.T) {
	cmPath := filepath.Join(t.TempDir(), "argocd-cm.yaml")
	const cmYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  url: https://example.com
  totally.unknown.key: dropped
`
	if err := os.WriteFile(cmPath, []byte(cmYAML), 0o644); err != nil {
		t.Fatalf("write cm: %v", err)
	}

	outFile := filepath.Join(t.TempDir(), "out.yaml")
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	root.SetArgs([]string{
		"from-configmaps",
		"--cm", cmPath,
		"--output", outFile,
		"--no-validate",
		"--strict",
	})

	execErr := root.Execute()
	w.Close()
	stderrBytes, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	stderrStr := string(stderrBytes)

	if execErr == nil {
		t.Fatal("expected non-zero exit with --strict and unknown key")
	}
	if code := ExitCode(execErr); code == 0 {
		t.Fatalf("ExitCode = 0, want non-zero; err=%v stderr=%s", execErr, stderrStr)
	}
	if !strings.Contains(stderrStr, "totally.unknown.key") {
		t.Fatalf("stderr missing unknown key diagnostic: %s", stderrStr)
	}
}

func TestToConfigMapsWritesDirectory(t *testing.T) {
	crFile := writeMinimalCR(t, t.TempDir())
	outDir := t.TempDir()

	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"to-configmaps",
		"--file", crFile,
		"--output", outDir,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	cmPath := filepath.Join(outDir, "argocd-cm.yaml")
	if _, err := os.Stat(cmPath); err != nil {
		t.Fatalf("argocd-cm.yaml not written: %v", err)
	}
	b, err := os.ReadFile(cmPath)
	if err != nil {
		t.Fatalf("read cm: %v", err)
	}
	if !strings.Contains(string(b), "name: argocd-cm") {
		t.Fatalf("unexpected cm content:\n%s", b)
	}
}

func TestToConfigMapsStdoutMultiDoc(t *testing.T) {
	crFile := writeMinimalCR(t, t.TempDir())

	stdout, _ := captureStdio(t, func(root *cobra.Command) {
		root.SetArgs([]string{
			"to-configmaps",
			"--file", crFile,
			"--output", "-",
		})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	if !strings.Contains(stdout, "kind: ConfigMap") {
		t.Fatalf("stdout missing ConfigMap:\n%s", stdout)
	}
	if !strings.Contains(stdout, "---") {
		t.Fatalf("stdout missing multi-doc separator:\n%s", stdout)
	}
	if strings.Count(stdout, "kind: ConfigMap") < 2 {
		t.Fatalf("expected multiple ConfigMaps on stdout:\n%s", stdout)
	}
}

func TestToConfigMapsSourcePreservesMetadata(t *testing.T) {
	sampleDir := testdataPath(t, "sample-cms")
	crFile := filepath.Join(t.TempDir(), "cr.yaml")

	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{
		"from-configmaps",
		"--output", crFile,
		"--no-validate",
	}, sampleCMSFlags(t)...))
	if err := root.Execute(); err != nil {
		t.Fatalf("from-configmaps: %v", err)
	}

	outDir := t.TempDir()
	root = NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"to-configmaps",
		"--file", crFile,
		"--source-cm", filepath.Join(sampleDir, "argocd-cm.yaml"),
		"--source-cmd-params", filepath.Join(sampleDir, "argocd-cmd-params-cm.yaml"),
		"--source-rbac", filepath.Join(sampleDir, "argocd-rbac-cm.yaml"),
		"--output", outDir,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("to-configmaps: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(outDir, "argocd-cm.yaml"))
	if err != nil {
		t.Fatalf("read cm: %v", err)
	}
	out := string(b)
	if !strings.Contains(out, "app.kubernetes.io/part-of: argocd") {
		t.Fatalf("metadata label not preserved:\n%s", out)
	}
}

func TestConvertRoundTripVersions(t *testing.T) {
	crFile := writeMinimalCR(t, t.TempDir())
	spokeFile := filepath.Join(t.TempDir(), "spoke.yaml")
	hubFile := filepath.Join(t.TempDir(), "hub.yaml")

	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"convert",
		"--file", crFile,
		"--to-version", argov1beta1.GroupVersion.String(),
		"--output", spokeFile,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("convert to v1beta1: %v", err)
	}

	spokeBytes, err := os.ReadFile(spokeFile)
	if err != nil {
		t.Fatalf("read spoke: %v", err)
	}
	spokeOut := string(spokeBytes)
	if !strings.Contains(spokeOut, argov1beta1.GroupVersion.String()) {
		t.Fatalf("spoke missing v1beta1 apiVersion:\n%s", spokeBytes)
	}
	if strings.Contains(spokeOut, "server:") {
		t.Fatalf("spoke should use flat spec.url, not spec.server:\n%s", spokeBytes)
	}
	if !strings.Contains(spokeOut, "url: https://argocd.example.com") {
		t.Fatalf("spoke missing URL:\n%s", spokeBytes)
	}

	root = NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"convert",
		"--file", spokeFile,
		"--to-version", argov1alpha1.GroupVersion.String(),
		"--output", hubFile,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("convert to v1alpha1: %v", err)
	}

	hubBytes, err := os.ReadFile(hubFile)
	if err != nil {
		t.Fatalf("read hub: %v", err)
	}
	hubOut := string(hubBytes)
	if !strings.Contains(hubOut, argov1alpha1.GroupVersion.String()) {
		t.Fatalf("hub missing v1alpha1 apiVersion:\n%s", hubBytes)
	}
	if !strings.Contains(hubOut, "https://argocd.example.com") {
		t.Fatalf("hub missing URL after round trip:\n%s", hubBytes)
	}
}

func TestValidateGoodFile(t *testing.T) {
	crFile := writeMinimalCR(t, t.TempDir())

	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"validate",
		"--file", crFile,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("validate good file: %v", err)
	}
	if ExitCode(nil) != 0 {
		t.Fatal("expected exit code 0 for success")
	}
}

func TestValidateWrongNameFails(t *testing.T) {
	dir := t.TempDir()
	crFile := filepath.Join(dir, "bad-name.yaml")
	const crYAML = `apiVersion: argo.crenshaw.dev/v1alpha1
kind: ArgoCDConfiguration
metadata:
  name: not-argocd-config
  namespace: argocd
spec:
  server:
    urls:
    - https://argocd.example.com
`
	if err := os.WriteFile(crFile, []byte(crYAML), 0o644); err != nil {
		t.Fatalf("write CR: %v", err)
	}

	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	_, stderr := captureStdio(t, func(root *cobra.Command) {
		root.SetArgs([]string{"validate", "--file", crFile})
		execErr := root.Execute()
		if execErr == nil {
			t.Fatal("expected validation error for wrong name")
		}
		if _, ok := execErr.(*exitError); !ok {
			t.Fatalf("expected *exitError, got %T: %v", execErr, execErr)
		}
		if code := ExitCode(execErr); code != 1 {
			t.Fatalf("ExitCode = %d, want 1", code)
		}
	})
	if !strings.Contains(stderr, "metadata.name") {
		t.Fatalf("stderr missing name diagnostic: %s", stderr)
	}
}

func TestFromConfigMapsSelfCheck(t *testing.T) {
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{
		"from-configmaps",
		"--self-check",
		"--output", "/dev/null",
		"--no-validate",
	}, sampleCMSFlags(t)...))
	if err := root.Execute(); err != nil {
		t.Fatalf("self-check round trip: %v", err)
	}
}

func TestReportJSONDiagnosticsOnStderr(t *testing.T) {
	cmPath := filepath.Join(t.TempDir(), "argocd-cm.yaml")
	const cmYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  url: https://example.com
  totally.unknown.key: dropped
`
	if err := os.WriteFile(cmPath, []byte(cmYAML), 0o644); err != nil {
		t.Fatalf("write cm: %v", err)
	}

	outFile := filepath.Join(t.TempDir(), "out.yaml")
	_, stderr := captureStdio(t, func(root *cobra.Command) {
		root.SetArgs([]string{
			"from-configmaps",
			"--cm", cmPath,
			"--output", outFile,
			"--no-validate",
			"--report", "json",
		})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	if !strings.Contains(stderr, "totally.unknown.key") {
		t.Fatalf("stderr missing unknown key: %s", stderr)
	}
	var payload any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &payload); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\n%s", err, stderr)
	}
}

func TestFromConfigMapsFromClusterConflictsWithFiles(t *testing.T) {
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"from-configmaps",
		"--from-cluster",
		"--cm", filepath.Join(testdataPath(t, "sample-cms"), "argocd-cm.yaml"),
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when combining --from-cluster and --cm")
	}
	if !strings.Contains(err.Error(), "--from-cluster") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigMapsInputRequiresSource(t *testing.T) {
	_, err := loadConfigMapsInput(t.Context(), false, "", "", "argocd", "", "", "")
	if err == nil {
		t.Fatal("expected error when no source provided")
	}
}

func TestReadConfigurationFromStdin(t *testing.T) {
	const crYAML = `apiVersion: argo.crenshaw.dev/v1alpha1
kind: ArgoCDConfiguration
metadata:
  name: argocd-config
  namespace: argocd
spec:
  server:
    urls:
    - https://stdin.example.com
`
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	if _, err := w.WriteString(crYAML); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	w.Close()

	stdout, _ := captureStdio(t, func(root *cobra.Command) {
		root.SetArgs([]string{
			"validate",
			"--file", "-",
		})
		if err := root.Execute(); err != nil {
			t.Fatalf("validate stdin: %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestExitCodePlainErrorVsExitError(t *testing.T) {
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"validate",
		"--file", filepath.Join(t.TempDir(), "missing.yaml"),
	})
	plainErr := root.Execute()
	if plainErr == nil {
		t.Fatal("expected error for missing file")
	}
	if _, ok := plainErr.(*exitError); ok {
		t.Fatalf("missing file should not be *exitError, got %v", plainErr)
	}
	if code := ExitCode(plainErr); code != 1 {
		t.Fatalf("ExitCode(plainErr) = %d, want 1", code)
	}

	crFile := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(crFile, []byte(`apiVersion: argo.crenshaw.dev/v1alpha1
kind: ArgoCDConfiguration
metadata:
  name: wrong
spec: {}
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	root = NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"validate", "--file", crFile})
	exitErr := root.Execute()
	if exitErr == nil {
		t.Fatal("expected validation exit error")
	}
	ee, ok := exitErr.(*exitError)
	if !ok {
		t.Fatalf("validation failure should be *exitError, got %T: %v", exitErr, exitErr)
	}
	if ExitCode(exitErr) != ee.code {
		t.Fatalf("ExitCode = %d, want exitError.code %d", ExitCode(exitErr), ee.code)
	}
}
