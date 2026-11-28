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
// newVirtualPathResolverForFilesystem (the script base,
// standardVirtualAnchorsForOS anchors, temporary roots, the declared
// filesystem.paths, and the effective workdir with @work when
// virtualWorkdir.isReadRoot holds), resolved by the same function. Under
// `restricted` access every accepted access must resolve physically inside an
// allowed root; `full` access is an explicit opt-out.
//
// This model transcribes the requirements "Traversal out of an allowed root is
// blocked" and "The work directory is an allowed root unless a module declares
// it outside the module" (openspec/specs/virtual-runtime-sandbox) and
// "Restricted access limits virtual file I/O"
// (openspec/specs/virtual-filesystem-access) as its policy, and records four
// findings:
//   F14 (fixed): the pre-fix normalizeExistingOrParent evaluated only the path
//        and its parent, so a deep missing path below an escaping symlink was
//        judged lexically. The pre-fix code is kept as
//        mutantPreFixF14ParentOnlyEval.
//   F15 (fixed): a module-declared workdir outside the module widened the
//        allowed roots under restricted access, admitting reads that are not
//        cwd operations. The workdir is now a root only when it is trusted or
//        resolves physically inside the module (TrustedOrInside); the pre-fix
//        rule is kept as mutantPreFixF15WorkdirRoot, and
//        trustedWorkdirReadableFixed with its mutant
//        mutantStrictWorkdirNeverRoot keeps the fix from becoming the strict
//        variant, in which no workdir is a root.
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
// a missing volume, a platform-disallowed path component such as ':' outside a
// Windows volume prefix) has no request here, and the code denies it. For
// callers that perform the operation on validate's returned path (rather than
// using validate solely as a pre-check, such as checkGzipOutputPaths and the
// cp/tar -x derived-path checks in internal/uroot/registry_derived.go), a
// concurrent process replacing a directory on that path between the check and
// the open can still redirect the operation; the model has no time and does
// not cover that race.
//
// Correspondence (checked by `scripts/formal.py correspondence`):
//
// | model element | Go symbol | file | binding | abstraction |
// |---|---|---|---|---|
// | normalized | normalizeExistingOrParent | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | the deepest-existing-ancestor walk with the missing components re-joined, FixedEval; the pre-fix fallbacks are the other evalMode values; unresolvable paths (an error, so a denial) are not modelled |
// | normalized | walkVirtualPath | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the walk through every symbolic link and junction to the deepest existing ancestor; the model keeps only the ancestor's location, the link kind and the depth; link targets, "..", hop limits, unreadable links and the canonical form are covered by the fake-filesystem test, not modelled |
// | normalized | virtualPathWalker.walk | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the component-by-component walk to the deepest existing ancestor; the model keeps only where it ends; a platform-disallowed component (such as ':' outside the volume prefix on Windows) is a refused path, not modelled |
// | normalized | virtualPathWalker.next | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the next component of the walk's queue |
// | normalized | virtualPathWalker.enterVolume | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the volume root a rooted link target restarts from; a missing volume is a refused path, not modelled |
// | normalized | virtualPathWalker.follow | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | a link's target read and queued; the model keeps only the target's location and the link kind |
// | normalized | virtualPathWalker.unresolvedLink | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the outermost link's own path, read only by the full-access fallback |
// | normalized | virtualPathWalk.path | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the deepest existing ancestor joined with the missing components |
// | normalized | missingVirtualPathSuffix | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the missing components as plain names; a ".." or a platform-disallowed component after a missing one is a refused path, not modelled |
// | normalized | isVirtualPathLink | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_WalkOverFakeFilesystem | the link test: a symbolic link or a junction-like reparse point (LinkKind) |
// | normalized | normalizeVirtualPathForAccess | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_NoFollowOperands, TestVirtualPathFullAccessFallback | the access-mode split: restricted is the strict walk, full falls back to a link's own path or the cleaned path, and no containment decision reads the fallback; the fallback forms are not modelled and are bound by TestVirtualPathFullAccessFallback |
// | normalized | virtualPathResolver.resolveNoFollow | internal/runtime/virtual_path_nofollow.go | TestVirtualPathHarness_NoFollowOperands, TestVirtualPathNoFollowValidatorDirectoryCheck | the no-follow request: the parent is a model request resolved by the walk, the final name is joined unread (platform-disallowed final components are refused under restricted access, not modelled); one request kind in the model; a final separator, "." or ".." uses the follow form, and the not-a-directory check on it is not modelled and is bound by TestVirtualPathNoFollowValidatorDirectoryCheck |
// | pathWithin | pathWithin | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | filepath.Rel prefix test |
// | allowedRoots | newVirtualPathResolverForFilesystem | internal/runtime/virtual_policy.go | TestVirtualPathHarness_RootConstruction | script base/anchors/temp/declared as tagged roots, and the workdir with @work only when workdirIsRoot holds |
// | workdirIsRoot | virtualWorkdir.isReadRoot | internal/runtime/virtual_workdir.go | TestVirtualPathHarness_RootConstruction, TestVirtualPathHarness_GoldenVectors, TestVirtualPathHarness_Property | the fixed rule TrustedOrInside: a workdir with no module root is trusted; a module-declared one is a root when it resolves physically inside the module, both paths resolved by normalizeExistingOrParent, and a resolution error or another volume counts as outside; the model takes the workdir as an abstract kind (H.workdir), and the precedence source and the physical placement (relative, absolute, through a link or a junction) are generator dimensions of RootConstruction and the rapid property, not model facts; model gap: H.workdir = ModuleInside implies no InWorkdir, so no model decision reads the rule for a module-inside workdir and a change of workdirIsRoot there changes no vector; the Go replays carry that case instead: the golden replay and the rapid property assert isReadRoot() true for a module-inside workdir (harnessWorkdirValidator), so a mutant that returns false there fails them (calibration seed 4), while RootConstruction's module-inside kind has no workdir-only probe |
// | workdirIsRoot | virtualWorkdirFor | internal/runtime/virtual_workdir.go | TestVirtualPathHarness_RootConstruction, TestVirtualPathHarness_Property | the workdir kind of a command run, from the invowkfile, the command, the implementation and the CLI override: the module root is recorded only for a module invowkfile and a level the invowkfile declares; the interactive subprocess receives that module root through the hidden --workdir-module-root flag (interactiveWorkDirModuleRoot) and rebuilds the workdir from it (newVirtualPathResolverForInteractiveConfig), and a missing flag makes the child treat the workdir as trusted; that process hand-off is outside the model and is bound by TestInteractiveSpecCarriesWorkDirModuleRoot, TestRunShScriptWorkDirModuleRoot, TestRunLuaScriptWorkDirModuleRoot and TestInteractiveCommandsPassWorkdirModuleRootOnlyWhenSet |
// | workdirIsRoot | Invowkfile.ResolveEffectiveWorkDir | pkg/invowkfile/workdir_source.go | TestVirtualPathHarness_RootConstruction, TestVirtualPathHarness_Property | the effective workdir with the level that supplied it; the path is GetEffectiveWorkDir's |
// | workdirIsRoot | effectiveWorkDirSource | pkg/invowkfile/workdir_source.go | TestVirtualPathHarness_RootConstruction, TestVirtualPathHarness_Property, TestResolveEffectiveWorkDir | the level of the workdir precedence, with GetEffectiveWorkDir's tests in its order; a swap of the implementation and command tests leaves the source declared and the root decision unchanged, so only the drift test TestResolveEffectiveWorkDir sees it |
// | workdirIsRoot | WorkDirSource.DeclaredByInvowkfile | pkg/invowkfile/workdir_source.go | TestVirtualPathHarness_RootConstruction, TestVirtualPathHarness_Property | the implementation, command and root levels are declared by the invowkfile; the CLI override and the default are not; an unknown value counts as declared (fail closed), which effectiveWorkDirSource never returns and the model does not hold |
// | anchors | standardVirtualAnchorsForOS | internal/runtime/virtual_policy.go | TestVirtualPathHarness_RootConstruction | narrow claim (decision 14): anchors are explicit allowed roots (@config/@data/@cache/@state/@tmp InReadRoot, @work InReadRoot exactly when the workdir is a root; @home/.ssh denied under restricted, cross-platform-path-anchors); it catches only an empty anchor, a reported anchor missing from the allowed roots where no other root encloses it, an anchor covering @home/.ssh (decision 17), and @work admitted for a module-declared workdir outside the module; masked (decision 31): dropping @work is equivalent wherever the workdir itself is an allowed root, and dropping @tmp is masked off Windows while TMPDIR lies under /tmp (defaultTempRoots, the Linux reach lane); anchor paths are inputs from the production resolver for the host GOOS, not verified; follow-up: model per-OS anchors |
// | validate | virtualPathValidator.validate | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors | callers either run on the returned path or use validate as a pre-check only (checkGzipOutputPaths, cp/tar -x derived-path checks in internal/uroot/registry_derived.go); any race between validate and the open is not modelled (a residual risk, not closed) |
// | accepts | virtualPathValidator.validateNoFollow | internal/runtime/virtual_path_nofollow.go | TestVirtualPathHarness_NoFollowOperands, TestVirtualPathNoFollowValidatorDirectoryCheck | the check on the no-follow path; the operation runs on it and does not follow the final name; the check-then-act race is not modelled; the not-a-directory check is not modelled and is bound by TestVirtualPathNoFollowValidatorDirectoryCheck |
// | access | VirtualFilesystemConfig.EffectiveAccess | pkg/invowkfile/virtual_filesystem.go | TestVirtualPathHarness_RootConstruction | restricted vs full |
// | contained | - | formal/alloy/VirtualPathHarness.als | TestVirtualPathHarness_Property | policy: physical resolution; a trusted workdir is a read root, a module-declared workdir outside the module is not |
// | allowedTrustedWorkdir | - | formal/alloy/VirtualPathHarness.als | TestVirtualPathHarness_Property | policy: a trusted workdir (the CLI override, a workdir of an invowkfile outside a module, a module's default workdir) is an allowed root under restricted access; justification: the user chose the directory, or it is the module directory, so no module widens its own roots through it |
// | allowedWorkdirCwd | - | formal/alloy/VirtualPathHarness.als | - | policy: a module-declared workdir outside the module is a cwd, never an allowed virtual read root (and never a container mount root unless admitted by Layer 2 <cwd> or Layer 3 IncludeEntry.workspace_root) |
// | mutantPreFixF14ParentOnlyEval | normalizeExistingOrParent | internal/runtime/virtual_policy.go | TestVirtualPathHarness_DeepMissingPathThroughSymlinkDenied | regression: the fixed F14 deep-missing-path hole; the replay asserts the one-, two- and three-level paths through an escaping symlink are denied and nothing is created outside |
// | mutantPreFixF19DanglingLinkAsMissingName | normalizeExistingOrParent | internal/runtime/virtual_policy.go | TestVirtualPathHarness_DanglingLinkEscapeDenied | regression: the fixed F19 dangling-link hole; the replay asserts a link to a missing outside file (directly, and two missing levels deep) is denied with nothing created outside; its third leg, a link target with ".." after a missing component, is a path the resolver refuses and the model does not hold |
// | mutantPreFixF15WorkdirRoot | newVirtualPathResolverForFilesystem | internal/runtime/virtual_policy.go | TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot | regression: the fixed F15 workdir widening; the replay asserts that a module-declared workdir at the filesystem root (the volume root on Windows) and its @work are not allowed roots and a read below it is denied, and that the same directory as a CLI override is a root |
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

