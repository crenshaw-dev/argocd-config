package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ArgoCDConfiguration is the v1beta1 spoke (hub is v1alpha1).
// This thin version carries only spec.url for conversion demonstration.
//
// +kubebuilder:object:root=true
type ArgoCDConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec Spec `json:"spec,omitempty"`
}

// Spec holds the v1beta1 spoke configuration surface.
type Spec struct {
	// URL is mirrored from hub spec.server.urls[0] for conversion demo.
	// +optional
	URL string `json:"url,omitempty"`
}

// ArgoCDConfigurationList contains a list of ArgoCDConfiguration.
//
// +kubebuilder:object:root=true
type ArgoCDConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ArgoCDConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ArgoCDConfiguration{}, &ArgoCDConfigurationList{})
}
