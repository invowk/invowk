\* SPDX-License-Identifier: MPL-2.0
------------------------ MODULE ConcurrentModuleEdits ------------------------
(*
  Two honest invowk processes running module commands concurrently, in one
  project or in two projects that share one module cache. Every operation is
  decomposed into the reads, fetch windows, and writes the code performs, in
  the code's order. `Scenario` selects each process's program (design D1);
  TLC configurations cannot assign function values, so the programs live in
  this module and the scenario is an ordinary string constant.

  Named assumptions:
    A1  Every fspath.AtomicWriteFile call (LockFile.Save, AddRequirement,
        RemoveRequirement) is one atomic replace; AtomicWrite owns crash
        durability and is not re-checked here.
    A2  A fetch (ListVersions, clone/fetch, checkout) has no shared effect,
        except that an abstract (non-source) fetch fills the cache with the
        honest content of the resolved version, idempotently. Scenarios that
        race on the shared Git checkout model it with Checkout/CopyToCache.
    A3  Every actor is an honest invowk process; attackers are LockIntegrity's.
    A4  A read that observes a truncated, partially written, or transiently
        removed file is recorded in `torn` and aborts the reader (a parse error
        or a wrong snapshot); ReadersSeeWholeFiles already fails at that read.
    A5  Content hashing is abstracted to content equality.

  Findings on today's code (base constants AdvisoryLock = "none",
  AtomicRestore = FALSE, Mutant = "none"):
    F8   lost updates: sync/update overwrite a concurrent add or remove, and an
         add's rollback erases another process's add
    F9   shared Git worktree: the lock records one commit with the hash of the
         other version's content
    F10  the rollback's os.WriteFile truncates in place, so a concurrent reader
         sees a partial file
  A vendor that returns ok from a lock replaced before it returned is a
  proposed finding without an id (escalated for allocation).
*)
\* Correspondence (checked by `scripts/formal.py correspondence`):
\*
\* | model element | Go symbol | file | binding | abstraction |
\* |---|---|---|---|---|
\* | AddProg (add_add_same) | Resolver.AddModuleDependency | internal/app/modulesync/use_cases.go | TestConcurrentEdits_FindingF8_DuplicateAddRollsBackOtherAdd | snapshots (126,130); rollback on edit error (140) |
\* | AddProg (sync_add) | Resolver.AddModuleDependency | internal/app/modulesync/use_cases.go | TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd | the concurrent add |
\* | RemoveProg | Resolver.RemoveModuleDependency | internal/app/modulesync/use_cases.go | TestConcurrentEdits_FindingF8_UpdateResurrectsConcurrentRemove | RemoveRequirement never fails, so no rollback path |
\* | Restore | fileSnapshot.restore | internal/app/modulesync/use_cases.go | - | F10 is model-only: os.WriteFile has no seam between its truncate and its write |
\* | SyncProg | SyncModule | internal/app/modulesync/use_cases.go | - | wrapper; its LoadRequirements-then-Sync body is transcribed in the sync replay |
\* | UpdateProg | UpdateModule | internal/app/modulesync/use_cases.go | - | wrapper with the real GitFetcher |
\* | TidyProg | TidyModule | internal/app/modulesync/use_cases.go | - | no fetcher seam; modelled as a reader of requires and the lock |
\* | AddProg (fetch_v1_v2) | Resolver.Add | internal/app/modulesync/resolver.go | TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent | lock read (136), resolve (142), re-read and save (149-158) |
\* | WriteLock remove | Resolver.Remove | internal/app/modulesync/resolver.go | TestConcurrentEdits_FindingF8_UpdateResurrectsConcurrentRemove | read (173) and save (200) with no fetch between |
\* | WriteLock rebuild | Resolver.Sync | internal/app/modulesync/resolver.go | TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd | a new lock built only from the pre-fetch requires (300-307) |
\* | WriteLock update (update_add) | Resolver.Update | internal/app/modulesync/resolver.go | TestConcurrentEdits_FindingF8_UpdateOverwritesConcurrentAdd | the pre-fetch lock with updated entries (216, 264) |
\* | WriteLock update (update_remove) | Resolver.Update | internal/app/modulesync/resolver.go | TestConcurrentEdits_FindingF8_UpdateResurrectsConcurrentRemove | - |
\* | Checkout, CopyToCache | Resolver.resolveOne | internal/app/modulesync/resolver_deps.go | TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent | Fetch (130), then copy from the shared worktree (151-170) |
\* | CopyToCache | Resolver.cacheModule | internal/app/modulesync/cache.go | TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent | stat, copy, hash; the fresh-copy branch trusts the worktree |
\* | Checkout | GitFetcher.checkout | internal/app/modulesync/git.go | TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent | one worktree per URL, not per version (94-117, 467) |
\* | WriteMod add | AddRequirement | pkg/invowkmod/invowkmod_edit.go | - | narrow read (29) to write (73) window with no seam |
\* | WriteMod remove | RemoveRequirement | pkg/invowkmod/invowkmod_edit.go | - | narrow read (82) to write (137) window with no seam |
\* | WriteLock | LockFile.Save | pkg/invowkmod/lockfile.go | TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd | atomic replace (A1) |
\* | VendorProg | vendorDependenciesWithResolver | internal/app/moduleops/vendor_dependencies.go | TestConcurrentEdits_VendorFromStaleLock | characterisation: LoadDeclaredFromLock (127), then VendorModules (104) |
\* | VendorRemove, VendorCopy | VendorModules | internal/app/moduleops/vendor.go | - | the RemoveAll (107) to copy (115) window has no seam |
EXTENDS Naturals, Sequences, FiniteSets

