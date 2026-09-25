// SPDX-License-Identifier: MPL-2.0
//
// Virtual-runtime path harness.
//
// The virtual runtimes (virtual-sh, virtual-lua) gate every filesystem access
// through virtualPathValidator.validate: it normalises the requested path with
// normalizeExistingOrParent (EvalSymlinks of the path, else of its parent, else
// a lexical fallback), then checks pathWithin against a list of allowed roots
// built by newVirtualPathResolverForFilesystem (the effective workdir, the
// script base, standardVirtualAnchorsForOS anchors, temporary roots, and the
// declared filesystem.paths). Under `restricted` access every accepted access
// must resolve physically inside an allowed root; `full` access is an explicit
// opt-out.
//
// This model transcribes the requirements "Traversal out of an allowed root is
// blocked" (openspec/specs/virtual-runtime-sandbox) and "Restricted access
// limits virtual file I/O" (openspec/specs/virtual-filesystem-access) as its
// policy, and records two findings:
//   F14: normalizeExistingOrParent evaluates only the path and its parent, so a
//        deep missing path below an escaping symlink is judged lexically.
//   F15: a module-declared workdir outside the module widens the allowed roots
//        under restricted access, admitting reads that are not cwd operations.
//
// Correspondence (checked by `scripts/formal.py correspondence`):
//
// | model element | Go symbol | file | binding | abstraction |
// |---|---|---|---|---|
// | normalize | normalizeExistingOrParent | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | path/parent/lexical fallback modelled by evalMode |
// | pathWithin | pathWithin | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | filepath.Rel prefix test |
// | allowedRoots | newVirtualPathResolverForFilesystem | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | workdir/script base/anchors/temp/declared as tagged roots |
// | anchors | standardVirtualAnchorsForOS | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | - |
// | validate | virtualPathValidator.validate | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | - |
// | access | VirtualFilesystemConfig.EffectiveAccess | pkg/invowkfile/virtual_filesystem.go | TestVirtualPathHarness_GoldenVectors | restricted vs full |
// | contained | - | formal/alloy/VirtualPathHarness.als | TestVirtualPathHarness_Property | policy: physical resolution, workdir excluded from read roots |
// | allowedWorkdirCwd | - | formal/alloy/VirtualPathHarness.als | - | policy: a workdir is a cwd, not an allowed read root |
// | findingF14 | normalizeExistingOrParent | internal/runtime/virtual_policy.go | TestVirtualPathHarness_F14DeepSymlink | characterisation: lexical fallback for a deep missing path |
// | findingF15 | newVirtualPathResolverForFilesystem | internal/runtime/virtual_policy.go | TestVirtualPathHarness_F15WorkdirWidening | characterisation: workdir added to the allowed roots |

module VirtualPathHarness

abstract sig Bool {}
one sig True, False extends Bool {}

abstract sig Access {}
one sig Restricted, Full extends Access {}

// The number of missing path components below the deepest existing ancestor of a
// requested path. normalizeExistingOrParent resolves the path when Zero, the
// parent when One, and falls back to a lexical (unresolved) path when TwoPlus.
abstract sig Depth {}
one sig Zero, One, TwoPlus extends Depth {}

// A location the harness can reach, and whether it lies inside a real allowed
// root (an anchor, temp root, script base, or declared path), inside a
// workdir-only root, or outside every root.
abstract sig Loc {}
sig InReadRoot extends Loc {}    // physically inside a non-workdir allowed root
sig InWorkdir extends Loc {}     // physically inside the declared workdir only
sig OutsideAll extends Loc {}    // physically outside every root

one sig H { access: one Access, includeWorkdir: one Bool }

// A request to the validator. lexAnc is the deepest existing ancestor's lexical
// location (the symlink's own place, unresolved); physAnc is where it physically
// resolves. When they differ the ancestor is an escaping symlink.
sig Req {
	depth: one Depth,
	lexAnc: one Loc,
	physAnc: one Loc
}

