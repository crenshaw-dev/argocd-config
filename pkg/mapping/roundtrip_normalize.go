package mapping

import (
	"reflect"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

const (
	impersonationEnabledKey  = "application.sync.impersonation.enabled"
	impersonationEnforcedKey = "application.sync.impersonation.enforced"
)

// ValueEqualer reports whether two ConfigMap data values are equivalent for a
// given key after applying a known-safe normalization. Return ok=false if this
// equaler does not apply to the key/values.
//
// Contributors: append to RoundTripValueEqualers when you discover a formatting
// or serialization difference that is semantically identical under --strict.
type ValueEqualer func(key, orig, round string) (equal bool, ok bool)

// RoundTripValueEqualers is the suite of known-safe round-trip value normalizations.
// Order does not matter; the first equaler that applies and reports equal wins.
var RoundTripValueEqualers = []ValueEqualer{
	durationValueEqual,
	yamlSemanticEqual,
}

// ConfigMapDataDiff is the result of comparing two ConfigMap data maps after
// known-safe normalizations.
type ConfigMapDataDiff struct {
	Missing []string // in orig, not in round
	Extra   []string // in round, not in orig
	Changed []string // in both, not equivalent after normalization
}

// DiffConfigMapDataNormalized compares ConfigMap data maps, ignoring known-safe
// differences (value equalers + key-pair canonicalization).
func DiffConfigMapDataNormalized(orig, round map[string]string) ConfigMapDataDiff {
	o := copyStringMap(orig)
	r := copyStringMap(round)
	canonicalizeImpersonation(o, r)

	var d ConfigMapDataDiff
	for k, v := range o {
		rv, ok := r[k]
		if !ok {
			d.Missing = append(d.Missing, k)
			continue
		}
		if !valuesEquivalent(k, v, rv) {
			d.Changed = append(d.Changed, k)
		}
	}
	for k := range r {
		if _, ok := o[k]; !ok {
			d.Extra = append(d.Extra, k)
		}
	}
	sort.Strings(d.Missing)
	sort.Strings(d.Extra)
	sort.Strings(d.Changed)
	return d
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func valuesEquivalent(key, orig, round string) bool {
	if orig == round {
		return true
	}
	for _, eq := range RoundTripValueEqualers {
		if equal, ok := eq(key, orig, round); ok && equal {
			return true
		}
	}
	return false
}

// canonicalizeImpersonation collapses enabled/enforced into a mode matching
// Argo CD defaults (enforced defaults to true when absent). When both sides
// share the same mode, the pair is removed so absent vs explicit enforced=true
// does not count as a diff.
func canonicalizeImpersonation(orig, round map[string]string) {
	om := impersonationModeFromData(orig)
	rm := impersonationModeFromData(round)
	if om == "" || om != rm {
		return
	}
	delete(orig, impersonationEnabledKey)
	delete(orig, impersonationEnforcedKey)
	delete(round, impersonationEnabledKey)
	delete(round, impersonationEnforcedKey)
}

// impersonationModeFromData mirrors Argo CD settings: enabled defaults false;
// when enabled, enforced defaults true unless explicitly "false".
func impersonationModeFromData(data map[string]string) string {
	en, hasEn := data[impersonationEnabledKey]
	if !hasEn {
		return ""
	}
	if !strings.EqualFold(en, "true") {
		return "disabled"
	}
	if ef, ok := data[impersonationEnforcedKey]; ok && strings.EqualFold(ef, "false") {
		return "optional"
	}
	return "required"
}

// durationValueEqual treats Go duration strings with the same length as equal
// (e.g. "180s" vs "3m0s"). Applies only when both values parse as durations.
func durationValueEqual(_, orig, round string) (bool, bool) {
	od, oerr := time.ParseDuration(strings.TrimSpace(orig))
	rd, rerr := time.ParseDuration(strings.TrimSpace(round))
	if oerr != nil || rerr != nil {
		return false, false
	}
	return od == rd, true
}

// yamlSemanticEqual treats YAML documents that decode to the same value as
// equal (key order, indentation, quoting, flow vs block). Applies when both
// sides unmarshal as YAML and at least one is structured (not a plain scalar
// string identical to its source).
func yamlSemanticEqual(_, orig, round string) (bool, bool) {
	var ov, rv any
	if err := yaml.Unmarshal([]byte(orig), &ov); err != nil {
		return false, false
	}
	if err := yaml.Unmarshal([]byte(round), &rv); err != nil {
		return false, false
	}
	if os, ok := ov.(string); ok && os == orig {
		if rs, ok := rv.(string); ok && rs == round {
			return false, false
		}
	}
	return reflect.DeepEqual(ov, rv), true
}
