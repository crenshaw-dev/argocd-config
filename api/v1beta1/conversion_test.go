package v1beta1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

type stubHub struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (*stubHub) Hub() {}

func (s *stubHub) DeepCopyObject() runtime.Object {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

func TestConvertToConvertFromURLRoundTrip(t *testing.T) {
	const wantURL = "https://roundtrip.example.com"

	spoke := &ArgoCDConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: GroupVersion.String(),
			Kind:       "ArgoCDConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mapping.DefaultConfigurationName,
			Namespace: "argocd",
		},
		Spec: Spec{URL: wantURL},
	}

	hub := &argov1alpha1.ArgoCDConfiguration{}
	if err := spoke.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if hub.Spec.Server == nil || hub.Spec.Server.URL != wantURL {
		t.Fatalf("hub URL = %#v, want %q", hub.Spec.Server, wantURL)
	}

	back := &ArgoCDConfiguration{}
	if err := back.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if back.Spec.URL != wantURL {
		t.Fatalf("spoke URL = %q, want %q", back.Spec.URL, wantURL)
	}
}

func TestConvertToWrongHubType(t *testing.T) {
	spoke := &ArgoCDConfiguration{}
	if err := spoke.ConvertTo(&stubHub{}); err == nil {
		t.Fatal("expected error for wrong hub type")
	}
}

func TestConvertFromWrongHubType(t *testing.T) {
	dst := &ArgoCDConfiguration{}
	if err := dst.ConvertFrom(&stubHub{}); err == nil {
		t.Fatal("expected error for wrong hub type")
	}
}
