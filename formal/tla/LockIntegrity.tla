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

  Four findings are characterised, each with a fix configuration:
    F5  fresh-cache sync trusted fetched content      (FixedSync, fixed)
    F4  a v1.0 lock without hashes disabled checks     (RejectUnhashed, fixed)
    F6  admission via C's lock, integrity via P's lock (CheckCallerHash, fixed)
    F12 a hashless caller entry admits P's copy        (AdmitRequiresCallerHash, open)

  Two steps mirror code paths that the trace harness observed, without
  changing any property: a sync that fails its commit check leaves the cache
  untouched (the check runs before the cache is read), and a v2 vendor whose
  hash check fails leaves the copied content in invowk_modules/ (the copy
  precedes the check; discovery still rejects that copy). A hashless or v1.0
  lock takes no vendor step, because RequireV2 fails before anything is
  copied. Command-scope admission loads the lock without RequireV2, so a v1.0
  entry reaches CallViaSibling (F12).
*)
\* Correspondence (checked by `scripts/formal.py correspondence`):
\*
\* | model element | Go symbol | file | binding | abstraction |
\* |---|---|---|---|---|
\* | Sync | Resolver.Sync | internal/app/modulesync/resolver.go | TestSyncFreshCacheRejectsChangedContent | one module, one version; the constraint always resolves to it |
\* | Sync commit check | LockedModule.ExpectedContentHash | pkg/invowkmod/lock_integrity.go | TestSyncRejectsRepointedTag | - |
\* | Sync cache handling | Resolver.cacheModule | internal/app/modulesync/cache.go | TestSyncExistingCacheRejectsTamperedContent | content hashing abstracted to content equality |
\* | Sync commit failure | LockedCommitMismatchError | pkg/invowkmod/lock_integrity.go | TestSyncCommitMismatchKeepsCache | a failed commit check leaves the cache untouched |
\* | Vendor | VendorModules | internal/app/moduleops/vendor.go | TestVendorHashMismatchLeavesCopiedContent | the cache is copied, then its hash checked; a mismatch leaves the copy |
\* | Vendor lock load | Resolver.LoadDeclaredFromLock | internal/app/modulesync/resolver.go | TestLockIntegrity_TraceHarness | RequireV2 fails a v1.0 lock before anything is copied |
\* | Discover | VerifyLockedVendoredModuleHash | pkg/invowkmod/verify.go | TestLockIntegrity_HashlessEntryIsRejected | - |
\* | CallViaSibling | IsDeclaredLockedCommandSource | pkg/invowkmod/vendored_policy.go | TestLockIntegrity_SiblingCopyMustMatchCallerHash | P's copy is assumed consistent with P's own lock |
\* | CallViaSibling, hashless entry | IsDeclaredLockedCommandSource | pkg/invowkmod/vendored_policy.go | TestLockIntegrity_HashlessCallerEntryAdmitsSiblingCopy | characterisation: an entry without a hash admits by identity only (F12) |
EXTENDS Naturals

CONSTANTS FixedSync, RejectUnhashed, CheckCallerHash, AdmitRequiresCallerHash, LegacyLock, Mutant

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
       ELSE \* failure: lock untouched. The commit check runs before the cache is
            \* touched; a fresh copy that fails its hash check is removed.
            /\ cache' = (IF ~commitOk \/ useCache THEN cache ELSE "absent") /\ syncs' = syncs + 1
            /\ UNCHANGED <<remoteCommit, lockVer, lockCommit, lockHash, vendored, pVendored, loaded, called>>

(* --- module vendor: copy the cache, then check the locked hash --- *)
\* A hashless entry is refused before the copy (RejectUnhashed). Otherwise the
\* cache is copied; a hash mismatch then fails the command but leaves the copy.
Vendor == cache /= "absent"
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
\* with C's lock entry; CheckCallerHash also compares C's locked hash, and
\* AdmitRequiresCallerHash (the F12 fix) refuses an entry that has none.
CallViaSibling == vendored = "absent" /\ called = "none"
    \* The mutants compare P's copy with P's own lock, which always agrees:
    \* always, or only when C's entry has no hash.
    /\ called' = (IF CheckCallerHash /\ (lockHash /= "none" \/ AdmitRequiresCallerHash)
                     /\ pVendored /= (IF Mutant = "compare_with_sibling_lock" \/
                                        (lockHash = "none" /\ Mutant = "hashless_uses_sibling_lock")
                                     THEN pVendored ELSE lockHash)
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
