// SPDX-License-Identifier: MPL-2.0
//
// Explicit-only dependency closure.
//
// Invowk resolves only the requirements declared in the root invowkmod.cue and
// fails sync if any resolved module requires something the root did not
// declare. The code checks ONE step (the direct requirements of the resolved
// modules); the policy speaks about the TRANSITIVE closure. This model checks
// that the one-step check decides the closure question, and states tidy's
// result statically. Tidy's round count is checked by rapid, not here.
//
// Closure is keyed by ModuleRefKey (Git URL + path), so two requirements that
// differ only by version are the same key.
//
// Correspondence (checked by `scripts/formal.py correspondence`):
//
// | model element | Go symbol | file | binding | abstraction |
// |---|---|---|---|---|
// | diagKeys | CheckMissingTransitiveDeps | pkg/invowkmod/transitive_policy.go | TestDependencyClosureGoldenVectors | diagnostics compared as a key set; attribution order ignored |
// | diagKeys | CheckMissingVendoredTransitiveDeps | pkg/invowkmod/transitive_policy.go | TestDependencyClosureGoldenVectors | vendored metadata requires modelled as the same relation |
// | tidyResult | tidyToFixedPoint | internal/app/modulesync/resolver_tidy.go | TestTidyToFixedPoint_MatchesClosure | round count and termination checked by rapid |
// | resolved | Resolver.resolveAll | internal/app/modulesync/resolver_deps.go | - | every declared key resolves to exactly one module |
// | Mod.key | ModuleRef.Key | pkg/invowkmod/dependency_types.go | - | version is not part of the key |

module DependencyClosure

sig Key {}
sig Mod {
	key: one Key,
	requires: set Key
}
one sig Root {
	roots: set Key,
	diagSet: set Key,
	tidySet: set Key
}

// Every key names exactly one module (resolution is total and deterministic).
fact keysResolve {
	all k: Key | one key.k
}

fun modOf[k: Key]: one Mod { key.k }
fun step: Key -> Key { { a, b: Key | b in modOf[a].requires } }

// Implementation: resolveAll resolves the declared roots only; the one-step
// check reports every direct requirement of a resolved module that is not a root.
fun resolved: set Mod { key.(Root.roots) }
fun diagKeys: set Key { resolved.requires - Root.roots }

// Mutant implementation: checks every module in the graph, not just resolved ones.
fun diagKeysAllModules: set Key { Mod.requires - Root.roots }

// Specification: the transitive closure of the roots.
fun closure: set Key { Root.roots.^step }
fun tidyResult: set Key { closure - Root.roots }

// Properties are parameterised by the function under test, so a mutant checks
// the same predicate as the property it guards.
pred closedIffNoDiag[d: set Key] { no d iff closure in Root.roots }
pred diagWithinClosure[d: set Key] { d in tidyResult }
pred tidyMinimal[t: set Key] { no t & Root.roots }

syncAcceptsOnlyClosedSets: check { closedIffNoDiag[diagKeys] } for 5 expect 0
anteSync: run { some Root.roots and some diagKeys } for 5 expect 1

diagnosticsWithinClosure: check { diagWithinClosure[diagKeys] } for 5 expect 0
anteDiag: run { some diagKeys } for 5 expect 1

tidyAddsNoDeclaredKey: check { tidyMinimal[tidyResult] } for 5 expect 0
anteTidy: run { some tidyResult } for 5 expect 1

// Rejecting mutants.
mutantAllModules: check { closedIffNoDiag[diagKeysAllModules] } for 5 expect 1
mutantDiagIncludesRoots: check { diagWithinClosure[resolved.requires] } for 5 expect 1
mutantTidyIncludesRoots: check { tidyMinimal[Root.roots.*step] } for 5 expect 1

// Witnesses.
witnessDeepChain: run {
	some disj a, b, c: Key | a in Root.roots and b in a.step and c in b.step and c not in Root.roots + a.step
} for 5 expect 1
witnessCycle: run { some k: Key | k in k.^step and some Root.roots } for 5 expect 1
witnessDiamond: run {
	some disj a, b, c: Key | (a + b) in Root.roots and c in a.step and c in b.step and c not in Root.roots
} for 5 expect 1

// Golden vectors: one-step diagnostics and tidy result for every small graph.
golden: run {
	Root.diagSet = diagKeys
	Root.tidySet = tidyResult
} for 3 expect 1
