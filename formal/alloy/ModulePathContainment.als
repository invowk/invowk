// SPDX-License-Identifier: MPL-2.0
//
// Module filesystem path containment.
//
// Module path containment means every path a module makes Invowk read,
// execute, copy, or hash resolves physically inside that module's root, unless a
// named allowed-location rule admits it. No single Go function enforces this: it
// emerges from a lexical containment check, two discovery facts (IsModule
// rejects a symlinked root; Load scans a non-symlinked root and refuses any
// inner symlink, except under invowk_modules/), symlink-skipping copies, and a
// symlink-skipping hash. This model records each layer's decision independently,
// states the policy in terms of PHYSICAL resolution only, and checks that no
// accepted operation escapes. The discovery facts are named so a mutant can drop
// one; the findings show escapes that survive every fact.
//
// Bounded resolution: a terminal link is followed to chain depth 2
// (chainsWithinTwo forbids a longer acyclic chain); a cycle resolves to nothing,
// as ELOOP does. An admitted module has no non-vendored inner link, so
// ancestor-link following is not lost by dereferencing the terminal node only.
//
// Correspondence (checked by `scripts/formal.py correspondence`):
//
// | model element | Go symbol | file | binding | abstraction |
// |---|---|---|---|---|
// | validates | ScriptFilePath.Validate | pkg/invowkfile/script_file_path.go | TestModulePathContainment_GoldenVectors | absolute dialects and `..` collapse to one refShape each |
// | validates | ValidateEnvFilePath | pkg/invowkfile/validation_filesystem.go | TestModulePathContainment_GoldenVectors | - |
// | validates | ValidateContainerfilePath | pkg/invowkfile/validation_filesystem.go | TestModulePathContainment_GoldenVectors | - |
// | lexContains | validateScriptPathContainment | pkg/invowkfile/implementation.go | TestModulePathContainment_ContainmentLayerGolden | filepath.Rel against the raw module path |
// | physContains | checkModuleFilePhysicalContainment | pkg/invowkfile/module_file_containment.go | TestModulePathContainment_GoldenVectors, TestModulePathContainment_AbsoluteInModuleLinkDenied, TestCheckModuleFilePhysicalContainment, TestCheckModuleFilePhysicalContainment_MissingRoot, TestCheckModuleFilePhysicalContainment_RootIsFile, TestCheckModuleFilePhysicalContainment_LinkToOwnDirectory | os.Root Stat of the path relative to the module directory; a dangling or cyclic link is denied (physTarget none); code stricter than model: a link with an absolute target is denied even inside the module (golden absolute pass, TestModulePathContainment_AbsoluteInModuleLinkDenied) |
// | readOp | ResolveScriptWithFSAndModule | pkg/invowkfile/implementation.go | TestModulePathContainment_GoldenVectors, TestModulePathContainment_AbsoluteInModuleLinkDenied | physical containment check, then the injected reader on the lexical path; code stricter than model: an absolute link target is denied even inside the module (golden absolute pass, TestModulePathContainment_AbsoluteInModuleLinkDenied) |
// | readOp | ResolveWithFSAndModule | pkg/invowkfile/dependency.go | TestModulePathContainment_GoldenVectors, TestModulePathContainment_AbsoluteInModuleLinkDenied | custom-check read, same check; code stricter than model: an absolute link target is denied even inside the module (golden absolute pass, TestModulePathContainment_AbsoluteInModuleLinkDenied) |
// | physContains | readModuleEnvFileWith | internal/runtime/dotenv.go | TestModulePathContainment_GoldenVectors, TestLoadEnvFile_ModuleContext, TestReadModuleEnvFile_PanicIsReadError, TestLoadEnvFile_ModuleLinkToOwnDirectory | env side of physContains: os.Root Stat, Lstat to tell an unresolvable link from a missing file, then os.Root ReadFile; a recovered os.Root panic is a read error |
// | physContains | classifyModuleEnvFileError | internal/runtime/dotenv.go | TestModulePathContainment_GoldenVectors, TestClassifyModuleEnvFileError, TestLoadEnvFile_ModuleContext | env-side classification: a non-OS error or an unresolvable link (Stat fails, Lstat succeeds) is an escape, not-exist from both is absent, any other OS error a read error |
// | readOp | LoadEnvFile | internal/runtime/dotenv.go | TestModulePathContainment_GoldenVectors, TestModulePathContainment_AbsoluteInModuleLinkDenied, TestLoadEnvFile_ModuleContext | module context: contained read through os.Root on the module root, verdict from ErrEnvFileOutsideModule; code stricter than model: an absolute link target is denied even inside the module (golden absolute pass, TestModulePathContainment_AbsoluteInModuleLinkDenied) |
// | workdirCwd | GetEffectiveWorkDir | pkg/invowkfile/invowkfile.go | TestModulePathContainment_ContainerWorkDirGolden | working directory only, never a read target |
// | workdirCwd | getContainerWorkDir | internal/runtime/container_provision.go | TestModulePathContainment_ContainerWorkDirGolden | - |
// | isModuleRejectsLinkedRoot | IsModule | pkg/invowkmod/operations.go | TestModulePathContainment_GoldenVectors | os.Lstat rejects a symlinked root |
// | loadScansNonLinkedRoot | inspectModuleEntry | pkg/invowkmod/operations_validate.go | TestModulePathContainment_GoldenVectors | WalkDir does not descend a symlinked root; invowk_modules skipped by name |
// | copySkipsInnerLinks | copyDir | internal/app/modulecache/cache.go | TestModulePathContainment_GoldenVectors | inner links skipped; a symlinked source root is followed |
// | copySkipsInnerLinks | CopyDir | internal/provision/helpers.go | TestModulePathContainment_GoldenVectors | - |
// | hashSkipsLinks | computeModuleHash | pkg/invowkmod/content_hash.go | TestModulePathContainment_GoldenVectors | non-regular files skipped |
// | unpackRejectsEscape | validateDestinationPath | internal/app/moduleops/packaging.go | - | not bound (decision 5): the model's destination check, unpackInside[destCheck] (refShape = RelInside), transcribes normalizeZIPPath followed by validateDestinationPath; alone, with the Join call shape, validateDestinationPath accepts an AbsPath name, and after normalizeZIPPath its rejection is unreachable on POSIX; follow-up: bind unpackInside to normalizeZIPPath (survivor-triage) |
// | contained | - | formal/alloy/ModulePathContainment.als | TestModulePathContainment_Property | policy: physical resolution and allowed-location predicates only |
// | allowedVendoredSelf | - | formal/alloy/ModulePathContainment.als | - | policy: a vendored child's own root, for the child's own ops |
// | allowedWorkdirCwd | - | formal/alloy/ModulePathContainment.als | - | policy: a declared workdir is a cwd, not a read/copy/hash target |
// | mutantPreFixF13LexicalScriptContainment | ResolveScriptWithFSAndModule | pkg/invowkfile/implementation.go | TestModulePathContainment_VendoredSymlinkScriptReadDenied | regression: the fixed F13 invowk_modules/ symlink hole; the replay asserts both script reads are denied |
// | mutantPreFixF17EnvNoPhysicalContainment | LoadEnvFile | internal/runtime/dotenv.go | TestModulePathContainment_VendoredSymlinkEnvReadDenied | regression: the fixed F17 invowk_modules/ symlink hole for env files; the replay asserts the module env read is denied |
// | mutantPreFixF16JunctionScriptRead | ResolveScriptWithFSAndModule | pkg/invowkfile/implementation.go | TestModulePathContainment_F16Junction | regression: the fixed F16 junction hole (windows-latest CI run 44648263846, job 135996334113); the leg asserts the script and env reads through an inner junction are denied |
// | probeF16 | inspectModuleEntry | pkg/invowkmod/operations_validate.go | TestModulePathContainment_F16Junction | characterisation: Windows junction leg; asserts IsModule and Load still admit an inner junction (windows-latest runs 43657442894 and 44648263846), the reads through it denied by the physical check (F16 fixed), and both copies and the hash contained |

