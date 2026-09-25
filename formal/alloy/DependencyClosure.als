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

// Commands live in formal/manifest.toml; scripts/formal.py renders them into a
// staged copy of this model under artifacts/formal/DependencyClosure/.
