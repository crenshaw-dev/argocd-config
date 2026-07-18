// Package v1beta1 contains API Schema definitions for the argo.crenshaw.dev v1beta1 API group.
// v1beta1 is a thin conversion spoke used to demonstrate hub/spoke apiVersion conversion.
// +kubebuilder:object:generate=true
// +groupName=argo.crenshaw.dev
package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "argo.crenshaw.dev", Version: "v1beta1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