module ModulePathContainment

abstract sig Bool {}
one sig True, False extends Bool {}

// A Name's fold class is a small tag; two names alias only when caseFold is set
// and they share a fold tag. A one-arity function keeps the golden bounded (a
// free Name->Name relation multiplies instances beyond the replay budget).
sig Fold {}
sig Name { fold: one Fold }
one sig FS {
	caseFold: one Bool,
	vendorName: one Name     // the exact name "invowk_modules"
}

// aliases[a, b]: name a and name b resolve to the same directory entry.
pred aliases[a, b: Name] { a = b or (FS.caseFold = True and a.fold = b.fold) }

abstract sig LinkKind {}
one sig SymKind, JunKind extends LinkKind {}

// A filesystem tree rooted at one host Root directory.
abstract sig Node {
	parent: lone Dir,
	label: lone Name
}
sig Dir extends Node {}
sig File extends Node {}
sig Link extends Node {
	tgt: lone Node,   // no tgt = a dangling link
	kind: one LinkKind
}
one sig Root extends Dir {}

// Modules. mroot is the physical module directory. viaLink, when set, is the
// symlink through which a caller reaches the root: IsModule's Lstat rejects it.
sig Module {
	mroot: one Dir,
	viaLink: lone Link
}

// ---------------------------------------------------------------------------
// Tree well-formedness

