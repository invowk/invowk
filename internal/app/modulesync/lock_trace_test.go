// SPDX-License-Identifier: MPL-2.0

package modulesync_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/tlatrace"
)

var (
	lockTraceOps = []string{"MoveTag", "TamperCache", "WipeCache", "TamperVendor", "Sync", "Vendor", "Discover", "CallViaSibling"}

	lockTraceDefaultInit = lockTraceInit{cache: true, vendored: true, sibling: lockLabelGood}
)

// lockTraceCase is one recorded trace: an initial state and the operations
// applied from it.
type lockTraceCase struct {
	start lockTraceInit
	ops   []string
}

func (st lockTraceInit) String() string {
	return fmt.Sprintf("%s cache=%v vendored=%v sibling=%s",
		map[bool]string{false: "v2", true: "v1"}[st.legacy], st.cache, st.vendored, st.sibling)
}

func (c lockTraceCase) name() string { return c.start.String() + ": " + strings.Join(c.ops, " ") }

// TestLockIntegrity_TraceHarness records traces of the real Resolver.Sync,
// vendoring (LoadDeclaredFromLock, then VendorModules), the discovery-time
// check of C's vendored copy, and command-scope admission through a sibling's
// copy, for trace validation against LockIntegrity.tla under
// LegacyLock = TRUE, plus targeted mutations that validation must reject.
func TestLockIntegrity_TraceHarness(t *testing.T) {
	t.Parallel()
	tlatrace.Dir(t) // skip before doing any work when trace output is off

	fx := newLockTraceFixture(t)
	cases := lockTraceCases()
	names := make([]string, 0, len(cases))
	byName := make(map[string]lockTraceCase, len(cases))
	for _, c := range cases {
		names = append(names, c.name())
		byName[c.name()] = c
	}
	accepted := tlatrace.RecordEach(t, names, func(t *testing.T, name string) []tlatrace.Record {
		t.Helper()
		c := byName[name]
		r := newLockTraceRun(t, fx, c.start)
		r.observe()
		for _, op := range c.ops {
			r.apply(op)
			r.observe()
		}
		return r.rec.Trace()
	})
	if accepted == nil {
		return
	}
	recorded := make(map[string][]tlatrace.Record, len(names))
	for i, name := range names {
		recorded[name] = accepted[i]
	}
	tlatrace.WriteSuite(t, "LockIntegrity", tlatrace.Traces{Accepted: accepted, Rejected: lockTraceMutations(t, recorded)})
}

// lockTraceCases returns every sequence of up to two operations from the
// default initial state, every single operation from each other initial
// state, and curated finding scenarios.
func lockTraceCases() []lockTraceCase {
	cases := []lockTraceCase{{start: lockTraceDefaultInit}}
	for _, first := range lockTraceOps {
		cases = append(cases, lockTraceCase{start: lockTraceDefaultInit, ops: []string{first}})
		for _, second := range lockTraceOps {
			cases = append(cases, lockTraceCase{start: lockTraceDefaultInit, ops: []string{first, second}})
		}
	}
	for _, legacy := range []bool{false, true} {
		for _, cache := range []bool{false, true} {
			for _, vendored := range []bool{false, true} {
				for _, sibling := range []string{lockLabelGood, lockLabelOther} {
					st := lockTraceInit{legacy: legacy, cache: cache, vendored: vendored, sibling: sibling}
					if st == lockTraceDefaultInit {
						continue
					}
					cases = append(cases, lockTraceCase{start: st})
					for _, op := range lockTraceOps {
						cases = append(cases, lockTraceCase{start: st, ops: []string{op}})
					}
				}
			}
		}
	}
	return append(cases, lockTraceCuratedCases()...)
}

