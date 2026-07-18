package mapping

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Severity classifies a Diagnostic.
type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

// Direction indicates which conversion direction produced a Diagnostic.
type Direction string

const (
	DirCMToCR Direction = "cm->cr"
	DirCRToCM Direction = "cr->cm"
)

// Diagnostic is a single conversion finding a user may need to act on.
type Diagnostic struct {
	Severity  Severity  `json:"severity"`
	Key       string    `json:"key,omitempty"`
	Direction Direction `json:"direction"`
	Message   string    `json:"message"`
}

// Diagnostics collects conversion findings.
type Diagnostics struct {
	items []Diagnostic
}

// Add appends a diagnostic.
func (d *Diagnostics) Add(sev Severity, dir Direction, key, msg string) {
	if d == nil {
		return
	}
	d.items = append(d.items, Diagnostic{
		Severity:  sev,
		Key:       key,
		Direction: dir,
		Message:   msg,
	})
}

// Warn is a convenience for SeverityWarn.
func (d *Diagnostics) Warn(dir Direction, key, msg string) {
	d.Add(SeverityWarn, dir, key, msg)
}

// Error is a convenience for SeverityError.
func (d *Diagnostics) Error(dir Direction, key, msg string) {
	d.Add(SeverityError, dir, key, msg)
}

// Info is a convenience for SeverityInfo.
func (d *Diagnostics) Info(dir Direction, key, msg string) {
	d.Add(SeverityInfo, dir, key, msg)
}

// Items returns a sorted copy of collected diagnostics (severity, key, message, direction).
func (d *Diagnostics) Items() []Diagnostic {
	if d == nil {
		return nil
	}
	out := make([]Diagnostic, len(d.items))
	copy(out, d.items)
	sortDiagnostics(out)
	return out
}

func sortDiagnostics(items []Diagnostic) {
	sevRank := map[Severity]int{
		SeverityError: 0,
		SeverityWarn:  1,
		SeverityInfo:  2,
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		ra, rb := sevRank[a.Severity], sevRank[b.Severity]
		if ra != rb {
			return ra < rb
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		if a.Message != b.Message {
			return a.Message < b.Message
		}
		return a.Direction < b.Direction
	})
}

// HasErrors reports whether any error-severity diagnostics were recorded.
func (d *Diagnostics) HasErrors() bool {
	if d == nil {
		return false
	}
	for _, it := range d.items {
		if it.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings reports whether any warn-severity diagnostics were recorded.
func (d *Diagnostics) HasWarnings() bool {
	if d == nil {
		return false
	}
	for _, it := range d.items {
		if it.Severity == SeverityWarn {
			return true
		}
	}
	return false
}

// Len returns the number of diagnostics.
func (d *Diagnostics) Len() int {
	if d == nil {
		return 0
	}
	return len(d.items)
}

// Merge appends diagnostics from other into d.
func (d *Diagnostics) Merge(other *Diagnostics) {
	if d == nil || other == nil {
		return
	}
	d.items = append(d.items, other.items...)
}

// WriteHuman writes a human-readable grouped report to w.
// Diagnostics within each severity group are sorted for stable output.
func (d *Diagnostics) WriteHuman(w io.Writer) error {
	items := d.Items()
	if len(items) == 0 {
		return nil
	}
	bySev := map[Severity][]Diagnostic{
		SeverityError: {},
		SeverityWarn:  {},
		SeverityInfo:  {},
	}
	for _, it := range items {
		bySev[it.Severity] = append(bySev[it.Severity], it)
	}
	order := []Severity{SeverityError, SeverityWarn, SeverityInfo}
	for _, sev := range order {
		group := bySev[sev]
		if len(group) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s (%d):\n", strings.ToUpper(string(sev)), len(group)); err != nil {
			return err
		}
		for _, it := range group {
			key := it.Key
			if key == "" {
				key = "(none)"
			}
			if _, err := fmt.Fprintf(w, "  [%s] %s: %s\n", it.Direction, key, it.Message); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteJSON writes the diagnostics as a sorted JSON array to w.
func (d *Diagnostics) WriteJSON(w io.Writer) error {
	items := d.Items()
	if items == nil {
		items = []Diagnostic{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

// keyTracker tracks which ConfigMap keys were consumed during mapping.
type keyTracker struct {
	source  map[string]string
	used    map[string]struct{}
	diag    *Diagnostics
	dir     Direction
	cmLabel string
}

func newKeyTracker(data map[string]string, diag *Diagnostics, dir Direction, cmLabel string) *keyTracker {
	src := data
	if src == nil {
		src = map[string]string{}
	}
	return &keyTracker{
		source:  src,
		used:    map[string]struct{}{},
		diag:    diag,
		dir:     dir,
		cmLabel: cmLabel,
	}
}

// use marks a key as consumed.
func (k *keyTracker) use(key string) {
	if k == nil {
		return
	}
	k.used[key] = struct{}{}
}

// get returns the value and whether the key exists, marking it consumed when present.
func (k *keyTracker) get(key string) (string, bool) {
	if k == nil {
		return "", false
	}
	v, ok := k.source[key]
	if ok {
		k.used[key] = struct{}{}
	}
	return v, ok
}

// reportUnknown emits a warn diagnostic for each unused source key (sorted).
func (k *keyTracker) reportUnknown() {
	if k == nil || k.diag == nil {
		return
	}
	var unknown []string
	for key := range k.source {
		if _, ok := k.used[key]; ok {
			continue
		}
		unknown = append(unknown, key)
	}
	sort.Strings(unknown)
	for _, key := range unknown {
		k.diag.Warn(k.dir, key, fmt.Sprintf("unhandled key in %s; value will be dropped on conversion", k.cmLabel))
	}
}