fact treeShape {
	no Root.parent and no Root.label
	all n: Node - Root | one n.parent and one n.label
	all n: Node | n not in n.^parent            // acyclic parent
	Root in Node.*parent or one Node            // a single tree under Root
	all disj a, b: Node |
		(some a.parent and a.parent = b.parent) implies not aliases[a.label, b.label]
}

pred chainsWithinTwo {
	// no acyclic chain longer than 2 hops; a cycle is allowed and resolves to none
	all l: Link | (l.tgt in Link and l not in l.^tgt) implies no l.tgt.tgt.tgt
}
fact { chainsWithinTwo }

// physTarget: the node a reference to n physically reaches, or none for a
// dangling link, a broken chain, or a cycle.
fun physTarget[n: Node]: lone Node {
	n not in Link => n
	else (n in n.^tgt => none                        // cycle -> error
	      else (n.tgt in Link => n.tgt.tgt else n.tgt))
}

// inTree[d, n]: d is an ancestor of (or equal to) n.
pred inTree[d: Dir, n: Node] { d in n.*parent }

// vendored dir test: a directory named exactly the vendored name.
pred isVendorDir[d: Dir] { d.label = FS.vendorName }

// underVendor[m, n]: n lies under some invowk_modules/ directory of module m.
pred underVendor[m: Module, n: Node] {
	some d: Dir | isVendorDir[d] and inTree[m.mroot, d] and inTree[d, n]
}

pred hasInnerNonVendoredLink[m: Module] {
	some l: Link | inTree[m.mroot, l] and not underVendor[m, l]
}

// ---------------------------------------------------------------------------
// Operations
//
// Each Op belongs to a module and names a reference node (the node reached by
// lexical name traversal, before any link is followed). refShape records the
// syntactic form the value type sees.

abstract sig OpKind {}
one sig ScriptRead, EnvRead, ContainerfileRef, WorkdirCwd, VendorCopy, Hash, Unpack extends OpKind {}

abstract sig RefShape {}
one sig RelInside, ParentEscape, AbsPath extends RefShape {}

sig Op {
	kind: one OpKind,
	owner: one Module,
	ref: one Node,
	refShape: one RefShape
}

// A RelInside op's reference lies lexically inside the module root; any other
// shape lies outside it (a `..` or absolute path that Validate rejects).
fact refPlacement {
	all o: Op | o.refShape = RelInside implies inTree[o.owner.mroot, o.ref]
	all o: Op | o.refShape != RelInside implies not inTree[o.owner.mroot, o.ref]
}

