package kube

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

func TestLoadConfigMaps(t *testing.T) {
	const ns = "argocd"

	t.Run("loads all three", func(t *testing.T) {
		cs := fake.NewSimpleClientset(
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: mapping.ArgoCDCMName, Namespace: ns},
				Data:       map[string]string{"url": "https://argocd.example.com"},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: mapping.ArgoCDCmdParamsCMName, Namespace: ns},
				Data:       map[string]string{"server.insecure": "true"},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: mapping.ArgoCDRBACCMName, Namespace: ns},
				Data:       map[string]string{"policy.default": "role:readonly"},
			},
		)
		got, err := LoadConfigMaps(context.Background(), ConfigMapsInNamespace(cs, ns), ns)
		if err != nil {
			t.Fatalf("LoadConfigMaps: %v", err)
		}
		if got.CM == nil || got.CM.Data["url"] != "https://argocd.example.com" {
			t.Fatalf("CM: %+v", got.CM)
		}
		if got.CmdParams == nil || got.CmdParams.Data["server.insecure"] != "true" {
			t.Fatalf("CmdParams: %+v", got.CmdParams)
		}
		if got.RBAC == nil || got.RBAC.Data["policy.default"] != "role:readonly" {
			t.Fatalf("RBAC: %+v", got.RBAC)
		}
	})

	t.Run("omits missing", func(t *testing.T) {
		cs := fake.NewSimpleClientset(
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: mapping.ArgoCDCMName, Namespace: ns},
				Data:       map[string]string{"url": "https://argocd.example.com"},
			},
		)
		got, err := LoadConfigMaps(context.Background(), ConfigMapsInNamespace(cs, ns), ns)
		if err != nil {
			t.Fatalf("LoadConfigMaps: %v", err)
		}
		if got.CM == nil {
			t.Fatal("expected CM")
		}
		if got.CmdParams != nil || got.RBAC != nil {
			t.Fatalf("expected nil optional CMs, got CmdParams=%v RBAC=%v", got.CmdParams, got.RBAC)
		}
	})

	t.Run("errors when none found", func(t *testing.T) {
		cs := fake.NewSimpleClientset()
		_, err := LoadConfigMaps(context.Background(), ConfigMapsInNamespace(cs, ns), ns)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