CONSTANTS
    Procs,          \* two model values
    Projects,       \* one project, or two sharing the cache (fetch_v1_v2)
    Scenario,       \* selects each process's program (design D1)
    AdvisoryLock,   \* "none" | "project" | "project_and_source"
    AtomicRestore,  \* TRUE: the rollback replaces each file atomically
    Mutant

Scenarios == {"sync_add", "update_add", "update_remove", "add_add_same", "rollback_read",
              "fetch_v1_v2", "vendor_sync", "vendor_vendor", "sync_update_same_source"}
Mutants == {"none", "lock_after_snapshot", "release_between_files", "reverse_order",
            "vendor_outside_lock", "restore_remove_then_write"}

ASSUME Cardinality(Procs) = 2 /\ Cardinality(Projects) \in {1, 2}
ASSUME Scenario \in Scenarios /\ Mutant \in Mutants
ASSUME AdvisoryLock \in {"none", "project", "project_and_source"} /\ AtomicRestore \in BOOLEAN

P1 == CHOOSE p \in Procs : TRUE
P2 == CHOOSE p \in Procs \ {P1} : TRUE
PA == CHOOSE j \in Projects : TRUE
PB == IF Cardinality(Projects) = 2 THEN CHOOSE j \in Projects \ {PA} : TRUE ELSE PA

Keys == {"a", "b", "x", "r"}          \* one Git source per key
Versions == {"v1", "v2"}
Commits == {"c1", "c2"}
Contents == {"C1", "C2"}
Commit == [v \in Versions |-> IF v = "v1" THEN "c1" ELSE "c2"]
CommitContent == [c \in Commits |-> IF c = "c1" THEN "C1" ELSE "C2"]
Entry(v) == [ver |-> v, commit |-> Commit[v], hash |-> CommitContent[Commit[v]]]
Entries == [ver : Versions, commit : Commits, hash : Contents]
EmptyLock == [k \in {} |-> Entry("v1")]
IsLock(f) == DOMAIN f \subseteq Keys /\ \A k \in DOMAIN f : f[k] \in Entries
Merge(new, old) == [k \in DOMAIN new \cup DOMAIN old |-> IF k \in DOMAIN new THEN new[k] ELSE old[k]]
Visibility == {"whole", "absent", "partial", "gap"}
Torn(vis) == vis \in {"partial", "gap"}
Results == {"running", "ok", "rolled_back", "err"}
ProjL(j) == <<"proj", j>>
SrcL(k) == <<"src", k>>
LockNames == {ProjL(j) : j \in Projects} \cup {SrcL(k) : k \in Keys}

(* --- Scenario programs (design D1) --- *)
Op(kind, key, ver, src) == [kind |-> kind, key |-> key, ver |-> ver, src |-> src]
Plan(p) ==
    CASE Scenario = "sync_add" ->
            IF p = P1 THEN Op("sync", "-", "v1", FALSE) ELSE Op("add", "b", "v1", FALSE)
      [] Scenario = "update_add" ->
            IF p = P1 THEN Op("update", "-", "v2", FALSE) ELSE Op("add", "b", "v1", FALSE)
      [] Scenario = "update_remove" ->
            IF p = P1 THEN Op("update", "-", "v2", FALSE) ELSE Op("remove", "a", "-", FALSE)
      [] Scenario = "add_add_same" -> Op("add", "x", "v1", FALSE)
      [] Scenario = "rollback_read" ->
            IF p = P1 THEN Op("add", "x", "v1", FALSE) ELSE Op("tidy", "-", "-", FALSE)
      [] Scenario = "fetch_v1_v2" ->
            IF p = P1 THEN Op("add", "r", "v1", TRUE) ELSE Op("add", "r", "v2", TRUE)
      [] Scenario = "vendor_sync" ->
            IF p = P1 THEN Op("vendor", "a", "-", FALSE) ELSE Op("update", "-", "v2", FALSE)
      [] Scenario = "vendor_vendor" -> Op("vendor", "a", "-", FALSE)
      [] Scenario = "sync_update_same_source" ->
            IF p = P1 THEN Op("sync", "a", "v1", TRUE) ELSE Op("update", "a", "v2", TRUE)
