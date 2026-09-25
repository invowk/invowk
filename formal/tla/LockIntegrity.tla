\* SPDX-License-Identifier: MPL-2.0
----------------------------- MODULE LockIntegrity -----------------------------
(*
  Lock-to-content integrity for one module dependency M of a calling module C,
  across `module sync`, `module vendor`, discovery, and command-scope admission.

  The trusted content is what C first locked: `Good`, the content of commit
  "c1". An attacker may re-point the upstream tag at commit "c2" (content
  `Evil`), tamper with or wipe the module cache, and tamper with C's vendored
  copy. A sibling module P may vendor its own pinned copy of M, whose content
  (`Good` or a different version, `Other`) is verified against P's lock.

  Three findings are characterised, each with a fix configuration:
    F5  fresh-cache sync trusted fetched content      (FixedSync, fixed)
    F4  a v1.0 lock without hashes disabled checks     (RejectUnhashed, fixed)
    F6  admission via C's lock, integrity via P's lock (CheckCallerHash)
*)
\* Correspondence (checked by `scripts/formal.py correspondence`):
\*
\* | model element | Go symbol | file | binding | abstraction |
\* |---|---|---|---|---|
\* | Sync | Resolver.Sync | internal/app/modulesync/resolver.go | TestSyncFreshCacheRejectsChangedContent | one module, one version; the constraint always resolves to it |
\* | Sync commit check | LockedModule.ExpectedContentHash | pkg/invowkmod/lock_integrity.go | TestSyncRejectsRepointedTag | - |
\* | Sync cache handling | Resolver.cacheModule | internal/app/modulesync/cache.go | TestSyncExistingCacheRejectsTamperedContent | content hashing abstracted to content equality |
\* | Vendor | VendorModules | internal/app/moduleops/vendor.go | - | vendoring copies the cache after checking the locked hash |
\* | Discover | VerifyLockedVendoredModuleHash | pkg/invowkmod/verify.go | TestLockIntegrity_HashlessEntryIsRejected | - |
\* | CallViaSibling | IsDeclaredLockedCommandSource | pkg/invowkmod/vendored_policy.go | TestLockIntegrity_FindingF6AdmissionIgnoresCallerHash | P's copy is assumed consistent with P's own lock |
EXTENDS Naturals

CONSTANTS FixedSync, RejectUnhashed, CheckCallerHash, LegacyLock, Mutant

Good == "Good"
Evil == "Evil"
Other == "Other"
Contents == {Good, Evil, Other}
CommitContent == [c \in {"c1", "c2"} |-> IF c = "c1" THEN Good ELSE Evil]

VARIABLES
    remoteCommit,  \* commit the upstream tag points at
    lockVer,       \* "v1" (no hashes) or "v2"
    lockCommit,    \* locked git_commit
    lockHash,      \* locked content hash, or "none"
    cache,         \* module cache content, or "absent"
    vendored,      \* C's own invowk_modules/M content, or "absent"
    pVendored,     \* sibling P's vendored copy of M
    loaded,        \* what discovery loaded from C's vendored copy
    called,        \* what C executed through P's copy
    syncs          \* completed syncs (bounded)

vars == <<remoteCommit, lockVer, lockCommit, lockHash, cache, vendored, pVendored, loaded, called, syncs>>

Init ==
    /\ remoteCommit = "c1"
    /\ lockVer \in (IF LegacyLock THEN {"v1", "v2"} ELSE {"v2"})
    /\ lockCommit = "c1"
    /\ lockHash = (IF lockVer = "v2" THEN Good ELSE "none")
    /\ cache \in {"absent", Good}
    /\ vendored \in {"absent", Good}
    /\ pVendored \in {Good, Other}
    /\ loaded = "none" /\ called = "none"
    /\ syncs = 0

(* --- Attacker and environment --- *)
MoveTag == remoteCommit = "c1" /\ remoteCommit' = "c2"
    /\ UNCHANGED <<lockVer, lockCommit, lockHash, cache, vendored, pVendored, loaded, called, syncs>>
TamperCache == cache \notin {"absent", Evil} /\ cache' = Evil
    /\ UNCHANGED <<remoteCommit, lockVer, lockCommit, lockHash, vendored, pVendored, loaded, called, syncs>>
