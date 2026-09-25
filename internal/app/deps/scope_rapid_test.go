// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"

	"github.com/invowk/invowk/internal/discovery"
)

// drawScopeCase generates command-scope scenarios restricted to what discovery
// produces (ScopeConstruction.als discoveryFacts): unique SourceIDs, only the
// cwd invowkfile without a ModuleID, global modules that are module-backed, and
// vendored children that inherit the global flag. ModuleIDs may repeat, which
// discovery allows for sources with an explicit command scope.
func drawScopeCase(t *rapid.T) scopeCase {
	modules := []string{"io.example.m0", "io.example.m1", "io.example.m2"}
	keys := []string{"k0", "k1", "k2"}

	n := rapid.IntRange(1, 7).Draw(t, "sources")
	c := scopeCase{lock: make(map[string]scopeLockEntry)}
	hasRoot := n > 1 && rapid.Bool().Draw(t, "hasRoot")
	for i := range n {
		if hasRoot && i == n-1 {
			c.cmds = append(c.cmds, scopeCmd{src: string(discovery.SourceIDInvowkfile), parent: -1})
			continue
		}
		cmd := scopeCmd{
			mod:    rapid.SampledFrom(modules).Draw(t, fmt.Sprintf("mod%d", i)),
			src:    fmt.Sprintf("src%d", i),
			global: rapid.Bool().Draw(t, fmt.Sprintf("global%d", i)),
			parent: -1,
		}
		if i > 0 {
			// Parents precede children, so the vendoring relation stays acyclic.
			if parent := rapid.IntRange(-1, i-1).Draw(t, fmt.Sprintf("parent%d", i)); parent >= 0 && c.cmds[parent].mod != "" {
				cmd.parent = parent
				cmd.global = cmd.global || c.cmds[parent].global // vendored children inherit the flag
			}
		}
		c.cmds = append(c.cmds, cmd)
	}
	// A ModuleID may repeat only between sources with an explicit command scope.
	for i := range c.cmds {
		for j := range i {
			if c.cmds[i].mod != "" && c.cmds[i].mod == c.cmds[j].mod {
				c.cmds[i].explicit, c.cmds[j].explicit = true, true
			}
		}
	}
	moduleSources := make([]int, 0, len(c.cmds))
	for i, cmd := range c.cmds {
		if cmd.mod != "" {
			moduleSources = append(moduleSources, i)
		}
	}
	c.self = rapid.SampledFrom(moduleSources).Draw(t, "caller")

	srcs := make([]string, 0, len(c.cmds)+1)
	for _, cmd := range c.cmds {
		srcs = append(srcs, cmd.src)
	}
	srcs = append(srcs, "unrelated")
	for _, key := range keys {
		if rapid.Bool().Draw(t, "requires-"+key) {
			c.requires = append(c.requires, key)
		}
		if rapid.Bool().Draw(t, "locked-"+key) {
			c.lock[key] = scopeLockEntry{
				mod: rapid.SampledFrom(modules).Draw(t, "lockmod-"+key),
				src: rapid.SampledFrom(srcs).Draw(t, "locksrc-"+key),
			}
		}
	}
	return c
}

// TestScopeConstruction_MatchesIntent checks the real buildCommandScope and
// CanCallTarget against ScopeConstruction.als's independently written intent
// (intendedAllowed) at scopes larger than the exhaustive golden vectors.
func TestScopeConstruction_MatchesIntent(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		c := drawScopeCase(rt)
		if violation := c.discoveryFactsViolation(); violation != "" {
			rt.Fatalf("generator produced a case discovery cannot: %s\n%+v", violation, c)
		}
		actual := c.realDecisions(t)
		for target := range c.cmds {
			if want := c.intendedAllowed(target); actual[target] != want {
				rt.Fatalf("allowedTargetsBounded/allowedTargetsComplete: target %d real allowed=%v, intended=%v\n%+v",
					target, actual[target], want, c)
			}
		}
	})
}
