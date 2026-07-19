package convert_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	argov1beta1 "github.com/crenshaw-dev/argocd-config/api/v1beta1"
	"github.com/crenshaw-dev/argocd-config/pkg/convert"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

func hubWithURL(url string) *argov1alpha1.ArgoCDConfiguration {
	return &argov1alpha1.ArgoCDConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: argov1alpha1.GroupVersion.String(),
			Kind:       "ArgoCDConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{Name: mapping.DefaultConfigurationName, Namespace: "argocd"},
		Spec: argov1alpha1.ArgoCDConfigurationSpec{
			Server: &argov1alpha1.ServerConfig{URLs: []string{url}},
		},
	}
}

func TestConvertSameVersionRoundTrip(t *testing.T) {
	in := hubWithURL("https://argocd.example.com")
	out, err := convert.ToVersion(in, argov1alpha1.GroupVersion.String())
	if err != nil {
		t.Fatal(err)
	}
	cfg := out.(*argov1alpha1.ArgoCDConfiguration)
	if cfg.Spec.Server == nil || len(cfg.Spec.Server.URLs) != 1 || cfg.Spec.Server.URLs[0] != "https://argocd.example.com" {
		t.Fatalf("got %#v", cfg.Spec.Server)
	}
	if cfg == in {
		t.Fatal("expected deep copy, got same pointer")
	}
}

func TestConvertHubToSpoke(t *testing.T) {
	in := hubWithURL("https://argocd.example.com")
	out, err := convert.ToVersion(in, argov1beta1.GroupVersion.String())
	if err != nil {
		t.Fatal(err)
	}
	spoke, ok := out.(*argov1beta1.ArgoCDConfiguration)
	if !ok {
		t.Fatalf("expected *v1beta1.ArgoCDConfiguration, got %T", out)
	}
	if spoke.Spec.URL != "https://argocd.example.com" {
		t.Fatalf("spoke URL = %q", spoke.Spec.URL)
	}
	if spoke.Name != mapping.DefaultConfigurationName {
		t.Fatalf("name = %q", spoke.Name)
	}
}

func TestConvertSpokeToHub(t *testing.T) {
	spoke := &argov1beta1.ArgoCDConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: argov1beta1.GroupVersion.String(),
			Kind:       "ArgoCDConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{Name: mapping.DefaultConfigurationName, Namespace: "argocd"},
		Spec:       argov1beta1.Spec{URL: "https://spoke.example.com"},
	}
	out, err := convert.ToVersion(spoke, argov1alpha1.GroupVersion.String())
	if err != nil {
		t.Fatal(err)
	}
	hub := out.(*argov1alpha1.ArgoCDConfiguration)
	if hub.Spec.Server == nil || len(hub.Spec.Server.URLs) != 1 || hub.Spec.Server.URLs[0] != "https://spoke.example.com" {
		t.Fatalf("hub server URL = %#v", hub.Spec.Server)
	}
}

func TestConvertHubSpokeRoundTrip(t *testing.T) {
	in := hubWithURL("https://roundtrip.example.com")
	spokeObj, err := convert.ToVersion(in, argov1beta1.GroupVersion.String())
	if err != nil {
		t.Fatal(err)
	}
	hubObj, err := convert.ToVersion(spokeObj, argov1alpha1.GroupVersion.String())
	if err != nil {
		t.Fatal(err)
	}
	hub := hubObj.(*argov1alpha1.ArgoCDConfiguration)
	if hub.Spec.Server == nil || len(hub.Spec.Server.URLs) != 1 || hub.Spec.Server.URLs[0] != "https://roundtrip.example.com" {
		t.Fatalf("round-trip URL = %#v", hub.Spec.Server)
	}
}

func TestConvertUnknownVersion(t *testing.T) {
	in := hubWithURL("")
	_, err := convert.ToVersion(in, "argoproj.io/v99")
	if err == nil {
		t.Fatal("expected error for unknown version")
	}
}

func TestConvertEmptyToVersion(t *testing.T) {
	in := hubWithURL("https://argocd.example.com")
	_, err := convert.ToVersion(in, "")
	if err == nil {
		t.Fatal("expected error for empty to-version")
	}
}

func TestNewSchemeRegistersBothVersions(t *testing.T) {
	s := convert.NewScheme()

	hub := &argov1alpha1.ArgoCDConfiguration{}
	hubKinds, _, err := s.ObjectKinds(hub)
	if err != nil || len(hubKinds) == 0 {
		t.Fatalf("hub not registered: %v", err)
	}
	if hubKinds[0].GroupVersion().String() != argov1alpha1.GroupVersion.String() {
		t.Fatalf("hub GVK = %v", hubKinds[0])
	}

	spoke := &argov1beta1.ArgoCDConfiguration{}
	spokeKinds, _, err := s.ObjectKinds(spoke)
	if err != nil || len(spokeKinds) == 0 {
		t.Fatalf("spoke not registered: %v", err)
	}
	if spokeKinds[0].GroupVersion().String() != argov1beta1.GroupVersion.String() {
		t.Fatalf("spoke GVK = %v", spokeKinds[0])
	}
}

func FuzzConvertRoundTrip(f *testing.F) {
	f.Add("https://argocd.example.com")
	f.Add("")
	f.Add("http://localhost:8080/path")

	f.Fuzz(func(t *testing.T, urlStr string) {
		in := hubWithURL(urlStr)
		spokeObj, err := convert.ToVersion(in, argov1beta1.GroupVersion.String())
		if err != nil {
			t.Fatalf("hub->spoke: %v", err)
		}
		spoke, ok := spokeObj.(*argov1beta1.ArgoCDConfiguration)
		if !ok {
			t.Fatalf("expected spoke, got %T", spokeObj)
		}
		if spoke.Spec.URL != urlStr {
			t.Fatalf("spoke URL = %q, want %q", spoke.Spec.URL, urlStr)
		}

		hubObj, err := convert.ToVersion(spokeObj, argov1alpha1.GroupVersion.String())
		if err != nil {
			t.Fatalf("spoke->hub: %v", err)
		}
		hub, ok := hubObj.(*argov1alpha1.ArgoCDConfiguration)
		if !ok {
			t.Fatalf("expected hub, got %T", hubObj)
		}
		got := ""
		if hub.Spec.Server != nil && len(hub.Spec.Server.URLs) > 0 {
			got = hub.Spec.Server.URLs[0]
		}
		if got != urlStr {
			t.Fatalf("hub URL = %q, want %q", got, urlStr)
		}
	})
}

// Ensure compile-time interface satisfaction for fuzz imports.
var _ runtime.Object = (*argov1beta1.ArgoCDConfiguration)(nil)