ProjOf(p) == IF Scenario = "fetch_v1_v2" /\ p = P2 THEN PB ELSE PA
\* The outcome each program is written to reach: rollback_read's adder finds x
\* already required, so its edit fails and it rolls back by design.
Intended(p) == IF Scenario = "rollback_read" /\ p = P1 THEN "rolled_back" ELSE "ok"

S(op, arg, key, ver) == [op |-> op, arg |-> arg, key |-> key, ver |-> ver, lk |-> <<>>]
St(op) == S(op, "-", "-", "-")
Acq(l) == [St("Acquire") EXCEPT !.lk = l]
Rel(l) == [St("Release") EXCEPT !.lk = l]
Opt(c, s) == IF c THEN <<s>> ELSE <<>>

\* Project mutex (fix): held from before the first read of either file until
\* after the last write or rollback, by every operation including the reader.
UseProj(kind) == AdvisoryLock /= "none" /\ ~(Mutant = "vendor_outside_lock" /\ kind = "vendor")
UseSrc == AdvisoryLock = "project_and_source"
TwoWrites(kind) == kind \in {"add", "remove"}
Early(kind, j) == Opt(UseProj(kind) /\ Mutant /= "lock_after_snapshot", Acq(ProjL(j)))
Late(kind, j) == Opt(UseProj(kind) /\ Mutant = "lock_after_snapshot", Acq(ProjL(j)))
Between(kind, j) == Opt(UseProj(kind) /\ Mutant = "release_between_files" /\ TwoWrites(kind), Rel(ProjL(j)))
Final(kind, j) == Opt(UseProj(kind) /\ ~(Mutant = "release_between_files" /\ TwoWrites(kind)), Rel(ProjL(j)))
\* reverse_order: P2 takes the source mutex before the project mutex.
Reversed(p) == UseSrc /\ Mutant = "reverse_order" /\ p = P2

\* resolveOne (resolver_deps.go:114-130): list versions, fetch, check out; the
\* source-fetching form splits checkout (git.go:410-413) from the cache copy
\* (resolver_deps.go:170, cache.go:62-89), under the source mutex when fixed.
FetchSteps(p, o, keyMode) ==
    IF o.src
    THEN Opt(UseSrc /\ ~Reversed(p), Acq(SrcL(o.key)))   \* reversed: taken first (Program)
         \o <<S("Checkout", "-", o.key, o.ver), S("CopyToCache", "-", o.key, o.ver)>>
         \o Opt(UseSrc, Rel(SrcL(o.key)))
    ELSE <<S("Fetch", keyMode, o.key, o.ver)>>

