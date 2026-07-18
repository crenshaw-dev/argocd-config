package mapping_test

import (
	"bytes"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

func TestUnknownKeyEmitsWarn(t *testing.T) {
	cms := mapping.ConfigMaps{
		CM: cmWithData(&corev1.ConfigMap{}, map[string]string{
			"url":                 "https://example.com",
			"totally.unknown.key": "drop-me",
		}),
	}
	_, diag, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("FromConfigMaps: %v", err)
	}
	if !hasDiagnostic(diag.Items(), mapping.SeverityWarn, "totally.unknown.key", "unhandled key") {
		t.Fatalf("expected unknown key warning, got %#v", diag.Items())
	}
}

func TestParseErrorBadDurationEmitsError(t *testing.T) {
	cms := mapping.ConfigMaps{
		CM: cmWithData(&corev1.ConfigMap{}, map[string]string{
			"timeout.reconciliation": "not-a-duration",
		}),
	}
	_, diag, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !diag.HasErrors() {
		t.Fatalf("expected error diagnostics, got %#v", diag.Items())
	}
	if !hasDiagnostic(diag.Items(), mapping.SeverityError, "timeout.reconciliation", "invalid duration") {
		t.Fatalf("expected duration parse error, got %#v", diag.Items())
	}
}

func TestParseErrorBadQuantityEmitsError(t *testing.T) {
	cms := mapping.ConfigMaps{
		CM: cmWithData(&corev1.ConfigMap{}, map[string]string{
			"webhook.maxPayloadSizeMB": "not-a-number",
		}),
	}
	_, diag, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !diag.HasErrors() {
		t.Fatalf("expected quantity parse error diagnostic, got %#v", diag.Items())
	}
	if !hasDiagnostic(diag.Items(), mapping.SeverityError, "webhook.maxPayloadSizeMB", "invalid quantity") {
		t.Fatalf("expected quantity parse error, got %#v", diag.Items())
	}
}

func TestCompareOptionsOffToNoneEmitsWarn(t *testing.T) {
	cms := mapping.ConfigMaps{
		CM: cmWithData(&corev1.ConfigMap{}, map[string]string{
			"resource.compareoptions": "ignoreResourceStatusField: \"off\"\n",
		}),
	}
	cfg, diag, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("FromConfigMaps: %v", err)
	}
	if !hasDiagnostic(diag.Items(), mapping.SeverityWarn, "resource.compareoptions", "normalized") {
		t.Fatalf("expected normalization warning, got %#v", diag.Items())
	}
	co := cfg.Spec.Controller.Resource.CompareOptions
	if co == nil || co.IgnoreResourceStatusField != "none" {
		t.Fatalf("expected ignoreResourceStatusField=none, got %#v", co)
	}
}

func TestWebhookMaxPayloadSizeMBEmitsWarn(t *testing.T) {
	cms := mapping.ConfigMaps{
		CM: cmWithData(&corev1.ConfigMap{}, map[string]string{
			"webhook.maxPayloadSizeMB": "50",
		}),
	}
	_, diag, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("FromConfigMaps: %v", err)
	}
	if !hasDiagnostic(diag.Items(), mapping.SeverityWarn, "webhook.maxPayloadSizeMB", "lose precision") {
		t.Fatalf("expected webhook precision warning, got %#v", diag.Items())
	}
}

func TestDualRepoServerClientPrefixCollapseEmitsWarn(t *testing.T) {
	cms := mapping.ConfigMaps{
		CmdParams: cmWithData(&corev1.ConfigMap{}, map[string]string{
			"controller.repo.server.timeout.seconds": "120",
			"server.repo.server.timeout.seconds":     "120",
		}),
	}
	_, diag, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("FromConfigMaps: %v", err)
	}
	if !hasDiagnostic(diag.Items(), mapping.SeverityWarn, "controller.repo.server.timeout.seconds", "collapsed") {
		t.Fatalf("expected dual-prefix collapse warning, got %#v", diag.Items())
	}
}

func TestDiagnosticsWriteHumanWriteJSONHasErrorsHasWarnings(t *testing.T) {
	diag := &mapping.Diagnostics{}
	diag.Warn(mapping.DirCMToCR, "foo", "warn message")
	diag.Error(mapping.DirCMToCR, "bar", "error message")
	diag.Info(mapping.DirCRToCM, "baz", "info message")

	if !diag.HasWarnings() {
		t.Fatal("expected HasWarnings true")
	}
	if !diag.HasErrors() {
		t.Fatal("expected HasErrors true")
	}
	if diag.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", diag.Len())
	}

	var human bytes.Buffer
	if err := diag.WriteHuman(&human); err != nil {
		t.Fatalf("WriteHuman: %v", err)
	}
	humanStr := human.String()
	for _, want := range []string{"ERROR (1):", "WARN (1):", "INFO (1):", "[cm->cr] bar:", "[cm->cr] foo:", "[cr->cm] baz:"} {
		if !strings.Contains(humanStr, want) {
			t.Fatalf("WriteHuman missing %q in:\n%s", want, humanStr)
		}
	}

	var jsonBuf bytes.Buffer
	if err := diag.WriteJSON(&jsonBuf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	jsonStr := jsonBuf.String()
	for _, want := range []string{`"severity": "error"`, `"severity": "warn"`, `"severity": "info"`, `"key": "bar"`} {
		if !strings.Contains(jsonStr, want) {
			t.Fatalf("WriteJSON missing %q in:\n%s", want, jsonStr)
		}
	}

	empty := &mapping.Diagnostics{}
	if empty.HasErrors() || empty.HasWarnings() {
		t.Fatal("empty diagnostics should not report errors or warnings")
	}
}

func TestToConfigMapsWithoutSourceEmitsMetadataPreservationWarn(t *testing.T) {
	cms := loadSampleCMS(t)
	cfg, _, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("FromConfigMaps: %v", err)
	}
	_, diag, err := mapping.ToConfigMaps(cfg, "argocd")
	if err != nil {
		t.Fatalf("ToConfigMaps: %v", err)
	}
	for _, name := range []string{mapping.ArgoCDCMName, mapping.ArgoCDCmdParamsCMName, mapping.ArgoCDRBACCMName} {
		if !hasDiagnostic(diag.Items(), mapping.SeverityWarn, name, "no source ConfigMap provided") {
			t.Fatalf("expected metadata preservation warning for %s, got %#v", name, diag.Items())
		}
	}
}
