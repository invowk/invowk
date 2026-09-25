// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"fmt"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
)

// scopeCase is one command-scope scenario shared by the Alloy golden-vector
// replay and the rapid property test: discovered command sources, the calling
// module's requires, and its lock file. It mirrors ScopeConstruction.als.
type (
	scopeCase struct {
		cmds     []scopeCmd
		self     int
		requires []string
		lock     map[string]scopeLockEntry
	}

	scopeCmd struct {
		mod      string // empty for the cwd invowkfile
		src      string
		global   bool
		explicit bool // source declares an explicit command scope
		parent   int  // index of the module this source is vendored under, or -1
	}

	scopeLockEntry struct {
		mod string
		src string
	}
)

func scopeKeyURL(key string) invowkmod.GitURL {
	return invowkmod.GitURL(fmt.Sprintf("https://example.com/%s.git", key))
}

// realDecisions builds the real discovery and lock inputs and returns, for each
// command index, whether buildCommandScope + CanCallTarget allow the call.
func (c scopeCase) realDecisions(t *testing.T) []bool {
	t.Helper()

	available := make(map[invowkfile.CommandName]*discovery.CommandInfo, len(c.cmds))
	infos := make([]*discovery.CommandInfo, len(c.cmds))
	for i, cmd := range c.cmds {
		var moduleID *invowkmod.ModuleID
		if cmd.mod != "" {
			id := invowkmod.ModuleID(cmd.mod)
			moduleID = &id
		}
		name := invowkfile.CommandName(fmt.Sprintf("c%d lint", i))
		info := scopedCommandInfo(name, discovery.SourceID(cmd.src), moduleID)
		info.IsGlobalModule = cmd.global
		available[name] = info
		infos[i] = info
	}

	requirements := make([]invowkmod.ModuleRequirement, 0, len(c.requires))
	for _, key := range c.requires {
		requirements = append(requirements, invowkmod.ModuleRequirement{GitURL: scopeKeyURL(key), Version: "^1.0.0"})
	}
	lock := &invowkmod.LockFile{Modules: make(map[invowkmod.ModuleRefKey]invowkmod.LockedModule, len(c.lock))}
	for key, entry := range c.lock {
		lock.Modules[invowkmod.ModuleRefKey(scopeKeyURL(key))] = invowkmod.LockedModule{
			GitURL:          scopeKeyURL(key),
			ModuleID:        invowkmod.ModuleID(entry.mod),
			CommandSourceID: invowkmod.ModuleSourceID(entry.src),
		}
	}

	self := infos[c.self]
	meta := mustModuleMetadata(t, &invowkfile.Invowkmod{Module: *self.ModuleID, Version: "1.0.0", Requires: requirements})
	caller := &discovery.CommandInfo{
		Name:       self.Name,
		SourceID:   self.SourceID,
		ModuleID:   self.ModuleID,
		Invowkfile: &invowkfile.Invowkfile{Metadata: meta},
	}
	scope := buildCommandScope(caller, available, lock)
	if scope == nil {
		t.Fatalf("buildCommandScope() = nil for a module caller")
	}
	decisions := make([]bool, len(infos))
	for i, info := range infos {
		decisions[i] = commandScopeDecision(scope, info).Allowed
	}
	return decisions
}

// discoveryFactsViolation transcribes ScopeConstruction.als's discoveryFacts
// and returns the first violated fact, or "" when discovery could have produced
// this case. Golden replay checks it against Alloy; rapid checks its generator.
func (c scopeCase) discoveryFactsViolation() string {
	seen := make(map[string]int, len(c.cmds))
	for i, cmd := range c.cmds {
		switch {
		case cmd.src == "":
			return fmt.Sprintf("moduleSourcesHaveIDs: source %d has no SourceID", i)
		case (cmd.src == string(discovery.SourceIDInvowkfile)) != (cmd.mod == ""):
			return fmt.Sprintf("moduleSourcesHaveIDs: source %d pairs SourceID %q with ModuleID %q", i, cmd.src, cmd.mod)
		case cmd.global && cmd.mod == "":
			return fmt.Sprintf("globalChildren: global source %d has no ModuleID", i)
		}
		if j, dup := seen[cmd.src]; dup {
			return fmt.Sprintf("srcUnique: sources %d and %d share SourceID %q", j, i, cmd.src)
		}
		seen[cmd.src] = i
		for j := range i {
			other := c.cmds[j]
			if cmd.mod != "" && cmd.mod == other.mod && (!cmd.explicit || !other.explicit) {
				return fmt.Sprintf("modUnlessExplicit: sources %d and %d share ModuleID %q", j, i, cmd.mod)
			}
		}
		if cmd.parent >= 0 {
			if cmd.mod == "" {
				return fmt.Sprintf("globalChildren: vendored source %d has no ModuleID", i)
			}
			if c.cmds[cmd.parent].global && !cmd.global {
				return fmt.Sprintf("globalChildren: source %d does not inherit its parent's global flag", i)
			}
			for hops, at := 0, cmd.parent; at >= 0; hops, at = hops+1, c.cmds[at].parent {
				if at == i || hops > len(c.cmds) {
					return fmt.Sprintf("globalChildren: source %d is its own ancestor", i)
				}
			}
		}
	}
	if c.cmds[c.self].mod == "" {
		return "callerIsModule: the caller has no ModuleID"
	}
	return ""
}

// intendedAllowed transcribes ScopeConstruction.als's intendedAllowed: the
// policy written independently of the implementation's branches.
func (c scopeCase) intendedAllowed(target int) bool {
	t, self := c.cmds[target], c.cmds[c.self]
	if t.mod == self.mod && t.src == self.src {
		return true // ownModule
	}
	if t.global {
		return true
	}
	if t.mod == "" {
		return false
	}
	return slices.ContainsFunc(c.requires, func(key string) bool {
		entry, ok := c.lock[key]
		return ok && entry.mod == t.mod && entry.src == t.src
	})
}
