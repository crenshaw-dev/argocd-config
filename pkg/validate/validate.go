package validate

import (
	"context"
	"fmt"
	"sync"

	apiextensionsinternal "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	structurallisttype "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/listtype"
	apiservervalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	"github.com/crenshaw-dev/argocd-config/config/crd/bases"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

// crdVersion is the hub schema version used for offline OpenAPI/CEL validation.
const crdVersion = "v1alpha1"

type crdValidators struct {
	structural *structuralschema.Structural
	schema     apiservervalidation.SchemaValidator
	cel        *cel.Validator
}

var (
	validatorsOnce sync.Once
	validators     *crdValidators
	validatorsErr  error
)

// Validate performs offline OpenAPI + CEL + list-type validation against the
// embedded ArgoCDConfiguration CRD schema (same libraries the apiserver uses).
func Validate(cfg *argov1alpha1.ArgoCDConfiguration) *mapping.Diagnostics {
	diag := &mapping.Diagnostics{}
	if cfg == nil {
		diag.Error("", "metadata", "configuration is nil")
		return diag
	}

	if cfg.Kind != "" && cfg.Kind != "ArgoCDConfiguration" {
		diag.Error("", "kind",
			fmt.Sprintf("kind must be %q, got %q", "ArgoCDConfiguration", cfg.Kind))
	}

	if cfg.APIVersion != "" &&
		cfg.APIVersion != argov1alpha1.GroupVersion.String() &&
		cfg.APIVersion != "argoproj.io/v1beta1" {
		diag.Warn("", "apiVersion",
			fmt.Sprintf("expected apiVersion %q or v1beta1 spoke, got %q", argov1alpha1.GroupVersion.String(), cfg.APIVersion))
	}

	v, err := loadValidators()
	if err != nil {
		diag.Error("", "crd", fmt.Sprintf("failed to load CRD validators: %v", err))
		return diag
	}

	obj, err := toUnstructured(cfg)
	if err != nil {
		diag.Error("", "object", fmt.Sprintf("failed to convert to unstructured: %v", err))
		return diag
	}

	var errs field.ErrorList
	errs = append(errs, apiservervalidation.ValidateCustomResource(nil, obj, v.schema)...)
	errs = append(errs, structurallisttype.ValidateListSetsAndMaps(nil, v.structural, obj)...)
	if v.cel != nil && !hasBlockingErr(errs) {
		celErrs, _ := v.cel.Validate(context.Background(), nil, v.structural, obj, nil, celconfig.RuntimeCELCostBudget)
		errs = append(errs, celErrs...)
	}

	for _, e := range errs {
		path := e.Field
		diag.Error("", path, e.Error())
	}
	return diag
}

// ValidateAgainstCRD verifies the embedded CRD can be compiled into validators.
func ValidateAgainstCRD() *mapping.Diagnostics {
	diag := &mapping.Diagnostics{}
	if _, err := loadValidators(); err != nil {
		diag.Error("", "crd", err.Error())
	}
	return diag
}

func loadValidators() (*crdValidators, error) {
	validatorsOnce.Do(func() {
		validators, validatorsErr = newValidators(bases.ArgoCDConfigurationCRD)
	})
	return validators, validatorsErr
}

func newValidators(crdYAML []byte) (*crdValidators, error) {
	var v1crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(crdYAML, &v1crd); err != nil {
		return nil, fmt.Errorf("decode CRD: %w", err)
	}

	var internal apiextensionsinternal.CustomResourceDefinition
	if err := apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(&v1crd, &internal, nil); err != nil {
		return nil, fmt.Errorf("convert CRD to internal: %w", err)
	}

	schema, err := apiextensionsinternal.GetSchemaForVersion(&internal, crdVersion)
	if err != nil {
		return nil, err
	}
	if schema == nil || schema.OpenAPIV3Schema == nil {
		return nil, fmt.Errorf("CRD version %q has no OpenAPI schema", crdVersion)
	}

	structural, err := structuralschema.NewStructural(schema.OpenAPIV3Schema)
	if err != nil {
		return nil, fmt.Errorf("structural schema: %w", err)
	}

	schemaValidator, _, err := apiservervalidation.NewSchemaValidator(schema.OpenAPIV3Schema)
	if err != nil {
		return nil, fmt.Errorf("schema validator: %w", err)
	}

	return &crdValidators{
		structural: structural,
		schema:     schemaValidator,
		cel:        cel.NewValidator(structural, true /* isResourceRoot */, celconfig.PerCallLimit),
	}, nil
}

func toUnstructured(cfg *argov1alpha1.ArgoCDConfiguration) (map[string]any, error) {
	cp := cfg.DeepCopy()
	cp.SetGroupVersionKind(argov1alpha1.GroupVersion.WithKind("ArgoCDConfiguration"))
	return runtime.DefaultUnstructuredConverter.ToUnstructured(cp)
}

// hasBlockingErr mirrors apiserver customresource strategy: skip CEL when the
// object is too malformed for expression evaluation.
func hasBlockingErr(errs field.ErrorList) bool {
	for _, err := range errs {
		if err.Type == field.ErrorTypeNotSupported ||
			err.Type == field.ErrorTypeRequired ||
			err.Type == field.ErrorTypeTooLong ||
			err.Type == field.ErrorTypeTooMany ||
			err.Type == field.ErrorTypeTypeInvalid {
			return true
		}
	}
	return false
}