Program(p) ==
    LET o == Plan(p)
        j == ProjOf(p)
        k == o.kind
        body ==
          CASE k = "add" ->                 \* Resolver.AddModuleDependency (use_cases.go:122)
                 Early(k, j) \o <<St("Snapshot")>> \o Late(k, j)          \* use_cases.go:126,130
                 \o <<St("ReadLock")>>                                    \* resolver.go:136
                 \o FetchSteps(p, o, "key")                               \* resolver.go:142
                 \o <<S("WriteLock", "add", o.key, "-")>>                 \* resolver.go:149-158
                 \o Between(k, j)
                 \o <<St("ReadMod"), S("WriteMod", "add", o.key, "-"),   \* invowkmod_edit.go:29,73
                      S("Restore", "mod", "-", "-"), S("Restore", "lock", "-", "-")>> \* use_cases.go:140
                 \o Final(k, j)
            [] k = "remove" ->              \* Resolver.RemoveModuleDependency (use_cases.go:163)
                 Early(k, j) \o <<St("Snapshot")>> \o Late(k, j)          \* use_cases.go:164,168
                 \o <<St("ReadLock"), S("WriteLock", "remove", o.key, "-")>> \* resolver.go:173,200
                 \o Between(k, j)
                 \o <<St("ReadMod"), S("WriteMod", "remove", o.key, "-")>> \* invowkmod_edit.go:82,137
                 \o Final(k, j)
            [] k = "sync" ->                \* SyncModule -> Resolver.Sync
                 Early(k, j) \o <<St("ReadMod")>> \o Late(k, j)           \* use_cases.go:308
                 \o <<St("ReadLock")>>                                    \* resolver.go:283
                 \o FetchSteps(p, o, "req")                               \* resolver.go:289
                 \o <<S("WriteLock", "rebuild", "-", "-")>>               \* resolver.go:300-307
                 \o Final(k, j)
            [] k = "update" ->              \* UpdateModule -> Resolver.Update
                 Early(k, j) \o <<St("ReadLock")>> \o Late(k, j)       \* resolver.go:216
                 \o FetchSteps(p, o, "lock")                              \* resolver.go:253
                 \o <<S("WriteLock", "update", "-", "-")>>                \* resolver.go:264
                 \o Final(k, j)
            [] k = "tidy" ->                \* TidyModule, as a reader
                 Early(k, j) \o <<St("ReadMod")>> \o Late(k, j)           \* use_cases.go:330
                 \o <<St("ReadLock")>>                                    \* resolver_tidy.go:29
                 \o Final(k, j)
            [] k = "vendor" ->              \* vendorDependenciesWithResolver
                 Early(k, j) \o <<St("ReadMod")>> \o Late(k, j)           \* vendor_dependencies.go:75
                 \o <<St("ReadLock")>>                                    \* vendor_dependencies.go:122,127
                 \o <<S("VendorRemove", "-", o.key, "-"), S("VendorCopy", "-", o.key, "-")>> \* vendor.go:107,115
                 \o Final(k, j)
    IN Opt(Reversed(p) /\ o.src, Acq(SrcL(o.key))) \o body \o <<St("Return")>>

(* --- State (design D2) --- *)
VARIABLES
    mod, modVis,        \* per project: requires keys, and invowkmod.cue visibility
    lock, lockVis,      \* per project: lock entries, and lock file visibility
    worktree,           \* per source: version checked out in the shared worktree
    cache,              \* per source and version: cached content, or "absent"
    vendor,             \* per project and key: invowk_modules/ content, or "absent"
    pc, result, holds,  \* per process: program counter, outcome, mutexes held
    snapLock, snapMod,  \* per process: rollback snapshots
    readLock, readMod,  \* per process: local copies read from the files
    res,                \* per process: resolved entries
    editErr,            \* per process: the invowkmod.cue edit failed (rollback)
    rphase,             \* per process: 1 between a restore's two steps
    torn,               \* processes that read a partial or transiently absent file
    staleVendor         \* vendors whose verified copy the lock no longer named

fileVars == <<mod, modVis, lock, lockVis>>
srcVars == <<worktree, cache, vendor>>
histVars == <<torn, staleVendor>>
localVars == <<snapLock, snapMod, readLock, readMod, res, editErr, rphase>>
vars == <<fileVars, srcVars, pc, result, holds, localVars, histVars>>

InitMod == CASE Scenario = "rollback_read" -> {"a", "x"}
             [] Scenario = "fetch_v1_v2" -> {}
             [] OTHER -> {"a"}
InitLock == IF Scenario = "fetch_v1_v2" THEN EmptyLock ELSE [k \in {"a"} |-> Entry("v1")]

Init ==
    /\ mod = [j \in Projects |-> InitMod]
    /\ lock = [j \in Projects |-> InitLock]
    /\ modVis = [j \in Projects |-> "whole"]
    /\ lockVis = [j \in Projects |-> "whole"]
    /\ worktree = [k \in Keys |-> "none"]
    /\ cache = [k \in Keys |-> [v \in Versions |->
                  IF k = "a" /\ v = "v1" /\ Scenario /= "fetch_v1_v2" THEN "C1" ELSE "absent"]]
    /\ vendor = [j \in Projects |-> [k \in Keys |-> "absent"]]
    /\ pc = [p \in Procs |-> 1]
    /\ result = [p \in Procs |-> "running"]
    /\ holds = [p \in Procs |-> {}]
    /\ snapLock = [p \in Procs |-> [present |-> FALSE, val |-> EmptyLock]]
    /\ snapMod = [p \in Procs |-> [present |-> FALSE, val |-> {}]]
    /\ readLock = [p \in Procs |-> EmptyLock]
    /\ readMod = [p \in Procs |-> {}]
    /\ res = [p \in Procs |-> EmptyLock]
    /\ editErr = [p \in Procs |-> FALSE]
    /\ rphase = [p \in Procs |-> 0]
    /\ torn = {}
    /\ staleVendor = {}

