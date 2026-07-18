package mapping_test

import (
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

func sampleCMSDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "sample-cms")
}

func loadSampleCMS(t *testing.T) mapping.ConfigMaps {
	t.Helper()
	dir := sampleCMSDir(t)
	return mapping.ConfigMaps{
		CM:        mustReadCM(t, filepath.Join(dir, "argocd-cm.yaml")),
		CmdParams: mustReadCM(t, filepath.Join(dir, "argocd-cmd-params-cm.yaml")),
		RBAC:      mustReadCM(t, filepath.Join(dir, "argocd-rbac-cm.yaml")),
	}
}

func cloneCMData(data map[string]string) map[string]string {
	if data == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(data))
	for k, v := range data {
		out[k] = v
	}
	return out
}

func cmWithData(base *corev1.ConfigMap, extra map[string]string) *corev1.ConfigMap {
	cm := base.DeepCopy()
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	for k, v := range extra {
		cm.Data[k] = v
	}
	return cm
}

func hasDiagnostic(items []mapping.Diagnostic, sev mapping.Severity, keySubstr, msgSubstr string) bool {
	for _, it := range items {
		if sev != "" && it.Severity != sev {
			continue
		}
		if keySubstr != "" && !strings.Contains(it.Key, keySubstr) {
			continue
		}
		if msgSubstr != "" && !strings.Contains(it.Message, msgSubstr) {
			continue
		}
		return true
	}
	return false
}
