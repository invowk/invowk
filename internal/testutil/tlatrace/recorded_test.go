// SPDX-License-Identifier: MPL-2.0

// External test package: tparallel follows RecordEach's t.Run into its own
// package and cannot see that the subtests it starts are parallel.
package tlatrace_test

import (
	"reflect"
	"testing"

	"github.com/invowk/invowk/internal/testutil/tlatrace"
)

func TestRecordEach_KeepsOrderAndLooksUpBases(t *testing.T) {
	t.Parallel()
	recorded := tlatrace.RecordEach(t, []string{"b", "a", "c"}, recordName)
	if recorded == nil {
		t.Fatal("RecordEach returned nil although every subtest passed")
	}
	want := [][]tlatrace.Record{nameTrace("b"), nameTrace("a"), nameTrace("c")}
	if got := recorded.Accepted(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Accepted() = %v, want %v", got, want)
	}
	if got := recorded.Base(t, "a"); !reflect.DeepEqual(got, nameTrace("a")) {
		t.Fatalf("Base(a) = %v, want %v", got, nameTrace("a"))
	}
}

func nameTrace(name string) []tlatrace.Record {
	return []tlatrace.Record{{"state": tlatrace.Str(name)}}
}

func recordName(_ *testing.T, name string) []tlatrace.Record { return nameTrace(name) }