// The kind of the effective workdir (virtualWorkdirFor): trusted (the CLI
// override, a workdir of an invowkfile outside a module, or a module's default
// workdir), declared by a module's own invowkfile and physically inside the
// module, or declared by it and physically outside the module.
abstract sig WorkdirKind {}
one sig TrustedWorkdir, ModuleInside, ModuleOutside extends WorkdirKind {}

one sig H { access: one Access, workdir: one WorkdirKind }

// A module-inside workdir is enclosed by the script base, which is a read root,
// so no location lies inside the workdir only.
fact moduleInsideWorkdirIsEnclosed {
	H.workdir = ModuleInside implies no InWorkdir
}

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

// The code's rule for whether the workdir and @work enter the allowed roots
// (virtualWorkdir.isReadRoot in newVirtualPathResolverForFilesystem):
//   AlwaysRoot:      the pre-fix code, the workdir is always a root (F15);
//   TrustedOrInside: the fixed code, the workdir is a root when it is trusted
//                    or declared by the module inside the module;
//   NeverRoot:       the strict variant, the workdir is never a root (not
//                    chosen: it breaks trusted workdirs).
abstract sig WorkdirRule {}
one sig AlwaysRoot, TrustedOrInside, NeverRoot extends WorkdirRule {}

pred workdirIsRoot[rule: WorkdirRule] {
	rule = AlwaysRoot or (rule = TrustedOrInside and H.workdir in TrustedWorkdir + ModuleInside)
}

