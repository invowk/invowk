// SPDX-License-Identifier: MPL-2.0

package fspath

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
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
	for _, failAt := range []string{"none", "create", "chmod", "write", "close", "rename"} {
		work := t.TempDir()
		target := filepath.Join(work, "invowkmod.lock.cue")
		if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		var trace []tlatrace.Record
		observe := func(ok, failed bool) {
			entries, _ := os.ReadDir(work)
			tmp := false
			for _, e := range entries {
				tmp = tmp || strings.HasSuffix(e.Name(), ".tmp")
			}
			content, _ := os.ReadFile(target)
			rec := atomicRecord(tmp, string(content) == "new", ok, failed)
			if len(trace) == 0 || !maps.Equal(trace[len(trace)-1], rec) {
				trace = append(trace, rec)
			}
		}
		observe(false, false)
		ops := atomicWriteOps{
			createTemp: func(d, pattern string) (atomicTempFile, error) {
				if failAt == "create" {
					return nil, errInjected
				}
				file, err := os.CreateTemp(d, pattern)
				if err != nil {
					return nil, err
				}
				observe(false, false)
				return &failingTempFile{File: file, failWrite: failAt == "write", failClose: failAt == "close"}, nil
			},
			chmod: func(name string, mode os.FileMode) error {
				if failAt == "chmod" {
					return errInjected
				}
				return os.Chmod(name, mode)
			},
			rename: func(from, to string) error {
				if failAt == "rename" {
					return errInjected
				}
				if err := os.Rename(from, to); err != nil {
					return fmt.Errorf("rename: %w", err)
				}
				observe(false, false)
				return nil
			},
			remove: os.Remove,
		}
		err := atomicWriteFile(target, []byte("new"), DefaultFilePerm, ops)
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