WipeCache == cache /= "absent" /\ cache' = "absent"
    /\ UNCHANGED <<remoteCommit, lockVer, lockCommit, lockHash, vendored, pVendored, loaded, called, syncs>>
TamperVendor == vendored \notin {"absent", Evil} /\ vendored' = Evil
    /\ UNCHANGED <<remoteCommit, lockVer, lockCommit, lockHash, cache, pVendored, loaded, called, syncs>>

(* --- module sync --- *)
Sync == syncs < 2 /\
    LET fetched   == CommitContent[remoteCommit]
        commitOk  == ~FixedSync \/ lockCommit = remoteCommit
        \* With RejectUnhashed, a hashless entry never vouches for a cached copy.
        useCache  == cache /= "absent" /\ ~(RejectUnhashed /\ lockHash = "none")
        candidate == IF useCache THEN cache ELSE fetched
        \* The current code compares only an existing cache; FixedSync also checks a fresh copy.
        hashOk    == lockHash = "none" \/ candidate = lockHash \/ (~useCache /\ ~FixedSync)
                     \/ (useCache /\ Mutant = "trust_cache")
    IN IF commitOk /\ hashOk
       THEN /\ lockVer' = "v2" /\ lockCommit' = remoteCommit /\ lockHash' = candidate
            /\ cache' = candidate /\ syncs' = syncs + 1
            /\ UNCHANGED <<remoteCommit, vendored, pVendored, loaded, called>>
       ELSE \* failure: lock untouched; a rejected fresh copy is removed from the cache
            /\ cache' = (IF useCache THEN cache ELSE "absent") /\ syncs' = syncs + 1
            /\ UNCHANGED <<remoteCommit, lockVer, lockCommit, lockHash, vendored, pVendored, loaded, called>>

(* --- module vendor: copy the cache after checking the locked hash --- *)
Vendor == cache /= "absent" /\ (lockHash = "none" \/ cache = lockHash)
    /\ ~(RejectUnhashed /\ lockHash = "none")
    /\ vendored' = cache
    /\ UNCHANGED <<remoteCommit, lockVer, lockCommit, lockHash, cache, pVendored, loaded, called, syncs>>

(* --- discovery of C's own vendored copy --- *)
Discover == vendored /= "absent" /\ loaded = "none"
    /\ loaded' = (IF (lockHash /= "none" /\ vendored /= lockHash)
                     \/ (RejectUnhashed /\ lockHash = "none" /\ Mutant /= "discover_accepts_unhashed")
                  THEN "rejected" ELSE vendored)
    /\ UNCHANGED <<remoteCommit, lockVer, lockCommit, lockHash, cache, vendored, pVendored, called, syncs>>

(* --- C calls M through sibling P's vendored copy --- *)
\* P's copy passed P's own lock check. Admission compares (ModuleID, SourceID)
\* with C's lock entry; CheckCallerHash also compares C's locked hash.
CallViaSibling == vendored = "absent" /\ called = "none"
    \* The mutant compares P's copy with P's own lock, which always agrees.
    /\ called' = (IF CheckCallerHash /\ lockHash /= "none"
                     /\ pVendored /= (IF Mutant = "compare_with_sibling_lock" THEN pVendored ELSE lockHash)
                  THEN "rejected" ELSE pVendored)
    /\ UNCHANGED <<remoteCommit, lockVer, lockCommit, lockHash, cache, vendored, pVendored, loaded, syncs>>

Next == MoveTag \/ TamperCache \/ WipeCache \/ TamperVendor \/ Sync \/ Vendor \/ Discover \/ CallViaSibling
    \/ UNCHANGED vars   \* explicit stuttering: every behaviour may stop

Spec == Init /\ [][Next]_vars

TypeOK == loaded \in Contents \cup {"none", "rejected"} /\ called \in Contents \cup {"none", "rejected"}
\* C only ever runs the content it first locked.
OwnLoadTrusted == loaded \in {"none", "rejected", Good}
SiblingCallTrusted == called \in {"none", "rejected", Good}

WitnessNoMovedTag == remoteCommit = "c1"
WitnessNeverLoadedGood == loaded /= Good
WitnessNeverCalledGood == called /= Good
=============================================================================
