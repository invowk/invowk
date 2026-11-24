// SPDX-License-Identifier: MPL-2.0
//
// Virtual-runtime path harness.
//
// The virtual runtimes (virtual-sh, virtual-lua) gate every filesystem access
// through virtualPathValidator.validate: it normalises the requested path with
// normalizeExistingOrParent, which walks it through every symbolic link and
// Windows junction to the deepest existing ancestor (walkVirtualPath) and joins
// the missing components to that ancestor, with no lexical fallback. It then
// checks pathWithin against a list of allowed roots built by
// newVirtualPathResolverForFilesystem (the effective workdir, the script base,
// standardVirtualAnchorsForOS anchors, temporary roots, and the declared
// filesystem.paths), resolved by the same function. Under `restricted` access
// every accepted access must resolve physically inside an allowed root; `full`
// access is an explicit opt-out.
//
// This model transcribes the requirements "Traversal out of an allowed root is
// blocked" (openspec/specs/virtual-runtime-sandbox) and "Restricted access
// limits virtual file I/O" (openspec/specs/virtual-filesystem-access) as its
// policy, and records four findings:
//   F14 (fixed): the pre-fix normalizeExistingOrParent evaluated only the path
//        and its parent, so a deep missing path below an escaping symlink was
//        judged lexically. The pre-fix code is kept as
//        mutantPreFixF14ParentOnlyEval.
//   F15: a module-declared workdir outside the module widens the allowed roots
//        under restricted access, admitting reads that are not cwd operations.
//   F18 (fixed): filepath.EvalSymlinks does not follow a Windows junction, so
//        the pre-fix code judged every path through a junction lexically. The
//        escape was observed on windows-latest (CI run 44523520503, job
//        135557989975). walkVirtualPath follows junctions; the pre-fix code is
//        kept as mutantPreFixF18JunctionNotFollowed.
//   F19 (fixed): the pre-fix code could not evaluate a symlink whose target is
//        missing, fell back to the link's parent, and judged the link as a
//        missing plain name. TestVirtualPathHarness_DanglingLinkEscapeDenied
//        fails against the product code of c2fe96ff. The pre-fix code is kept
//        as mutantPreFixF19DanglingLinkAsMissingName.
//
// Abstractions. A request is a path the resolver can resolve: a path it
// refuses (a link cycle, an unreadable link, a ".." after a missing component,
// a missing volume) has no request here, and the code denies it. The check and
// the open are two steps: after validate returns, a concurrent process that
// replaces a directory on the resolved path with a link can still redirect the
// operation. The model has no time, so it does not cover that race.
//
// Correspondence (checked by `scripts/formal.py correspondence`):
//
// | model element | Go symbol | file | binding | abstraction |
// |---|---|---|---|---|
// | normalized | normalizeExistingOrParent | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | the deepest-existing-ancestor walk with the missing components re-joined, FixedEval; the pre-fix fallbacks are the other evalMode values; unresolvable paths (an error, so a denial) are not modelled |
// | normalized | walkVirtualPath | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the walk through every symbolic link and junction to the deepest existing ancestor; the model keeps only the ancestor's location, the link kind and the depth; link targets, "..", hop limits, unreadable links and the canonical form are covered by the fake-filesystem test, not modelled |
// | normalized | virtualPathWalker.walk | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the component-by-component walk to the deepest existing ancestor; the model keeps only where it ends |
// | normalized | virtualPathWalker.next | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the next component of the walk's queue |
// | normalized | virtualPathWalker.enterVolume | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the volume root a rooted link target restarts from; a missing volume is a refused path, not modelled |
// | normalized | virtualPathWalker.follow | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | a link's target read and queued; the model keeps only the target's location and the link kind |
// | normalized | virtualPathWalker.unresolvedLink | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the outermost link's own path, read only by the full-access fallback |
// | normalized | virtualPathWalk.path | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the deepest existing ancestor joined with the missing components |
// | normalized | missingVirtualPathSuffix | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the missing components as plain names; a ".." after a missing one is a refused path, not modelled |
// | normalized | isVirtualPathLink | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the link test: a symbolic link or a junction-like reparse point (LinkKind) |
// | normalized | normalizeVirtualPathForAccess | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_NoFollowOperands, TestVirtualPathFullAccessFallback | the access-mode split: restricted is the strict walk, full falls back to a link's own path or the cleaned path, and no containment decision reads the fallback; the fallback forms are not modelled and are bound by TestVirtualPathFullAccessFallback |
// | normalized | virtualPathResolver.resolveNoFollow | internal/runtime/virtual_path_nofollow.go | TestVirtualPathHarness_NoFollowOperands, TestVirtualPathNoFollowValidatorDirectoryCheck | the no-follow request: the parent is a model request resolved by the walk, the final name is joined unread; one request kind in the model; a final separator, "." or ".." uses the follow form, and the not-a-directory check on it is not modelled and is bound by TestVirtualPathNoFollowValidatorDirectoryCheck |
// | pathWithin | pathWithin | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | filepath.Rel prefix test |
// | allowedRoots | newVirtualPathResolverForFilesystem | internal/runtime/virtual_policy.go | TestVirtualPathHarness_RootConstruction | workdir/script base/anchors/temp/declared as tagged roots |
// | anchors | standardVirtualAnchorsForOS | internal/runtime/virtual_policy.go | TestVirtualPathHarness_RootConstruction | narrow claim (decision 14): anchors are explicit allowed roots (@config/@data/@cache/@state/@tmp/@work InReadRoot; @home/.ssh denied under restricted, cross-platform-path-anchors); it catches only an empty anchor, a reported anchor missing from the allowed roots where no other root encloses it, and an anchor covering @home/.ssh (decision 17); masked (decision 31): dropping @work is equivalent (the workdir is always a root), and dropping @tmp is masked off Windows while TMPDIR lies under /tmp (defaultTempRoots, the Linux reach lane); anchor paths are inputs from the production resolver for the host GOOS, not verified; follow-up: model per-OS anchors |
// | validate | virtualPathValidator.validate | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | the check only; the operation then runs on the returned path, and the race between this check and that open is not modelled (a residual risk, not closed) |
// | accepts | virtualPathValidator.validateNoFollow | internal/runtime/virtual_path_nofollow.go | TestVirtualPathHarness_NoFollowOperands, TestVirtualPathNoFollowValidatorDirectoryCheck | the check on the no-follow path; the operation runs on it and does not follow the final name; the check-then-act race is not modelled; the not-a-directory check is not modelled and is bound by TestVirtualPathNoFollowValidatorDirectoryCheck |
// | access | VirtualFilesystemConfig.EffectiveAccess | pkg/invowkfile/virtual_filesystem.go | TestVirtualPathHarness_RootConstruction | restricted vs full |
// | contained | - | formal/alloy/VirtualPathHarness.als | TestVirtualPathHarness_Property | policy: physical resolution, workdir excluded from read roots |
// | allowedWorkdirCwd | - | formal/alloy/VirtualPathHarness.als | - | policy: a workdir is a cwd, not an allowed read root |
// | mutantPreFixF14ParentOnlyEval | normalizeExistingOrParent | internal/runtime/virtual_policy.go | TestVirtualPathHarness_DeepMissingPathThroughSymlinkDenied | regression: the fixed F14 deep-missing-path hole; the replay asserts the one-, two- and three-level paths through an escaping symlink are denied and nothing is created outside |
// | mutantPreFixF19DanglingLinkAsMissingName | normalizeExistingOrParent | internal/runtime/virtual_policy.go | TestVirtualPathHarness_DanglingLinkEscapeDenied | regression: the fixed F19 dangling-link hole; the replay asserts a link to a missing outside file (directly, and two missing levels deep) is denied with nothing created outside; its third leg, a link target with ".." after a missing component, is a path the resolver refuses and the model does not hold |
// | findingF15 | newVirtualPathResolverForFilesystem | internal/runtime/virtual_policy.go | TestVirtualPathHarness_F15WorkdirWidening | characterisation: workdir added to the allowed roots |
// | mutantPreFixF18JunctionNotFollowed | walkVirtualPath | internal/runtime/virtual_policy.go | TestVirtualPathHarness_JunctionEscapeDenied | characterisation: only to keep this Windows-only replay out of the killers (it is skipped off Windows by the capability policy); it asserts the fixed F18 behaviour, not an open defect. A model request with link = Junction is materialised with fstree.MustJunction: an existing file, a new file, and a new file two missing components below a junction to an outside directory are denied and nothing is created outside, a path through a junction whose target is inside the root is accepted, and a root given through a junction accepts a request written through it at any depth |