// isReadRoot: a location that counts as an allowed root for the pathWithin
// check: a read root, or the workdir when the rule makes it a root.
pred locIsAllowedRoot[l: Loc, rule: WorkdirRule] {
	l in InReadRoot or (workdirIsRoot[rule] and l in InWorkdir)
}

// accepts: the validator admits the access. Full access is an explicit opt-out;
// under restricted access the normalized location must be within an allowed root.
pred accepts[r: Req, mode: EvalMode, rule: WorkdirRule] {
	H.access = Full or locIsAllowedRoot[normalized[r, mode], rule]
}

// contained: the policy, written from the specs (virtual-runtime-sandbox "The
// work directory is an allowed root unless a module declares it outside the
// module"), not from the code's rule. Under restricted access every accepted
// access resolves physically inside a real allowed root, or inside a trusted
// workdir (allowedTrustedWorkdir); a module-declared workdir outside the module
// is a cwd, not a read root (allowedWorkdirCwd). Full access admits everything
// by design.
pred allowedTrustedWorkdir[r: Req] { H.workdir = TrustedWorkdir and r.physAnc in InWorkdir }
pred allowedWorkdirCwd[r: Req] { no r }   // no read is admitted merely by an outside module workdir
pred contained[r: Req] {
	H.access = Full or r.physAnc in InReadRoot or allowedTrustedWorkdir[r] or allowedWorkdirCwd[r]
}

