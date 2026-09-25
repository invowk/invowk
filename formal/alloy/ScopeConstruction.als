// SPDX-License-Identifier: MPL-2.0
//
// Command-scope construction and query.
//
// Models how a module command's scope is BUILT (buildCommandScope from the
// discovered command sources, the caller's invowkmod.cue requires, and its lock
// file) and QUERIED (CommandScope.CanCallTarget), together with the discovery
// facts the result depends on. Modelling CanCallTarget alone would only restate
// its four-way "or"; a correct query over a wrongly built scope still leaks.
//
// Each `Cmd` atom is one discovered command SOURCE; all commands of a source
// share one scope decision, so one atom per source loses nothing.
//
// Correspondence (checked by `scripts/formal.py correspondence`):
//
// | model element | Go symbol | file | binding | abstraction |
// |---|---|---|---|---|
// | allowedImpl | buildCommandScope | internal/app/deps/deps.go | TestScopeConstructionGoldenVectors | caller is always a module command (root callers get no scope) |
// | allowedImpl | commandScopeDecision | internal/app/deps/deps.go | TestScopeConstructionGoldenVectors | - |
// | allowedImpl | CommandScope.CanCallTarget | pkg/invowkmod/command_scope.go | TestScopeConstructionGoldenVectors | target.Validate() assumed to pass: atoms map to valid IDs |
// | declaredLocked | IsDeclaredLockedCommandSource | pkg/invowkmod/vendored_policy.go | TestScopeConstructionGoldenVectors | lock identity uses explicit module_id and command_source_id |
// | srcUnique | CheckModuleCollisions | internal/discovery/discovery.go | - | discovery fact, not re-checked here |
// | modUnlessExplicit | CheckModuleCollisions | internal/discovery/discovery.go | - | discovery fact, not re-checked here |
// | moduleSourcesHaveIDs | moduleIdentityFor | internal/discovery/discovery.go | - | discovery fact: module-backed sources always get a SourceID |
// | globalChildren | IsGlobalModule | internal/discovery/discovery_files.go | - | vendored children of global modules inherit the global flag |
// | RootSrc | SourceID | internal/discovery/discovery_commands.go | - | the cwd invowkfile publishes SourceID "invowkfile" without a ModuleID |

module ScopeConstruction

sig Mod {}
sig Src {}
one sig RootSrc extends Src {}
sig Key {}
sig Entry {
	eid: one Mod,
	esrc: one Src
}

sig Cmd {
	mod: lone Mod,
	src: lone Src,
	childOf: lone Cmd
}
sig GlobalCmd in Cmd {}
sig Explicit in Cmd {}

one sig Caller {
	self: one Cmd,
	requires: set Key,
	lock: Key -> lone Entry,
	allowedSet: set Cmd
}

// ---------------------------------------------------------------------------
// Discovery facts (what the code producing the inputs guarantees)

// CheckModuleCollisions: command source IDs are unique across all sources,
// including the cwd invowkfile's "invowkfile".
pred srcUnique { all disj a, b: Cmd | some a.src implies a.src != b.src }

// CheckModuleCollisions: a ModuleID may repeat only when both sources declare
// an explicit command scope.
pred modUnlessExplicit { all disj a, b: Cmd | (some a.mod and a.mod = b.mod) implies (a + b) in Explicit }

// moduleIdentityFor always attaches a SourceID to module-backed sources; only
// the cwd invowkfile carries RootSrc, and it has no ModuleID.
pred moduleSourcesHaveIDs {
	all c: Cmd | some c.src
	all c: Cmd | c.src = RootSrc iff no c.mod
}

// Global modules are module-backed, and their vendored children inherit the flag.
pred globalChildren {
	GlobalCmd in mod.Mod
	all c: Cmd | (some c.childOf and c.childOf in GlobalCmd) implies c in GlobalCmd
	no c: Cmd | c in c.^childOf
	all c: Cmd | some c.childOf implies some c.mod
}

pred callerIsModule { some Caller.self.mod }

pred discoveryFacts { srcUnique and modUnlessExplicit and moduleSourcesHaveIDs and globalChildren and callerIsModule }

// Mutated discovery facts, each dropping one guarantee.
pred discoveryFactsNoSrcUnique { modUnlessExplicit and moduleSourcesHaveIDs and globalChildren and callerIsModule }
pred discoveryFactsRootNotReserved {
	srcUnique and modUnlessExplicit and globalChildren and callerIsModule
	all c: Cmd | some c.src
}
pred discoveryFactsEmptySrc {
	srcUnique and modUnlessExplicit and globalChildren and callerIsModule
	all c: Cmd | c.src = RootSrc implies no c.mod
}

// ---------------------------------------------------------------------------
// Implementation (buildCommandScope + CanCallTarget, transcribed)

fun globals: set Src { GlobalCmd.src }

pred declaredLocked[m: Mod, s: Src] {
	some k: Caller.requires | some Caller.lock[k] and Caller.lock[k].eid = m and Caller.lock[k].esrc = s
}

pred allowedImpl[t: Cmd] {
	// no source identity and no module identity: local/root branch
	(no t.src and no t.mod)
	// same module: ModuleID matches; SourceID must match when the target has one
	or (t.mod = Caller.self.mod and (no t.src or t.src = Caller.self.src))
	// global: by SourceID only
	or (some t.src and t.src in globals)
	// direct dependency: (ModuleID, SourceID) declared and locked by the caller
	or (some t.mod and some t.src and declaredLocked[t.mod, t.src])
}

// ---------------------------------------------------------------------------
// Intent (written independently of the implementation's branches)

pred ownModule[t: Cmd] { t.mod = Caller.self.mod and t.src = Caller.self.src }

pred intendedAllowed[t: Cmd] {
	ownModule[t] or t in GlobalCmd or (some t.mod and declaredLocked[t.mod, t.src])
}

pred boundedProp { all t: Cmd | allowedImpl[t] implies intendedAllowed[t] }
pred completeProp { all t: Cmd | intendedAllowed[t] implies allowedImpl[t] }
pred rootDeniedProp { all t: Cmd | t.src = RootSrc implies not allowedImpl[t] }
pred localBranchDeadProp { no t: Cmd | no t.src and no t.mod }

// ---------------------------------------------------------------------------
// Golden-vector support. The model declares no commands: every run and check
// lives in formal/manifest.toml, and scripts/formal.py renders them into a
// staged copy under artifacts/formal/ScopeConstruction/.

// Atoms that cannot influence a decision only multiply instances; the golden
// enumeration excludes them. Every excluded atom is unreferenced by any Cmd,
// requires entry, or lock entry, so no decision is lost.
pred noJunk {
	Entry in Key.(Caller.lock)
	Key in Caller.requires + (Caller.lock).Entry
	Mod in Cmd.mod + Entry.eid
	Src in Cmd.src + Entry.esrc + RootSrc
}
