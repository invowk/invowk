// SPDX-License-Identifier: MPL-2.0

package tlatrace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func rec(state string, ok bool) Record { return Record{"state": Str(state), "ok": Bool(ok)} }

func TestRecorder_AppendsOnlyChanges(t *testing.T) {
	t.Parallel()
	var r Recorder
	observed := rec("a", false)
	r.Observe(observed)
	observed["state"] = Str("mutated") // the recorder keeps its own copy
	r.Observe(rec("a", false))
	r.Observe(rec("b", false))
	r.Observe(rec("b", false))
	r.Observe(rec("a", false))
	want := []Record{rec("a", false), rec("b", false), rec("a", false)}
	if got := r.Trace(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Trace() = %v, want %v", got, want)
	}
}

func TestWriteSuite_WritesModuleAndFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	traces := Traces{
		Accepted: [][]Record{{rec("a", false), rec("b", true)}},
		Rejected: [][]Record{{rec("a", false)}},
	}
	if err := writeSuite(dir, "Model", traces); err != nil {
		t.Fatalf("writeSuite() error = %v", err)
	}
	module, err := os.ReadFile(filepath.Join(dir, "ModelTraces.tla"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(module), "---- MODULE ModelTraces ----\n") ||
		!strings.Contains(string(module), `Accepted == <<<<[ok |-> FALSE, state |-> "a"], [ok |-> TRUE, state |-> "b"]>>>>`) {
		t.Fatalf("module = %s", module)
	}
	data, err := os.ReadFile(filepath.Join(dir, "ModelTraces.json"))
	if err != nil {
		t.Fatal(err)
	}
	var counts suiteCounts
	if err := json.Unmarshal(data, &counts); err != nil {
		t.Fatal(err)
	}
	if want := (suiteCounts{Accepted: 1, Rejected: 1, Fields: []string{"ok", "state"}}); !reflect.DeepEqual(counts, want) {
		t.Fatalf("counts = %+v, want %+v", counts, want)
	}
}

func TestWriteSuite_RejectsInconsistentSuites(t *testing.T) {
	t.Parallel()
	good := [][]Record{{rec("a", false)}}
	for name, tc := range map[string]struct {
		traces Traces
		want   string
	}{
		"no accepted": {Traces{Rejected: good}, "no accepted traces"},
		"no rejected": {Traces{Accepted: good}, "no rejected traces"},
		"empty trace": {Traces{Accepted: good, Rejected: [][]Record{{}}}, "rejected trace 1 is empty"},
		"misspelled key": {
			Traces{Accepted: good, Rejected: [][]Record{{rec("a", false), {"stat": Str("b"), "ok": Bool(false)}}}},
			"rejected trace 1 record 2 has fields [ok stat], want [ok state]",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			err := writeSuite(dir, "Model", tc.traces)
			if !errors.Is(err, errSuiteShape) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("writeSuite() error = %v, want %q", err, tc.want)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "ModelTraces.tla")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("an inconsistent suite must write nothing (stat: %v)", statErr)
			}
		})
	}
}