(* --- Helpers --- *)
Running(p) == result[p] = "running"
Step(p) == Program(p)[pc[p]]
At(p, op) == Running(p) /\ pc[p] <= Len(Program(p)) /\ Step(p).op = op
LockValue(j) == IF lockVis[j] = "whole" THEN lock[j] ELSE EmptyLock
LockKeys(j) == DOMAIN LockValue(j)

Advance(p) == pc' = [pc EXCEPT ![p] = @ + 1] /\ UNCHANGED <<result, holds>>
\* An error return: the process exits, which releases its advisory locks.
Abort(p) == /\ result' = [result EXCEPT ![p] = "err"]
            /\ holds' = [holds EXCEPT ![p] = {}]
            /\ UNCHANGED pc
\* A read that sees a torn file (assumption A4).
TornRead(p) == torn' = torn \cup {p} /\ Abort(p) /\ UNCHANGED <<fileVars, srcVars, localVars, staleVendor>>

SetLock(j, vis, val) == /\ lock' = [lock EXCEPT ![j] = val]
                        /\ lockVis' = [lockVis EXCEPT ![j] = vis]
                        /\ UNCHANGED <<mod, modVis>>
SetMod(j, vis, val) == /\ mod' = [mod EXCEPT ![j] = val]
                       /\ modVis' = [modVis EXCEPT ![j] = vis]
                       /\ UNCHANGED <<lock, lockVis>>

(* --- Generic step actions, dispatched on the program counter --- *)
Snapshot(p) ==
    /\ At(p, "Snapshot")
    /\ LET j == ProjOf(p) IN
       /\ IF Torn(lockVis[j]) \/ Torn(modVis[j]) THEN TornRead(p)
          ELSE /\ snapLock' = [snapLock EXCEPT ![p] = [present |-> lockVis[j] = "whole", val |-> lock[j]]]
               /\ snapMod' = [snapMod EXCEPT ![p] = [present |-> modVis[j] = "whole", val |-> mod[j]]]
               /\ Advance(p)
               /\ UNCHANGED <<fileVars, srcVars, readLock, readMod, res, editErr, rphase, histVars>>

ReadMod(p) ==
    /\ At(p, "ReadMod")
    /\ LET j == ProjOf(p) IN
       /\ IF Torn(modVis[j]) THEN TornRead(p)
          ELSE /\ readMod' = [readMod EXCEPT ![p] = mod[j]]
               /\ Advance(p)
               /\ UNCHANGED <<fileVars, srcVars, snapLock, snapMod, readLock, res, editErr, rphase, histVars>>

\* A missing lock reads as empty (LoadLockFile, loadLockedModules).
ReadLock(p) ==
    /\ At(p, "ReadLock")
    /\ LET j == ProjOf(p) IN
       /\ IF Torn(lockVis[j]) THEN TornRead(p)
          ELSE /\ readLock' = [readLock EXCEPT ![p] = LockValue(j)]
               /\ Advance(p)
               /\ UNCHANGED <<fileVars, srcVars, snapLock, snapMod, readMod, res, editErr, rphase, histVars>>

\* Abstract fetch (A2). The honest content always matches a locked entry, so
\* resolveOne's commit and hash checks never fire here.
Fetch(p) ==
    /\ At(p, "Fetch")
    /\ LET s == Step(p)
           ks == CASE s.arg = "req" -> readMod[p]
                   [] s.arg = "lock" -> DOMAIN readLock[p]
                   [] OTHER -> {s.key}
       IN
       /\ res' = [res EXCEPT ![p] = [k \in ks |-> Entry(s.ver)]]
       /\ cache' = [k \in Keys |-> [v \in Versions |->
                      IF k \in ks /\ v = s.ver /\ cache[k][v] = "absent" THEN Entry(v).hash ELSE cache[k][v]]]
       /\ Advance(p)
       /\ UNCHANGED <<fileVars, worktree, vendor, snapLock, snapMod, readLock, readMod, editErr, rphase, histVars>>

\* GitFetcher.checkout: Force-checkout of the tag in the one worktree per URL.
Checkout(p) ==
    /\ At(p, "Checkout")
    /\ LET s == Step(p) IN
       /\ worktree' = [worktree EXCEPT ![s.key] = s.ver]
       /\ Advance(p)
       /\ UNCHANGED <<fileVars, cache, vendor, snapLock, snapMod, readLock, readMod, res, editErr, rphase, histVars>>

