// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"pgregory.net/rapid"

	"github.com/invowk/invowk/internal/testutil/alloygolden"
)

// The DependencyClosure golden vectors live next to the pkg/invowkmod replay;
// tidy reuses them to check its fixed point against the model's tidyResult.
const dependencyClosureGoldenFile = "../../../pkg/invowkmod/testdata/formal/dependency_closure_golden.json.gz"

// graphResolver returns a resolveAllFunc over a fixed requirement graph and
// counts how many times tidy calls it.
func graphResolver(graph map[ModuleRefKey][]ModuleRef, calls *int) resolveAllFunc {
	return func(_ context.Context, requirements []ModuleRef, _ map[ModuleRefKey]LockedModule) ([]*ResolvedModule, error) {
		*calls++
		resolved := make([]*ResolvedModule, 0, len(requirements))
		for _, req := range requirements {
			resolved = append(resolved, &ResolvedModule{ModuleRef: req, ModuleID: ModuleID(req.Key()), TransitiveDeps: graph[req.Key()]})
		}
		return resolved, nil
	}
}

func refKeys(refs []ModuleRef) []ModuleRefKey {
	keys := make([]ModuleRefKey, 0, len(refs))
	for _, ref := range refs {
		keys = append(keys, ref.Key())
	}
	slices.Sort(keys)
	return keys
}

// closureLayers returns the transitive closure of roots minus the roots, and
// the number of BFS layers needed to reach it.
func closureLayers(roots []ModuleRef, graph map[ModuleRefKey][]ModuleRef) (added []ModuleRefKey, layers int) {
	seen := make(map[ModuleRefKey]bool)
	frontier := make([]ModuleRef, 0, len(roots))
	for _, root := range roots {
		if !seen[root.Key()] {
			seen[root.Key()] = true
			frontier = append(frontier, root)
		}
	}
	for len(frontier) > 0 {
		var next []ModuleRef
		for _, ref := range frontier {
			for _, dep := range graph[ref.Key()] {
				if !seen[dep.Key()] {
					seen[dep.Key()] = true
					next = append(next, dep)
					added = append(added, dep.Key())
				}
			}
		}
		if len(next) > 0 {
			layers++
		}
		frontier = next
	}
	slices.Sort(added)
	return added, layers
}

// TestTidyToFixedPoint_GoldenVectors replays every DependencyClosure instance:
// the keys tidy adds must equal the model's tidyResult (closure minus roots).
func TestTidyToFixedPoint_GoldenVectors(t *testing.T) {
	t.Parallel()

	instances := alloygolden.Load(t, dependencyClosureGoldenFile, "DependencyClosure")
	ref := func(atom string) ModuleRef {
		return ModuleRef{GitURL: GitURL(alloygolden.GitURL(atom)), Version: "^1.0.0"}
	}
	for i, inst := range instances {
		root := inst.Atoms("Root")[0]
		graph := make(map[ModuleRefKey][]ModuleRef)
		for _, modAtom := range inst.Atoms("Mod") {
			key := ref(inst.Target("key", modAtom)).Key()
			for _, reqAtom := range inst.Targets("requires", modAtom) {
				graph[key] = append(graph[key], ref(reqAtom))
			}
		}
		var roots []ModuleRef
		for _, keyAtom := range inst.Targets("roots", root) {
			roots = append(roots, ref(keyAtom))
		}
		var want []ModuleRefKey
		for _, keyAtom := range inst.Targets("tidySet", root) {
			want = append(want, ref(keyAtom).Key())
		}
		slices.Sort(want)

		calls := 0
		missing, err := tidyToFixedPoint(t.Context(), roots, nil, graphResolver(graph, &calls))
		if err != nil {
			t.Fatalf("instance %d: tidyToFixedPoint() error = %v", i, err)
		}
		if got := refKeys(missing); !slices.Equal(got, want) {
			t.Fatalf("instance %d: tidy added %v, model tidyResult %v", i, got, want)
		}
	}
}

// TestTidyToFixedPoint_MatchesClosure checks tidy on larger generated graphs,
// including cycles and diamonds: it adds exactly the undeclared part of the
// closure, never a declared key, and calls resolveAll once per BFS layer plus
// one confirming round.
func TestTidyToFixedPoint_MatchesClosure(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 8).Draw(rt, "modules")
		refs := make([]ModuleRef, n)
		for i := range refs {
			refs[i] = ModuleRef{GitURL: GitURL(fmt.Sprintf("https://example.com/m%d.git", i)), Version: "^1.0.0"}
		}
		graph := make(map[ModuleRefKey][]ModuleRef)
		for _, from := range refs {
			for j, to := range refs {
				if rapid.Bool().Draw(rt, fmt.Sprintf("%s->%d", from.Key(), j)) {
					graph[from.Key()] = append(graph[from.Key()], to)
				}
			}
		}
		var roots []ModuleRef
		for i, r := range refs {
			if i == 0 || rapid.Bool().Draw(rt, fmt.Sprintf("root%d", i)) {
				roots = append(roots, r)
			}
		}

		wantAdded, layers := closureLayers(roots, graph)
		calls := 0
		missing, err := tidyToFixedPoint(context.Background(), roots, nil, graphResolver(graph, &calls)) //nolint:usetesting // rapid.T has no Context
		if err != nil {
			rt.Fatalf("tidyToFixedPoint() error = %v", err)
		}
		added, rootKeys := refKeys(missing), refKeys(roots)
		if !slices.Equal(added, wantAdded) {
			rt.Fatalf("tidyResult: tidy added %v, closure minus roots %v", added, wantAdded)
		}
		for _, key := range added {
			if slices.Contains(rootKeys, key) {
				rt.Fatalf("tidyAddsNoDeclaredKey: tidy added declared key %s", key)
			}
		}
		if calls != layers+1 {
			rt.Fatalf("tidy called resolveAll %d times, want %d (layers %d + confirming round)", calls, layers+1, layers)
		}
	})
}