module VirtualPathHarness

abstract sig Bool {}
one sig True, False extends Bool {}

abstract sig Access {}
one sig Restricted, Full extends Access {}

// The number of missing path components below the deepest existing ancestor of a
// requested path. normalizeExistingOrParent resolves that ancestor at every
// depth. The pre-fix code resolved the path when Zero, the parent when One, and
// fell back to a lexical (unresolved) path when TwoPlus.
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

// The kind of link a request crosses: a symbolic link, or a Windows junction
// (a mount-point reparse point, which os.Lstat reports with neither ModeSymlink
// nor ModeDir, and which filepath.EvalSymlinks does not follow).
abstract sig LinkKind {}
one sig Symlink, Junction extends LinkKind {}

// A request to the validator. lexAnc is the deepest existing ancestor's lexical
// location (the link's own place, unresolved); physAnc is where it physically
// resolves. When they differ the ancestor is reached through a link that leaves
// its location. link is the kind of link the request crosses, if any; a link
// may also stay in its location (lexAnc = physAnc). dangling says that the
// link's target does not exist: the deepest existing ancestor is then the
// target's directory, and the missing target is one of the missing components.
sig Req {
	depth: one Depth,
	lexAnc: one Loc,
	physAnc: one Loc,
	link: lone LinkKind,
	dangling: one Bool
}