func lockTraceCuratedCases() []lockTraceCase {
	v1Cache := lockTraceInit{legacy: true, cache: true, sibling: lockLabelGood}
	v2Cache := lockTraceInit{cache: true, sibling: lockLabelGood}
	v2Vendored := lockTraceInit{vendored: true, sibling: lockLabelGood}
	return []lockTraceCase{
		// F4: a hashless entry never vouches for a tampered cache; sync refetches.
		{v1Cache, strings.Fields("TamperCache Sync Vendor Discover")},
		// F4: a hashless entry never vouches for a tampered vendored copy.
		{lockTraceInit{legacy: true, cache: true, vendored: true, sibling: lockLabelGood}, strings.Fields("TamperVendor Discover")},
		// A moved tag with a hashless entry: the failed sync keeps the cache.
		{v1Cache, strings.Fields("MoveTag Sync Vendor WipeCache Sync")},
		// F5: a sync after a moved tag with an empty cache fails and keeps the lock.
		{v2Vendored, strings.Fields("MoveTag Sync Discover")},
		{v2Vendored, strings.Fields("MoveTag Sync TamperVendor Discover Sync")},
		// F6: the sibling's other version is rejected under C's hash; Good is admitted.
		{lockTraceInit{cache: true, sibling: lockLabelOther}, strings.Fields("CallViaSibling Sync Vendor Discover")},
		{v2Cache, strings.Fields("CallViaSibling Vendor Discover")},
		// F12 (open): a hashless entry admits the sibling's other version.
		{lockTraceInit{legacy: true, sibling: lockLabelOther}, strings.Fields("CallViaSibling Sync Vendor Discover")},
		// A tampered cache fails sync and vendor; the leftover copy is rejected.
		{v2Cache, strings.Fields("TamperCache Sync Vendor Discover")},
		{v2Cache, strings.Fields("TamperCache WipeCache Sync Vendor Discover")},
	}
}

// lockTraceMutations edits accepted traces one step at a time into
// behaviours LockIntegrity.tla forbids. A sibling admitted under a hashless
// entry is not among them: the base configuration accepts it (open finding F12).
func lockTraceMutations(t *testing.T, recorded map[string][]tlatrace.Record) [][]tlatrace.Record {
	t.Helper()
	trace := func(st lockTraceInit, ops string) []tlatrace.Record {
		c := lockTraceCase{start: st, ops: strings.Fields(ops)}
		tr, ok := recorded[c.name()]
		if !ok {
			t.Fatalf("mutation base %q was not recorded", c.name())
		}
		return tr
	}
	str := tlatrace.Str
	tamperDiscover := trace(lockTraceDefaultInit, "TamperVendor Discover")
	return [][]tlatrace.Record{
		// OwnLoadTrusted: a tampered vendored copy is loaded.
		tlatrace.Edit(tamperDiscover, -1, tlatrace.Record{"loaded": str(lockLabelEvil)}),
		// A tamper and a discovery land in one record.
		{tamperDiscover[0], tamperDiscover[len(tamperDiscover)-1]},
		// SiblingCallTrusted and F6: Other admitted under C's Good hash.
		tlatrace.Edit(trace(lockTraceInit{cache: true, sibling: lockLabelOther}, "CallViaSibling"), -1,
			tlatrace.Record{"called": str(lockLabelOther)}),
		// F5: a sync after a moved tag rewrites the lock to c2 and Evil.
		tlatrace.Edit(trace(lockTraceInit{vendored: true, sibling: lockLabelGood}, "MoveTag Sync Discover"), 2,
			tlatrace.Record{"lockCommit": str("c2"), "lockHash": str(lockLabelEvil), "cache": str(lockLabelEvil)}),
		// F4: a hashless lock whose Evil vendored copy is loaded.
		tlatrace.Edit(trace(lockTraceInit{legacy: true, cache: true, vendored: true, sibling: lockLabelGood}, "TamperVendor Discover"), -1,
			tlatrace.Record{"loaded": str(lockLabelEvil)}),
		// F4: a v1.0 lock vendors.
		tlatrace.Extend(trace(lockTraceInit{legacy: true, cache: true, sibling: lockLabelGood}, "Vendor"),
			tlatrace.Record{"vendored": str(lockLabelGood)}),
		// A failed sync rewrites the lock.
		tlatrace.Edit(trace(lockTraceDefaultInit, "TamperCache Sync"), -1, tlatrace.Record{"lockHash": str(lockLabelEvil)}),
	}
}