// ---------------------------------------------------------------------------
// Safety predicates (parameterised so mutants and findings reuse them)

pred harnessContained[mode: EvalMode, rule: WorkdirRule] {
	all r: Req | accepts[r, mode, rule] implies contained[r]
}

// The narrow fix must not become the strict one: under restricted access every
// request inside a trusted workdir is accepted.
pred trustedWorkdirReadable[rule: WorkdirRule] {
	all r: Req | (H.access = Restricted and allowedTrustedWorkdir[r]) implies accepts[r, FixedEval, rule]
}

// ---------------------------------------------------------------------------
// Antecedents / witnesses

pred anteHarness { some r: Req | H.access = Restricted and accepts[r, FixedEval, TrustedOrInside] }
pred anteTrustedWorkdir { H.access = Restricted and some r: Req | allowedTrustedWorkdir[r] }

pred witEscapingAncestor { some r: Req | r.lexAnc != r.physAnc }
pred witDeepMissing { some r: Req | r.depth = TwoPlus }
pred witExisting { some r: Req | r.depth = Zero }
pred witFullAccess { H.access = Full }
pred witWorkdirRoot { some InWorkdir }
pred witModuleOutsideWorkdir { H.workdir = ModuleOutside and some InWorkdir }
pred witJunctionAncestor { some r: Req | r.link = Junction and r.lexAnc != r.physAnc }
pred witInLocationLink { some r: Req | some r.link and r.lexAnc = r.physAnc }
pred witDanglingLink { some r: Req | r.dangling = True and r.lexAnc != r.physAnc }

// F14 (fixed): a deep missing path below an escaping symlink, which the pre-fix
// code accepted lexically.
pred deepSymlinkEscape {
	H.access = Restricted
	some r: Req | r.depth = TwoPlus and r.lexAnc in InReadRoot and r.physAnc in OutsideAll
}

// F15 (fixed): a module-declared workdir outside the module, and a physical
// access inside it, which the pre-fix rule (AlwaysRoot) accepted.
pred workdirWidening {
	H.access = Restricted and H.workdir = ModuleOutside
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
	Decisions.dAccepts = { r: Req | accepts[r, FixedEval, TrustedOrInside] }
	Decisions.dContained = { r: Req | contained[r] }
}

// The golden fixes restricted access and leaves the workdir kind free, so the
// replay compares the real validator with the fixed evaluation and the fixed
// root rule for every kind; the model-level checks vary the rest.
pred goldenScenario {
	H.access = Restricted
}

pred noJunk {
	Loc = Req.lexAnc + Req.physAnc
	some Req
}