// ---------------------------------------------------------------------------
// Discovery facts and admission
//
// IsModule rejects a symlinked root (viaLink). Load's WalkDir descends only a
// non-symlinked root, so the inner-symlink rejection is conditional on the root
// not being a link. admitted[m, rootReject, scan] holds when the module passes
// discovery under the given active facts.

pred admitted[m: Module, rootReject: Bool, scan: Bool] {
	(rootReject = True) implies no m.viaLink
	(scan = True and no m.viaLink) implies not hasInnerNonVendoredLink[m]
}

// The named discovery facts and their conjunction (every fact active).
pred isModuleRejectsLinkedRoot { all m: Module | some m.viaLink implies not admitted[m, True, True] }
pred loadScansNonLinkedRoot { all m: Module | (no m.viaLink and hasInnerNonVendoredLink[m]) implies not admitted[m, True, True] }
pred discoveryFacts { isModuleRejectsLinkedRoot and loadScansNonLinkedRoot }

// Finding F16 (windows-latest, CI run 43657442894): a junction is not
// ModeSymlink (Go 1.23+), so on Windows Load's scan records inner symlinks only
// and admits a module with an inner non-vendored junction, and before
// module-read-physical-containment a script read through the junction left the
// module. scannedKinds is the set of link kinds the scan rejects: LinkKind is a
// junction-aware scan, SymKind is today's Windows code. phys adds the physical
// containment check of the script and env reads (os.Root), which denies a path
// through a junction (an absolute target). The golden fixes Link.kind =
// SymKind, so admitted[...] above stays exact there. F16 is fixed (outcome d,
// windows-latest CI run 44648263846, Test (windows) job 135996334113): the
// admission still admits such a module, but the read is contained, so
// scriptReadScanGuarded[SymKind, True] passes and the pre-fix configuration
// [SymKind, False] is the regression mutant mutantPreFixF16JunctionScriptRead.
pred admittedScanning[m: Module, scannedKinds: set LinkKind] {
	no m.viaLink
	not (some l: Link | l.kind in scannedKinds and inTree[m.mroot, l] and not underVendor[m, l])
}

pred scriptReadScanGuarded[scannedKinds: set LinkKind, phys: Bool] {
	all o: Op |
		(o.kind in (ScriptRead + EnvRead) and validates[o] and lexContains[o]
			and not underVendor[o.owner, o.ref]
			and admittedScanning[o.owner, scannedKinds]
			and (phys = True implies physContains[o]))
		implies contained[o]
}

// ---------------------------------------------------------------------------
// Implementation layers (transcribed per layer)

pred validates[o: Op] { o.refShape = RelInside }
pred lexContains[o: Op] { inTree[o.owner.mroot, o.ref] }
pred physContains[o: Op] { some physTarget[o.ref] and inTree[o.owner.mroot, physTarget[o.ref]] }

fun touches[o: Op]: lone Node { physTarget[o.ref] }

abstract sig ContMode {}
one sig NoneMode, LexMode, PhysMode extends ContMode {}

pred containCheck[o: Op, mode: ContMode] {
	mode = NoneMode or (mode = LexMode and lexContains[o]) or (mode = PhysMode and physContains[o])
}

// accepts[o]: the operation's final read/copy/hash effect happens (current
// code: value-type validate, then the lexical containment check, on an admitted
// module; a script or env-file read is also physically contained, through
// os.Root, since module-read-physical-containment fixed F13 and F17).
pred accepts[o: Op] {
	o.kind in (ScriptRead + EnvRead + ContainerfileRef + VendorCopy + Hash + Unpack)
	and validates[o] and lexContains[o] and admitted[o.owner, True, True]
	and (o.kind in (ScriptRead + EnvRead) implies physContains[o])
}

// ---------------------------------------------------------------------------
// Independent containment policy (physical resolution only)

