// SPDX-License-Identifier: MPL-2.0

// Package tlatrace writes traces recorded from real code as a generated TLA+
// module, for trace validation by `scripts/formal.py traces`.
//
// A harness records one Record per observation: a projection of the model's
// variables, as TLA+ literals. Accepted traces come from running the real
// code; Rejected traces are targeted mutations that trace validation must
// refuse, proving the check is not vacuous.
package tlatrace

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// EnvDir names the directory harnesses write to. Harnesses skip when unset.
const EnvDir = "INVOWK_FORMAL_TRACE_DIR"

var errSuiteShape = errors.New("inconsistent trace suite")

type (
	// Record maps a projected variable name to its TLA+ literal.
	Record map[string]string

	// Traces are the traces one harness produced.
	Traces struct {
		Accepted [][]Record
		Rejected [][]Record
	}

	// Recorder builds one trace from observations of the real code.
	Recorder struct {
		trace []Record
	}

	// Recorded holds the traces RecordEach produced, keyed by name, so a
	// harness can build its targeted mutations from recorded traces.
	Recorded struct {
		names  []string
		byName map[string][]Record
	}

	suiteCounts struct {
		Accepted int      `json:"accepted"`
		Rejected int      `json:"rejected"`
		Fields   []string `json:"fields"`
	}
)

// Str renders a TLA+ string literal.
func Str(s string) string { return strconv.Quote(s) }

// Bool renders a TLA+ boolean.
func Bool(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

// Set renders a TLA+ set of string literals.
func Set(elems ...string) string {
	quoted := make([]string, 0, len(elems))
	for _, e := range elems {
		quoted = append(quoted, Str(e))
	}
	slices.Sort(quoted)
	return "{" + strings.Join(quoted, ", ") + "}"
}

// Dir returns the output directory, skipping the test when trace output is off.
func Dir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv(EnvDir)
	if dir == "" {
		t.Skipf("trace harness: set %s to record traces", EnvDir)
	}
	return dir
}

func renderRecord(r Record) string {
	keys := make([]string, 0, len(r))
	for k := range r {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	fields := make([]string, 0, len(keys))
	for _, k := range keys {
		fields = append(fields, fmt.Sprintf("%s |-> %s", k, r[k]))
	}
	return "[" + strings.Join(fields, ", ") + "]"
}

func renderTraces(traces [][]Record) string {
	rendered := make([]string, 0, len(traces))
	for _, trace := range traces {
		records := make([]string, 0, len(trace))
		for _, r := range trace {
			records = append(records, renderRecord(r))
		}
		rendered = append(rendered, "<<"+strings.Join(records, ", ")+">>")
	}
	return "<<" + strings.Join(rendered, ",\n    ") + ">>"
}

// Observe appends rec unless it equals the last record, so a trace holds one
// record per change of the observable projection.
func (r *Recorder) Observe(rec Record) {
	if n := len(r.trace); n > 0 && maps.Equal(r.trace[n-1], rec) {
		return
	}
	r.trace = append(r.trace, maps.Clone(rec))
}

// Trace returns the records observed so far.
func (r *Recorder) Trace() []Record { return slices.Clone(r.trace) }

// Edit returns a targeted mutation of trace: a copy of records 0..k with set
// applied to record k. A negative k counts from the end.
func Edit(trace []Record, k int, set Record) []Record {
	if k < 0 {
		k += len(trace)
	}
	mutated := make([]Record, 0, k+1)
	for _, rec := range trace[:k+1] {
		mutated = append(mutated, maps.Clone(rec))
	}
	maps.Copy(mutated[k], set)
	return mutated
}

// Extend returns a targeted mutation of trace: a copy with one more record,
// the last record with set applied.
func Extend(trace []Record, set Record) []Record {
	mutated := Edit(trace, -1, nil)
	next := maps.Clone(mutated[len(mutated)-1])
	maps.Copy(next, set)
	return append(mutated, next)
}

// RecordEach runs record for every name in a parallel subtest and returns the
// recorded traces; it returns nil when a subtest failed. The "traces" group
// itself is serial, so the caller writes the suite after every parallel trace
// finished.
func RecordEach(t *testing.T, names []string, record func(t *testing.T, name string) []Record) *Recorded {
	t.Helper()
	var mu sync.Mutex
	recorded := make(map[string][]Record, len(names))
	t.Run("traces", func(t *testing.T) {
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				trace := record(t, name)
				mu.Lock()
				recorded[name] = trace
				mu.Unlock()
			})
		}
	})
	if t.Failed() {
		return nil
	}
	return &Recorded{names: slices.Clone(names), byName: recorded}
}

