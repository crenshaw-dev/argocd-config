package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/conversion"

// Hub marks ArgoCDConfiguration as the conversion hub for future apiVersions.
// Spoke versions implement Convertible to convert via this hub.
func (*ArgoCDConfiguration) Hub() {}

// Ensure the conversion package is referenced so Hub satisfies conversion.Hub.
var _ conversion.Hub = &ArgoCDConfiguration{}