pred allowedWorkdirCwd[o: Op] { o.kind = WorkdirCwd }
pred allowedVendoredSelf[o: Op] { no o }   // no vendored-child self-op is modelled

pred contained[o: Op] {
	allowedWorkdirCwd[o] or allowedVendoredSelf[o]
	or (some touches[o] and inTree[o.owner.mroot, touches[o]])
}

// ---------------------------------------------------------------------------
// Safety predicates (parameterised so mutants and findings reuse them)

// readContainedKind: for one op kind, an accepted operation (validate + a
// containment check on an admitted module) stays contained.
pred readContainedKind[k: OpKind, mode: ContMode] {
	all o: Op |
		(o.kind = k and validates[o] and containCheck[o, mode] and admitted[o.owner, True, True])
		implies contained[o]
}

// scriptReadGuarded: the non-vendored read region, protected by the discovery
// facts. Dropping a fact admits a module it should have refused.
pred scriptReadGuarded[rootReject: Bool, scan: Bool] {
	all o: Op |
		(o.kind in (ScriptRead + EnvRead) and validates[o] and lexContains[o]
			and not underVendor[o.owner, o.ref]
			and admitted[o.owner, rootReject, scan])
		implies contained[o]
}

// copyHashInside: a copy or hash of an admitted module reaches only inside
// nodes; the skip-inner-links fact (followLinks = False) excludes links.
pred copyReaches[o: Op, followLinks: Bool] { followLinks = True or o.ref not in Link }
pred copyHashInside[followLinks: Bool] {
	all o: Op |
		(o.kind in (VendorCopy + Hash) and admitted[o.owner, True, True]
			and inTree[o.owner.mroot, o.ref] and copyReaches[o, followLinks])
		implies (some touches[o] and inTree[o.owner.mroot, touches[o]])
}

// copyHashScanInside: copyHashInside over the modules a scan of
// scannedKinds admits (decision 29 of harden-formal-instruments). Base
// admission counts junctions as scanned, so copyHashInside never examines the
// junction modules that today's Windows scan (SymKind) admits (F16); this
// checks copy and hash containment for them.
pred copyHashScanInside[followLinks: Bool, scannedKinds: set LinkKind] {
	all o: Op |
		(o.kind in (VendorCopy + Hash) and admittedScanning[o.owner, scannedKinds]
			and inTree[o.owner.mroot, o.ref] and copyReaches[o, followLinks])
		implies (some touches[o] and inTree[o.owner.mroot, touches[o]])
}

// unpackInside: unpack rejects a `..`/absolute member (validateDestinationPath);
// dropping the check writes outside the destination.
pred unpackInside[destCheck: Bool] {
	all o: Op | (o.kind = Unpack and (destCheck = False or o.refShape = RelInside))
		implies inTree[o.owner.mroot, o.ref]
}

// ---------------------------------------------------------------------------
// Antecedents / witnesses

pred anteScriptGuarded { some o: Op | o.kind = ScriptRead and accepts[o] and not underVendor[o.owner, o.ref] }
pred anteReadContained { some o: Op | o.kind = ScriptRead and validates[o] and physContains[o] and admitted[o.owner, True, True] }
pred anteEnvContained { some o: Op | o.kind = EnvRead and validates[o] and physContains[o] and admitted[o.owner, True, True] }
pred anteCopyHash { some o: Op | o.kind in (VendorCopy + Hash) and admitted[o.owner, True, True] and inTree[o.owner.mroot, o.ref] and o.ref not in Link }
pred anteCopyHashJunction {
	some o: Op | o.kind in (VendorCopy + Hash) and admittedScanning[o.owner, SymKind]
		and inTree[o.owner.mroot, o.ref] and o.ref not in Link
		and some l: Link | l.kind = JunKind and inTree[o.owner.mroot, l] and not underVendor[o.owner, l]
}
pred anteUnpack { some o: Op | o.kind = Unpack and o.refShape = RelInside }

