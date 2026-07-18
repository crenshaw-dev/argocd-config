package convert

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	argov1beta1 "github.com/crenshaw-dev/argocd-config/api/v1beta1"
)

// NewScheme returns a scheme with ArgoCDConfiguration hub and spoke versions registered.
func NewScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = argov1alpha1.AddToScheme(s)
	_ = argov1beta1.AddToScheme(s)
	return s
}

// ToVersion converts an ArgoCDConfiguration to the requested apiVersion.
func ToVersion(obj runtime.Object, toAPIVersion string) (runtime.Object, error) {
	gv, err := parseAPIVersion(toAPIVersion)
	if err != nil {
		return nil, err
	}

	hubGV := argov1alpha1.GroupVersion.String()
	spokeGV := argov1beta1.GroupVersion.String()

	switch gv {
	case hubGV:
		hub, err := toHub(obj)
		if err != nil {
			return nil, err
		}
		hub.TypeMeta = metav1.TypeMeta{
			APIVersion: hubGV,
			Kind:       "ArgoCDConfiguration",
		}
		return hub, nil
	case spokeGV:
		hub, err := toHub(obj)
		if err != nil {
			return nil, err
		}
		spoke := &argov1beta1.ArgoCDConfiguration{}
		if err := spoke.ConvertFrom(hub); err != nil {
			return nil, fmt.Errorf("convert from hub: %w", err)
		}
		spoke.TypeMeta = metav1.TypeMeta{
			APIVersion: spokeGV,
			Kind:       "ArgoCDConfiguration",
		}
		return spoke, nil
	default:
		return nil, fmt.Errorf("unsupported target apiVersion %q (registered: %s, %s)", toAPIVersion, hubGV, spokeGV)
	}
}

func toHub(obj runtime.Object) (*argov1alpha1.ArgoCDConfiguration, error) {
	if cfg, ok := obj.(*argov1alpha1.ArgoCDConfiguration); ok {
		return cfg.DeepCopy(), nil
	}
	if convertible, ok := obj.(conversion.Convertible); ok {
		hub := &argov1alpha1.ArgoCDConfiguration{}
		if err := convertible.ConvertTo(hub); err != nil {
			return nil, fmt.Errorf("convert to hub: %w", err)
		}
		return hub, nil
	}
	return nil, fmt.Errorf("expected *ArgoCDConfiguration or conversion.Convertible, got %T", obj)
}

func parseAPIVersion(apiVersion string) (string, error) {
	if apiVersion == "" {
		return "", fmt.Errorf("apiVersion is required")
	}
	return apiVersion, nil
}