// Only a link moves a path to another location.
fact linkExplainsRelocation {
	all r: Req | r.lexAnc != r.physAnc implies some r.link
}

// A dangling link is a symbolic link, and its missing target is at least one
// missing component. Dangling junctions are left out: nothing can be created
// through a junction whose target directory is missing.
fact danglingLinkShape {
	all r: Req | r.dangling = True implies (r.link = Symlink and r.depth != Zero)
}

// The normalized location the validator compares, per the evaluation mode.
//   FixedEval:     the deepest existing ancestor is always evaluated physically
//                  (the code).
// Each pre-fix mode differs from FixedEval in one defect of the old code:
//   PreFixF14Eval: a path with two or more missing components (TwoPlus) keeps
//                  its lexical ancestor (the parent-only evaluation, F14).
//   PreFixF18Eval: a junction is not followed, so a request that crosses one
//                  keeps its lexical ancestor at every depth (F18).
//   PreFixF19Eval: a link whose target is missing is read as a missing plain
//                  name, so a dangling request keeps its lexical ancestor (F19).
//   NoParentEval:  the walk is used only when nothing is missing; with one or
//                  more missing components the unresolved path is returned
//                  (the mutant that compares the unevaluated path; a superset
//                  of PreFixF14Eval).
abstract sig EvalMode {}
one sig FixedEval, NoParentEval, PreFixF14Eval, PreFixF18Eval, PreFixF19Eval extends EvalMode {}

fun normalized[r: Req, mode: EvalMode]: one Loc {
	mode = FixedEval => r.physAnc
	else (mode = PreFixF14Eval => (r.depth = TwoPlus => r.lexAnc else r.physAnc)
	      else (mode = PreFixF18Eval => (r.link = Junction => r.lexAnc else r.physAnc)
	            else (mode = PreFixF19Eval => (r.dangling = True => r.lexAnc else r.physAnc)
	                  else (r.depth = Zero => r.physAnc else r.lexAnc))))
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
pred witJunctionAncestor { some r: Req | r.link = Junction and r.lexAnc != r.physAnc }
pred witInLocationLink { some r: Req | some r.link and r.lexAnc = r.physAnc }
pred witDanglingLink { some r: Req | r.dangling = True and r.lexAnc != r.physAnc }

// F14 (fixed): a deep missing path below an escaping symlink, which the pre-fix
// code accepted lexically.
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
	dNormalized: Req -> lone Loc,
	dAccepts: set Req,
	dContained: set Req
}

pred goldenDecisions {
	Decisions.dNormalized = { r: Req, l: Loc | l = normalized[r, FixedEval] }
	Decisions.dAccepts = { r: Req | accepts[r, FixedEval, H.includeWorkdir] }
	Decisions.dContained = { r: Req | contained[r] }
}

// The golden fixes the current-code configuration: restricted access, with the
// workdir still an allowed root (F15 is open), so the replay compares the real
// validator with the fixed evaluation; the model-level checks vary the flags.
pred goldenScenario {
	H.access = Restricted
	H.includeWorkdir = True
}

pred noJunk {
	Loc = Req.lexAnc + Req.physAnc
	some Req
}