pred witInModuleLink { some l: Link | some m: Module | inTree[m.mroot, l] and not underVendor[m, l] }
pred witEscapingLink { some l: Link | some physTarget[l] and no m: Module | inTree[m.mroot, physTarget[l]] }
pred witChainOfTwo { some l: Link | l.tgt in Link }
pred witDanglingLink { some l: Link | no l.tgt }
pred witCycle { some l: Link | l in l.^tgt }
pred witSymlinkedRoot { some m: Module | some m.viaLink }
pred witJunction { some l: Link | l.kind = JunKind }
pred witCaseFold { FS.caseFold = True and some disj a, b: Name | aliases[a, b] }
pred witVendoredChild { some m: Module | some l: Link | underVendor[m, l] }
pred witAbsRef { some o: Op | o.refShape = AbsPath }

// F13 / F17: a vendored symlink named by a read escapes every discovery fact.
pred vendoredEscape[k: OpKind] {
	some o: Op | o.kind = k and o.ref in Link and underVendor[o.owner, o.ref]
		and o.refShape = RelInside and lexContains[o]
		and admitted[o.owner, True, True]
		and some touches[o] and no m: Module | inTree[m.mroot, touches[o]]
}

// ---------------------------------------------------------------------------
// Golden-vector support. Commands live in formal/manifest.toml.

// goldenScenario pins a canonical single-module layout and lets the golden
// enumerate only the decision-relevant variations (op kind, reference shape,
// whether the reference is a link and where it points, a symlinked root, a
// vendored subtree, and case-folding). Exhaustive enumeration of the full
// relational filesystem is intractable for a replay that materialises every
// instance on a real disk (design D9); this canonical layout keeps the golden
// small while covering every layer decision the findings and calibration need.
pred goldenScenario {
	// one module whose root sits directly under the host root
	Module.mroot.parent = Root
	Op.owner = Module
	// a vendor dir, when present, sits directly under the module root
	all d: Dir | isVendorDir[d] implies d.parent = Module.mroot
	lone d: Dir | isVendorDir[d]
	// interior nodes sit at most two levels under the module root, or directly
	// under the host root (an "outside" sibling of the module)
	all n: Node - Root | let p = n.parent |
		p = Root or p = Module.mroot or p.parent = Module.mroot
	// the reference names an interior or outside node
	// reads and copies name a regular file or a link, never a bare directory
	Op.ref in (File + Link)
	// links do not target the host root
	no l: Link | l.tgt = Root
	// case-folding and junctions are covered by the witness commands and by the
	// Windows-only Go tests; the golden fixes them to keep enumeration bounded
	// for a replay that materialises every instance on disk (design D9)
	FS.caseFold = False
	Link.kind = SymKind
	// a symlinked module root is exercised by the discovery-fact mutant commands
	// (mutantDropLinkedRootReject); the golden keeps the admitted (real) root
	no Module.viaLink
}

one sig Decisions {
	dValidates: set Op,
	dLexContains: set Op,
	dPhysContains: set Op,
	dAccepts: set Op,
	dContained: set Op,
	dTouch: Op -> lone Node
}

pred goldenDecisions {
	Decisions.dValidates = { o: Op | validates[o] }
	Decisions.dLexContains = { o: Op | lexContains[o] }
	Decisions.dPhysContains = { o: Op | physContains[o] }
	Decisions.dAccepts = { o: Op | accepts[o] }
	Decisions.dContained = { o: Op | contained[o] }
	Decisions.dTouch = { o: Op, n: Node | n = touches[o] }
}

// noJunk excludes atoms that cannot influence any decision.
pred noJunk {
	Node = Root + Op.ref.*parent + Link.tgt + Module.mroot.*parent + Module.viaLink
	Name = Node.label + FS.vendorName
	Module = Op.owner
	Link in Op.ref.*parent + Link.tgt + Module.viaLink
	// fold tags matter only under case-folding; otherwise collapse them so they
	// do not multiply instances
	FS.caseFold = False implies (one Fold and Name.fold = Fold)
	Fold in Name.fold
}
