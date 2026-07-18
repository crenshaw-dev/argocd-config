package v1beta1

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
)

var _ conversion.Convertible = &ArgoCDConfiguration{}

// ConvertTo converts this spoke to the hub version.
func (src *ArgoCDConfiguration) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*argov1alpha1.ArgoCDConfiguration)
	if !ok {
		return fmt.Errorf("expected hub type *argov1alpha1.ArgoCDConfiguration, got %T", dstRaw)
	}
	dst.ObjectMeta = *src.ObjectMeta.DeepCopy()
	dst.Spec = argov1alpha1.ArgoCDConfigurationSpec{}
	if src.Spec.URL != "" {
		dst.Spec.Server = &argov1alpha1.ServerConfig{URL: src.Spec.URL}
	}
	return nil
}

// ConvertFrom converts the hub version to this spoke.
func (dst *ArgoCDConfiguration) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*argov1alpha1.ArgoCDConfiguration)
	if !ok {
		return fmt.Errorf("expected hub type *argov1alpha1.ArgoCDConfiguration, got %T", srcRaw)
	}
	dst.ObjectMeta = *src.ObjectMeta.DeepCopy()
	dst.Spec = Spec{}
	if src.Spec.Server != nil {
		dst.Spec.URL = src.Spec.Server.URL
	}
	return nil
}
