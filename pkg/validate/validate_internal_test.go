package validate

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestNewValidatorsRejectsBadYAML(t *testing.T) {
	_, err := newValidators([]byte("{not yaml"))
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestNewValidatorsRejectsNonCRD(t *testing.T) {
	_, err := newValidators([]byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: x
`))
	if err == nil {
		t.Fatal("expected schema/version error for non-CRD")
	}
}

func TestHasBlockingErr(t *testing.T) {
	if hasBlockingErr(nil) {
		t.Fatal("empty list should not block")
	}
	if !hasBlockingErr(field.ErrorList{field.Required(field.NewPath("spec"), "missing")}) {
		t.Fatal("Required should block")
	}
	if !hasBlockingErr(field.ErrorList{field.TooLong(field.NewPath("name"), "x", 1)}) {
		t.Fatal("TooLong should block")
	}
	if !hasBlockingErr(field.ErrorList{field.TooMany(field.NewPath("items"), 2, 1)}) {
		t.Fatal("TooMany should block")
	}
	if !hasBlockingErr(field.ErrorList{field.TypeInvalid(field.NewPath("spec"), nil, "wrong type")}) {
		t.Fatal("TypeInvalid should block")
	}
	if !hasBlockingErr(field.ErrorList{field.NotSupported(field.NewPath("mode"), "x", []string{"a"})}) {
		t.Fatal("NotSupported should block")
	}
	if hasBlockingErr(field.ErrorList{field.Invalid(field.NewPath("spec"), "x", "cel rule")}) {
		t.Fatal("Invalid alone should not block CEL")
	}
}