\* cacheModule: an existing copy is verified against the locked hash; a fresh
\* copy is taken from whatever the shared worktree holds now.
CopyToCache(p) ==
    /\ At(p, "CopyToCache")
    /\ LET s == Step(p)
           k == s.key
           v == s.ver
           expected == IF k \in DOMAIN readLock[p] /\ readLock[p][k].ver = v THEN readLock[p][k].hash ELSE "none"
           fresh == cache[k][v] = "absent"
           content == IF fresh THEN CommitContent[Commit[worktree[k]]] ELSE cache[k][v]
           entry == [ver |-> v, commit |-> Commit[v], hash |-> content]
       IN
       /\ IF expected /= "none" /\ content /= expected
          THEN Abort(p) /\ UNCHANGED <<fileVars, srcVars, localVars, histVars>>
          ELSE /\ cache' = [cache EXCEPT ![k][v] = content]
               /\ res' = [res EXCEPT ![p] = Merge([key \in {k} |-> entry], res[p])]
               /\ Advance(p)
               /\ UNCHANGED <<fileVars, worktree, vendor, snapLock, snapMod, readLock, readMod, editErr, rphase, histVars>>

\* LockFile.Save (A1). Add re-reads the lock before saving; remove, sync, and
\* update write from their earlier reads.
WriteLock(p) ==
    /\ At(p, "WriteLock")
    /\ LET j == ProjOf(p)
           s == Step(p)
           rl == readLock[p]
       IN
       /\ CASE s.arg = "add" ->
                 IF Torn(lockVis[j]) THEN TornRead(p)
                 ELSE /\ SetLock(j, "whole", Merge(res[p], LockValue(j)))
                      /\ Advance(p) /\ UNCHANGED <<srcVars, localVars, histVars>>
            [] s.arg = "remove" ->
                 IF s.key \notin DOMAIN rl   \* resolveIdentifier finds no match
                 THEN Abort(p) /\ UNCHANGED <<fileVars, srcVars, localVars, histVars>>
                 ELSE /\ SetLock(j, "whole", [k \in DOMAIN rl \ {s.key} |-> rl[k]])
                      /\ Advance(p) /\ UNCHANGED <<srcVars, localVars, histVars>>
            [] s.arg = "rebuild" ->
                 /\ SetLock(j, "whole", [k \in readMod[p] |-> res[p][k]])
                 /\ Advance(p) /\ UNCHANGED <<srcVars, localVars, histVars>>
            [] s.arg = "update" ->
                 /\ SetLock(j, "whole", [k \in DOMAIN rl |-> res[p][k]])
                 /\ Advance(p) /\ UNCHANGED <<srcVars, localVars, histVars>>

\* AddRequirement / RemoveRequirement write from the ReadMod copy. A duplicate
\* add fails (ErrModuleAlreadyExists) and falls through to the Restore steps
\* that follow; a successful edit skips them.
AfterRestores(p) ==
    CHOOSE i \in pc[p] + 1 .. Len(Program(p)) :
        Program(p)[i].op /= "Restore" /\ \A m \in pc[p] + 1 .. i - 1 : Program(p)[m].op = "Restore"
WriteMod(p) ==
    /\ At(p, "WriteMod")
    /\ LET j == ProjOf(p)
           s == Step(p)
       IN
       /\ IF s.arg = "add" /\ s.key \in readMod[p]
          THEN /\ editErr' = [editErr EXCEPT ![p] = TRUE]
               /\ Advance(p)
               /\ UNCHANGED <<fileVars, srcVars, snapLock, snapMod, readLock, readMod, res, rphase, histVars>>
          ELSE /\ SetMod(j, "whole", IF s.arg = "add" THEN readMod[p] \cup {s.key} ELSE readMod[p] \ {s.key})
               /\ pc' = [pc EXCEPT ![p] = AfterRestores(p)]
               /\ UNCHANGED <<result, holds, srcVars, localVars, histVars>>

\* fileSnapshot.restore, declaration first and then the lock (use_cases.go:140,
\* 245-263). A snapshot of a missing file is restored with os.Remove; an existing
\* one with os.WriteFile, which truncates and then writes.
RestoreSnap(p) == IF Step(p).arg = "mod" THEN snapMod[p] ELSE snapLock[p]
\* Only a whole file holds the snapshot's content; the other states are empty.
RestoreSet(p, vis) ==
    IF Step(p).arg = "mod"
    THEN SetMod(ProjOf(p), vis, IF vis = "whole" THEN snapMod[p].val ELSE {})
    ELSE SetLock(ProjOf(p), vis, IF vis = "whole" THEN snapLock[p].val ELSE EmptyLock)
