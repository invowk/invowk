// SPDX-License-Identifier: MPL-2.0
//
// Lock-file identity and ambiguity.
//
// A lock entry's identity is its module_id, or else the prefix of its
// namespace before "@" (older lock files). FindAmbiguousLockedModuleEntries and
// the vendored-module hash evaluation compute that identity independently; the
// golden replay checks that both agree with this model on every small lock.
// The checked property ties to finding F4: a v2.0 lock never leaves an
// identity without a hash, while a v1.0 lock can, and then vendored
// verification returns "unavailable" instead of comparing content.
//
// Correspondence (checked by `scripts/formal.py correspondence`):
//
// | model element | Go symbol | file | binding | abstraction |
// |---|---|---|---|---|
// | identity | LockedModule.IdentityModuleID | pkg/invowkmod/verify.go | TestLockIdentityGoldenVectors | namespace prefix modelled as an optional Id |
// | ambiguousIds | FindAmbiguousLockedModuleEntries | pkg/invowkmod/verify.go | TestLockIdentityGoldenVectors | - |
// | evaluation | EvaluateVendoredModuleHash | pkg/invowkmod/verify.go | TestLockIdentityGoldenVectors | content hashing abstracted: a single hashed entry is "compared" |
// | v2Parse | parseLockFile | pkg/invowkmod/lockfile_parser.go | - | parser fact: v2.0 entries carry module_id and content_hash |

module LockIdentity

sig Id {}
sig Hash {}
// Each Entry is one lock-file map entry; its key is the entry itself, so keys
// are unique by construction.
sig Entry {
	mid: lone Id,
	nsId: lone Id,
	hash: lone Hash
}
abstract sig Version {}
one sig V1, V2 extends Version {}

one sig Lock {
	version: one Version,
	ambiguous: set Id,
	missing: set Id,
	unhashed: set Id,
	compared: set Id
}

// Parser fact: v2.0 lock files require module_id and content_hash.
pred v2Parse { Lock.version = V2 implies (all e: Entry | some e.mid and some e.hash) }

fun identity[e: Entry]: lone Id { some e.mid implies e.mid else e.nsId }
fun claims[m: Id]: set Entry { { e: Entry | identity[e] = m } }

// Implementation.
fun ambiguousIds: set Id { { m: Id | #claims[m] > 1 } }
pred evalMissing[m: Id] { no claims[m] }
pred evalAmbiguous[m: Id] { #claims[m] > 1 }
pred evalUnhashed[m: Id] { one claims[m] and no claims[m].hash }
pred evalCompared[m: Id] { one claims[m] and some claims[m].hash }

pred noUnhashedIdentity[p: Version] { Lock.version = p implies (no m: Id | evalUnhashed[m]) }

v2NeverUnhashed: check { v2Parse implies noUnhashedIdentity[V2] } for 4 expect 0
anteV2: run { v2Parse and Lock.version = V2 and some Entry } for 4 expect 1

// Rejecting mutant: without the parser fact, a v2 lock could skip verification.
mutantV2WithoutParseFact: check { noUnhashedIdentity[V2] } for 4 expect 1

// Witnesses.
witnessV1Unhashed: run { v2Parse and Lock.version = V1 and some m: Id | evalUnhashed[m] } for 4 expect 1
witnessFallbackCollision: run {
	v2Parse and Lock.version = V1
	some disj a, b: Entry | no a.mid and some b.mid and a.nsId = b.mid
} for 4 expect 1
witnessEmptyIdentity: run { v2Parse and some e: Entry | no identity[e] } for 4 expect 1

pred noJunk { Hash in Entry.hash }

// Golden vectors: the implementation's decisions for every small lock.
golden: run {
	v2Parse
	noJunk
	Lock.ambiguous = ambiguousIds
	Lock.missing = { m: Id | evalMissing[m] }
	Lock.unhashed = { m: Id | evalUnhashed[m] }
	Lock.compared = { m: Id | evalCompared[m] }
} for 3 but 2 Id, 2 Hash expect 1
