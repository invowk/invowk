// SPDX-License-Identifier: MPL-2.0

package fspath

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/testutil/tlatrace"
)

func atomicRecord(tmp, replaced, ok, failed bool) tlatrace.Record {
	return tlatrace.Record{
		"tmp": tlatrace.Bool(tmp), "replaced": tlatrace.Bool(replaced),
		"ok": tlatrace.Bool(ok), "err": tlatrace.Bool(failed),
	}
}

// TestAtomicWrite_TraceHarness records atomicWriteFile on the real filesystem
// with a failure injected at each step (and none), for trace validation
// against AtomicWrite.tla, plus targeted mutations validation must reject.
func TestAtomicWrite_TraceHarness(t *testing.T) {
	t.Parallel()
	dir := tlatrace.Dir(t)

	var traces tlatrace.Traces
	for _, failAt := range atomicWriteSteps {
		work := t.TempDir()
		target := filepath.Join(work, "invowkmod.lock.cue")
		if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		var trace []tlatrace.Record
		observe := func(ok, failed bool) {
			content, _ := os.ReadFile(target)
			rec := atomicRecord(len(tempFilesIn(t, work)) > 0, string(content) == "new", ok, failed)
			if len(trace) == 0 || !maps.Equal(trace[len(trace)-1], rec) {
				trace = append(trace, rec)
			}
		}
		observe(false, false)
		err := atomicWriteFile(target, []byte("new"), DefaultFilePerm,
			injectingAtomicWriteOps(failAt, false, func() { observe(false, false) }))
		observe(err == nil, err != nil)
		traces.Accepted = append(traces.Accepted, trace)
	}
	traces.Rejected = [][]tlatrace.Record{
		// The target cannot be replaced before a temp file exists.
		{atomicRecord(false, false, false, false), atomicRecord(false, true, false, false), atomicRecord(false, true, true, false)},
		// Success never leaves the temp file visible.
		{atomicRecord(false, false, false, false), atomicRecord(true, false, false, false), atomicRecord(true, true, true, false)},
		// A failed write never replaces the target.
		{atomicRecord(false, false, false, false), atomicRecord(true, false, false, false), atomicRecord(false, true, false, true)},
	}
	tlatrace.Write(t, dir, "AtomicWriteTraces", traces)
}
