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
// | readOp | ResolveScriptWithFSAndModule | pkg/invowkfile/implementation.go | TestModulePathContainment_GoldenVectors | os.ReadFile follows links physically |
// | readOp | ResolveWithFSAndModule | pkg/invowkfile/dependency.go | TestModulePathContainment_GoldenVectors | custom-check read |
// | readOp | LoadEnvFile | internal/runtime/dotenv.go | TestModulePathContainment_GoldenVectors | - |
// | workdirCwd | GetEffectiveWorkDir | pkg/invowkfile/invowkfile.go | TestModulePathContainment_GoldenVectors | working directory only, never a read target |
// | workdirCwd | getContainerWorkDir | internal/runtime/container_provision.go | TestModulePathContainment_ContainerWorkDirGolden | - |
// | isModuleRejectsLinkedRoot | IsModule | pkg/invowkmod/operations.go | TestModulePathContainment_GoldenVectors | os.Lstat rejects a symlinked root |
// | loadScansNonLinkedRoot | inspectModuleEntry | pkg/invowkmod/operations_validate.go | TestModulePathContainment_GoldenVectors | WalkDir does not descend a symlinked root; invowk_modules skipped by name |
// | copySkipsInnerLinks | copyDir | internal/app/modulecache/cache.go | TestModulePathContainment_GoldenVectors | inner links skipped; a symlinked source root is followed |
// | copySkipsInnerLinks | CopyDir | internal/provision/helpers.go | TestModulePathContainment_GoldenVectors | - |
// | hashSkipsLinks | computeModuleHash | pkg/invowkmod/content_hash.go | TestModulePathContainment_GoldenVectors | non-regular files skipped |
// | unpackRejectsEscape | validateDestinationPath | internal/app/moduleops/packaging.go | TestModulePathContainment_GoldenVectors | filepath.Rel of destination vs root |
// | contained | - | formal/alloy/ModulePathContainment.als | TestModulePathContainment_Property | policy: physical resolution and allowed-location predicates only |
// | allowedVendoredSelf | - | formal/alloy/ModulePathContainment.als | - | policy: a vendored child's own root, for the child's own ops |
// | allowedWorkdirCwd | - | formal/alloy/ModulePathContainment.als | - | policy: a declared workdir is a cwd, not a read/copy/hash target |
// | ValidateScriptPath | ValidateScriptPath | pkg/invowkmod/invowkmod.go | - | dead code: no production caller (fix change wires or deletes it) |
// | checkSymlinkSafety | checkSymlinkSafety | pkg/invowkmod/invowkmod.go | - | dead code: no production caller |
// | findingF13 | ResolveScriptWithFSAndModule | pkg/invowkfile/implementation.go | TestModulePathContainment_F13VendoredSymlink | invowk_modules/ symlink hole |
// | findingF17 | LoadEnvFile | internal/runtime/dotenv.go | TestModulePathContainment_F17EnvVendoredSymlink | invowk_modules/ symlink hole for env files |

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
// module).
pred accepts[o: Op] {
	o.kind in (ScriptRead + EnvRead + ContainerfileRef + VendorCopy + Hash + Unpack)
	and validates[o] and lexContains[o] and admitted[o.owner, True, True]
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