RemoveThenWrite == AtomicRestore /\ Mutant = "restore_remove_then_write"

RestoreRemove(p) ==
    /\ At(p, "Restore")
    /\ rphase[p] = 0
    /\ ~RestoreSnap(p).present \/ RemoveThenWrite
    /\ IF ~RestoreSnap(p).present
       THEN RestoreSet(p, "absent") /\ Advance(p) /\ UNCHANGED rphase
       ELSE RestoreSet(p, "gap") /\ rphase' = [rphase EXCEPT ![p] = 1]
            /\ UNCHANGED <<pc, result, holds>>
    /\ UNCHANGED <<srcVars, snapLock, snapMod, readLock, readMod, res, editErr, histVars>>

RestoreTruncate(p) ==
    /\ At(p, "Restore")
    /\ rphase[p] = 0 /\ RestoreSnap(p).present /\ ~AtomicRestore
    /\ RestoreSet(p, "partial")
    /\ rphase' = [rphase EXCEPT ![p] = 1]
    /\ UNCHANGED <<pc, result, holds, srcVars, snapLock, snapMod, readLock, readMod, res, editErr, histVars>>

RestoreWrite(p) ==
    /\ At(p, "Restore")
    /\ RestoreSnap(p).present
    /\ rphase[p] = 1 \/ (AtomicRestore /\ ~RemoveThenWrite)
    /\ RestoreSet(p, "whole")
    /\ rphase' = [rphase EXCEPT ![p] = 0]
    /\ Advance(p)
    /\ UNCHANGED <<srcVars, snapLock, snapMod, readLock, readMod, res, editErr, histVars>>

\* VendorModules: LoadDeclaredFromLock fails for a requirement missing from the
\* lock; each module's copy is removed, then copied from the cache and verified
\* against the hash read from the lock.
VendorRemove(p) ==
    /\ At(p, "VendorRemove")
    /\ LET j == ProjOf(p)
           k == Step(p).key
       IN
       /\ IF ~(readMod[p] \subseteq DOMAIN readLock[p])
          THEN Abort(p) /\ UNCHANGED vendor
          ELSE vendor' = [vendor EXCEPT ![j][k] = "absent"] /\ Advance(p)
       /\ UNCHANGED <<fileVars, worktree, cache, localVars, histVars>>

VendorCopy(p) ==
    /\ At(p, "VendorCopy")
    /\ LET j == ProjOf(p)
           k == Step(p).key
           e == readLock[p][k]
           src == cache[k][e.ver]
       IN
       /\ IF src = "absent"
          THEN Abort(p) /\ UNCHANGED <<vendor, staleVendor>>
          ELSE /\ vendor' = [vendor EXCEPT ![j][k] = src]
               /\ IF src /= e.hash THEN Abort(p) /\ UNCHANGED staleVendor
                  ELSE /\ Advance(p)
                       \* The copy verified against the hash this process read; it
                       \* must also be what the lock names now.
                       /\ staleVendor' = IF k \notin LockKeys(j) \/ src /= CommitContent[lock[j][k].commit]
                                         THEN staleVendor \cup {p} ELSE staleVendor
       /\ UNCHANGED <<fileVars, worktree, cache, localVars, torn>>

Acquire(p) ==
    /\ At(p, "Acquire")
    /\ LET l == Step(p).lk IN
       /\ \A q \in Procs : l \notin holds[q]
       /\ holds' = [holds EXCEPT ![p] = @ \cup {l}]
       /\ pc' = [pc EXCEPT ![p] = @ + 1]
       /\ UNCHANGED <<result, fileVars, srcVars, localVars, histVars>>

Release(p) ==
    /\ At(p, "Release")
    /\ holds' = [holds EXCEPT ![p] = @ \ {Step(p).lk}]
    /\ pc' = [pc EXCEPT ![p] = @ + 1]
    /\ UNCHANGED <<result, fileVars, srcVars, localVars, histVars>>

Return(p) ==
    /\ At(p, "Return")
    /\ result' = [result EXCEPT ![p] = IF editErr[p] THEN "rolled_back" ELSE "ok"]
    /\ pc' = [pc EXCEPT ![p] = @ + 1]
    /\ UNCHANGED <<holds, fileVars, srcVars, localVars, histVars>>