// The normalized location the validator compares, per the evaluation mode.
//   CurrentEval: path (Zero) or parent (One) evaluated physically; a deeper
//                missing path (TwoPlus) falls back to the lexical ancestor.
//   FixedEval:   the deepest existing ancestor is always evaluated physically.
//   NoParentEval: only the path itself is evaluated; a missing path (One or
//                 TwoPlus) falls back to lexical (mutant: parent eval dropped).
abstract sig EvalMode {}
one sig CurrentEval, FixedEval, NoParentEval extends EvalMode {}

fun normalized[r: Req, mode: EvalMode]: one Loc {
	mode = FixedEval => r.physAnc
	else (mode = CurrentEval => (r.depth = TwoPlus => r.lexAnc else r.physAnc)
	      else (r.depth = Zero => r.physAnc else r.lexAnc))
}

// isReadRoot: a location that counts as an allowed root for the pathWithin
// check. Under the current code the workdir is added to the roots
// (includeWorkdir = True); the fix keeps it out.
pred locIsAllowedRoot[l: Loc, includeWorkdir: Bool] {
	l in InReadRoot or (includeWorkdir = True and l in InWorkdir)
}

// accepts: the validator admits the access. Full access is an explicit opt-out;
// under restricted access the normalized location must be within an allowed root.
pred accepts[r: Req, mode: EvalMode, includeWorkdir: Bool] {
	H.access = Full or locIsAllowedRoot[normalized[r, mode], includeWorkdir]
}

// contained: the policy. Under restricted access every accepted access resolves
// physically inside a real allowed root; the workdir is a cwd, not a read root
// (allowedWorkdirCwd). Full access admits everything by design.
pred allowedWorkdirCwd[r: Req] { no r }   // no read is admitted merely by the workdir
pred contained[r: Req] {
	H.access = Full or r.physAnc in InReadRoot or allowedWorkdirCwd[r]
}

// ---------------------------------------------------------------------------
// Safety predicates (parameterised so mutants and findings reuse them)

pred harnessContained[mode: EvalMode, includeWorkdir: Bool] {
	all r: Req | accepts[r, mode, includeWorkdir] implies contained[r]
}

// ---------------------------------------------------------------------------
// Antecedents / witnesses

pred anteHarness { some r: Req | H.access = Restricted and accepts[r, FixedEval, False] }

pred witEscapingAncestor { some r: Req | r.lexAnc != r.physAnc }
pred witDeepMissing { some r: Req | r.depth = TwoPlus }
pred witExisting { some r: Req | r.depth = Zero }
pred witFullAccess { H.access = Full }
pred witWorkdirRoot { some InWorkdir }

// F14: a deep missing path below an escaping symlink, accepted lexically.
pred deepSymlinkEscape {
	H.access = Restricted
	some r: Req | r.depth = TwoPlus and r.lexAnc in InReadRoot and r.physAnc in OutsideAll
}

// F15: a workdir outside every read root, admitting a physical access inside it.
pred workdirWidening {
	H.access = Restricted and H.includeWorkdir = True
	some r: Req | r.depth = Zero and r.physAnc in InWorkdir and r.lexAnc in InWorkdir
}

// ---------------------------------------------------------------------------
// Golden-vector support. Commands live in formal/manifest.toml.

one sig Decisions {
	dNormalizedCurrent: Req -> lone Loc,
	dNormalizedFixed: Req -> lone Loc,
	dAcceptsCurrent: set Req,
	dAcceptsWorkdir: set Req,
	dContained: set Req
}

pred goldenDecisions {
	Decisions.dNormalizedCurrent = { r: Req, l: Loc | l = normalized[r, CurrentEval] }
	Decisions.dNormalizedFixed = { r: Req, l: Loc | l = normalized[r, FixedEval] }
	Decisions.dAcceptsCurrent = { r: Req | accepts[r, CurrentEval, H.includeWorkdir] }
	Decisions.dAcceptsWorkdir = { r: Req | accepts[r, FixedEval, H.includeWorkdir] }
	Decisions.dContained = { r: Req | contained[r] }
}

// The golden fixes the current-code configuration (workdir widening on) so the
// replay reproduces both findings; the model-level checks vary the flags.
pred goldenScenario {
	H.access = Restricted
	H.includeWorkdir = True
}

pred noJunk {
	Loc = Req.lexAnc + Req.physAnc
	some Req
}