// Accepted returns the recorded traces in the order RecordEach was given
// their names.
func (r *Recorded) Accepted() [][]Record {
	traces := make([][]Record, 0, len(r.names))
	for _, name := range r.names {
		traces = append(traces, r.byName[name])
	}
	return traces
}

// Base returns the trace recorded under name, failing the test when no
// trace was: a targeted mutation must start from a behaviour the real code
// produced.
func (r *Recorded) Base(t *testing.T, name string) []Record {
	t.Helper()
	trace, ok := r.byName[name]
	if !ok {
		t.Fatalf("mutation base %q was not recorded", name)
	}
	return trace
}

// unique drops duplicate traces, keeping the first occurrence of each in
// order. Different operation sequences often record the same trace, because
// operations the model disables leave the projection unchanged, and each
// trace costs one TLC run. It fails when a targeted mutation equals an
// accepted trace: the real code produced the behaviour the mutation claims
// validation must reject.
func (ts Traces) unique() (Traces, error) {
	var out Traces
	accepted := make(map[string]bool, len(ts.Accepted))
	for _, trace := range ts.Accepted {
		if key := renderTraces([][]Record{trace}); !accepted[key] {
			accepted[key] = true
			out.Accepted = append(out.Accepted, trace)
		}
	}
	rejected := make(map[string]bool, len(ts.Rejected))
	for i, trace := range ts.Rejected {
		key := renderTraces([][]Record{trace})
		if accepted[key] {
			return Traces{}, fmt.Errorf("%w: rejected trace %d equals an accepted trace", errSuiteShape, i+1)
		}
		if !rejected[key] {
			rejected[key] = true
			out.Rejected = append(out.Rejected, trace)
		}
	}
	return out, nil
}

// WriteSuite de-duplicates the traces and emits <model>Traces.tla defining
// Accepted and Rejected into the trace directory, and <model>Traces.json with their counts and the sorted
// projection fields, which the runner compares with the trace spec's Proj.
// It fails the test when either set is empty, a trace is empty, a targeted
// mutation equals an accepted trace, or records have different key sets: a
// misspelled key in a targeted mutation would otherwise make it vacuously
// rejected.
func WriteSuite(t *testing.T, model string, traces Traces) {
	t.Helper()
	if err := writeSuite(Dir(t), model, traces); err != nil {
		t.Fatalf("trace suite %s: %v", model, err)
	}
}

func writeSuite(dir, model string, traces Traces) error {
	traces, err := traces.unique()
	if err != nil {
		return err
	}
	fields, err := suiteFields(traces)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create trace dir: %w", err)
	}
	module := model + "Traces"
	var b strings.Builder
	fmt.Fprintf(&b, "---- MODULE %s ----\n\\* Generated by a trace harness; do not edit.\n", module)
	fmt.Fprintf(&b, "Accepted == %s\n\n", renderTraces(traces.Accepted))
	fmt.Fprintf(&b, "Rejected == %s\n====\n", renderTraces(traces.Rejected))
	if err = os.WriteFile(filepath.Join(dir, module+".tla"), []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write trace module: %w", err)
	}
	counts, err := json.Marshal(suiteCounts{Accepted: len(traces.Accepted), Rejected: len(traces.Rejected), Fields: fields})
	if err != nil {
		return fmt.Errorf("encode trace counts: %w", err)
	}
	if err = os.WriteFile(filepath.Join(dir, module+".json"), counts, 0o644); err != nil {
		return fmt.Errorf("write trace counts: %w", err)
	}
	return nil
}

// suiteFields returns the sorted key set shared by every record.
func suiteFields(traces Traces) ([]string, error) {
	var fields []string
	for _, suite := range []struct {
		set  string
		list [][]Record
	}{{"accepted", traces.Accepted}, {"rejected", traces.Rejected}} {
		set, list := suite.set, suite.list
		if len(list) == 0 {
			return nil, fmt.Errorf("%w: no %s traces", errSuiteShape, set)
		}
		for i, trace := range list {
			if len(trace) == 0 {
				return nil, fmt.Errorf("%w: %s trace %d is empty", errSuiteShape, set, i+1)
			}
			for j, rec := range trace {
				keys := slices.Sorted(maps.Keys(rec))
				if fields == nil {
					fields = keys
				} else if !slices.Equal(fields, keys) {
					return nil, fmt.Errorf("%w: %s trace %d record %d has fields %v, want %v", errSuiteShape, set, i+1, j+1, keys, fields)
				}
			}
		}
	}
	return fields, nil
}