Quiescent == \A p \in Procs : ~Running(p)
\* Explicit terminal stuttering only, so TLC's deadlock check stays meaningful.
Terminated == Quiescent /\ UNCHANGED vars

Next ==
    \/ \E p \in Procs :
          \/ Snapshot(p) \/ ReadMod(p) \/ ReadLock(p) \/ Fetch(p) \/ Checkout(p)
          \/ CopyToCache(p) \/ WriteLock(p) \/ WriteMod(p)
          \/ RestoreTruncate(p) \/ RestoreWrite(p) \/ RestoreRemove(p)
          \/ VendorRemove(p) \/ VendorCopy(p) \/ Acquire(p) \/ Release(p) \/ Return(p)
    \/ Terminated

Spec == Init /\ [][Next]_vars

(* --- Properties (design D3) --- *)
TypeOK ==
    /\ \A j \in Projects : mod[j] \subseteq Keys /\ IsLock(lock[j])
    /\ modVis \in [Projects -> Visibility] /\ lockVis \in [Projects -> Visibility]
    /\ worktree \in [Keys -> {"none"} \cup Versions]
    /\ cache \in [Keys -> [Versions -> {"absent"} \cup Contents]]
    /\ vendor \in [Projects -> [Keys -> {"absent"} \cup Contents]]
    /\ pc \in [Procs -> Nat] /\ result \in [Procs -> Results]
    /\ holds \in [Procs -> SUBSET LockNames]
    /\ \A p \in Procs : IsLock(snapLock[p].val) /\ snapMod[p].val \subseteq Keys
                        /\ IsLock(readLock[p]) /\ readMod[p] \subseteq Keys /\ IsLock(res[p])
    /\ editErr \in [Procs -> BOOLEAN] /\ rphase \in [Procs -> {0, 1}]
    /\ torn \subseteq Procs /\ staleVendor \subseteq Procs

Opposite(p, q) == ProjOf(p) = ProjOf(q) /\ Plan(p).key = Plan(q).key
                  /\ {Plan(p).kind, Plan(q).kind} = {"add", "remove"}

\* The effect of every successful add or remove survives in both files.
NoLostUpdate ==
    Quiescent =>
        \A p \in Procs :
            (result[p] = "ok" /\ Plan(p).kind \in {"add", "remove"} /\ ~\E q \in Procs \ {p} : Opposite(p, q))
            => LET j == ProjOf(p)
                   k == Plan(p).key
               IN IF Plan(p).kind = "add" THEN k \in mod[j] /\ k \in LockKeys(j)
                  ELSE k \notin mod[j] /\ k \notin LockKeys(j)

Mutating == {"add", "remove", "sync", "update"}
LockMatchesRequires ==
    Quiescent =>
        \A j \in Projects :
            (\A p \in Procs : (ProjOf(p) = j /\ Plan(p).kind \in Mutating) => result[p] = "ok")
            => LockKeys(j) = mod[j]

ReadersSeeWholeFiles == torn = {}

LockHashMatchesCommit ==
    \A j \in Projects : \A k \in LockKeys(j) : lock[j][k].hash = CommitContent[lock[j][k].commit]

\* "Final" is the lock when the vendor finishes its verified copy, the last
\* step it performs on the files: a later sequential update that leaves
\* invowk_modules/ behind is ordinary and fixed by re-vendoring.
VendorMatchesFinalLock == staleVendor = {}

WaitsFor(p) == IF At(p, "Acquire") THEN {Step(p).lk} ELSE {}
NoCircularWait ==
    ~\E p, q \in Procs : p /= q /\ \E l1 \in WaitsFor(p), l2 \in WaitsFor(q) : l1 \in holds[q] /\ l2 \in holds[p]

(* --- Witnesses (design D4): each ~W must be violated --- *)
\* One process finishes while the other is inside its fetch window.
WitnessNoFetchOverlap == ~\E p, q \in Procs : p /= q /\ At(p, "Fetch") /\ result[q] = "ok"
WitnessNoRollback == \A p \in Procs : result[p] /= "rolled_back"
WitnessNoPartialFile == \A j \in Projects : lockVis[j] /= "partial" /\ modVis[j] /= "partial"
WitnessNoVendorOverlap ==
    ~\A p \in Procs : At(p, "VendorCopy") /\ vendor[ProjOf(p)][Step(p).key] = "absent"
\* Every process ends with its intended outcome: the fix neither blocks nor
\* fails every behaviour. rollback_read's adder is designed to roll back.
WitnessNotAllOk == ~(Quiescent /\ \A p \in Procs : result[p] = Intended(p))
=============================================================================
