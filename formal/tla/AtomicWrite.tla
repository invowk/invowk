\* SPDX-License-Identifier: MPL-2.0
------------------------------ MODULE AtomicWrite ------------------------------
(*
  pkg/fspath.AtomicWriteFile (used by LockFile.Save) over an abstract
  filesystem that separates what processes see from what survives power loss.

  Named axioms:
    A1  Writes change the visible inode data, not its durable data.
    A2  fsync(file) makes the file's current data durable.
    A3  rename is atomic in the visible namespace.
    A4  A directory entry change becomes durable only after fsync(dir).
    A5  Power loss replaces the visible state with a durable state; unsynced
        data may be lost (empty) or may have reached the disk.
    A6  (filesystem assumption, not POSIX) rename is atomic across a crash:
        the durable entry names either the old or the new inode, never neither.

  SyncFile and SyncDir parameterise the two fsync calls the code does not
  make today (finding F3).
*)
\* Correspondence (checked by `scripts/formal.py correspondence`):
\*
\* | model element | Go symbol | file | binding | abstraction |
\* |---|---|---|---|---|
\* | steps | atomicWriteFile | pkg/fspath/atomic.go | TestAtomicWriteFile_FailureAtEveryStep | chmod folded into create; one writer |
\* | cleanup | atomicWriteOps | pkg/fspath/atomic.go | TestAtomicWriteFile_FailureAtEveryStep | temp files left by a process crash are an expected abstraction |
\* | caller | LockFile.Save | pkg/invowkmod/lockfile.go | - | concurrent multi-process writers not modelled |
EXTENDS Naturals

CONSTANTS SyncFile, SyncDir, Mutant

VARIABLES
    pc,             \* writer step
    tmpVisible,     \* the temp file's directory entry is visible
    tmpData,        \* visible data of the temp inode: "empty" | "old" | "partial" | "new"
    fileSynced,     \* A2 applied to the temp inode after the write finished
    targetInode,    \* visible entry for the target: "old" | "tmp"
    dirSynced,      \* A4 applied after rename
    crashContent,   \* target content after power loss, or "none"
    returnedOk,     \* AtomicWriteFile returned nil before any crash
    returnedErr     \* AtomicWriteFile returned an error

vars == <<pc, tmpVisible, tmpData, fileSynced, targetInode, dirSynced, crashContent, returnedOk, returnedErr>>

\* The in-place mutant writes straight into the target, without a temp file.
Init ==
    /\ pc = "create"
    /\ tmpVisible = FALSE /\ fileSynced = FALSE
    \* In place, the "temp" inode is the target itself, starting with the old data.
    /\ tmpData = (IF Mutant = "in_place" THEN "old" ELSE "empty")
    /\ targetInode = (IF Mutant = "in_place" THEN "tmp" ELSE "old")
    /\ dirSynced = FALSE
    /\ crashContent = "none" /\ returnedOk = FALSE /\ returnedErr = FALSE

VisibleContent == IF targetInode = "old" THEN "old" ELSE tmpData
Running == pc \notin {"done", "failed", "crashed"}

Step(from, to) == pc = from /\ pc' = to
Keep(vs) == UNCHANGED vs

Create == Step("create", "write1") /\ tmpVisible' = (Mutant /= "in_place")
    /\ Keep(<<tmpData, fileSynced, targetInode, dirSynced, crashContent, returnedOk, returnedErr>>)
Write1 == Step("write1", "write2") /\ tmpData' = "partial"
    /\ Keep(<<tmpVisible, fileSynced, targetInode, dirSynced, crashContent, returnedOk, returnedErr>>)
Write2 == Step("write2", "sync") /\ tmpData' = "new"
    /\ Keep(<<tmpVisible, fileSynced, targetInode, dirSynced, crashContent, returnedOk, returnedErr>>)
FsyncFile == Step("sync", "rename") /\ fileSynced' = SyncFile
    /\ Keep(<<tmpVisible, tmpData, targetInode, dirSynced, crashContent, returnedOk, returnedErr>>)
Rename == Step("rename", "syncdir") /\ targetInode' = "tmp" /\ tmpVisible' = FALSE
    /\ Keep(<<tmpData, fileSynced, dirSynced, crashContent, returnedOk, returnedErr>>)
FsyncDir == Step("syncdir", "done") /\ dirSynced' = SyncDir /\ returnedOk' = TRUE
    /\ Keep(<<tmpVisible, tmpData, fileSynced, targetInode, crashContent, returnedErr>>)

\* Any step up to rename may fail (CreateTemp itself included); the deferred
\* cleanup removes the temp file if one was created.
Fail == pc \in {"create", "write1", "write2", "sync", "rename"}
    /\ pc' = "failed" /\ returnedErr' = TRUE
    /\ tmpVisible' = (tmpVisible /\ Mutant = "skip_cleanup")
    /\ Keep(<<tmpData, fileSynced, targetInode, dirSynced, crashContent, returnedOk>>)

\* A5 + A6: the surviving entry and data are nondeterministic unless synced.
PowerLoss == pc /= "crashed"
    /\ \E durTarget \in (IF dirSynced THEN {targetInode} ELSE {"old", targetInode}),
         durData \in (IF fileSynced THEN {"new"} ELSE {"empty", tmpData}) :
         crashContent' = (IF durTarget = "old" THEN "old" ELSE durData)
    /\ pc' = "crashed"
    /\ Keep(<<tmpVisible, tmpData, fileSynced, targetInode, dirSynced, returnedOk, returnedErr>>)

Finished == pc \in {"done", "failed", "crashed"} /\ UNCHANGED vars

Next == Create \/ Write1 \/ Write2 \/ FsyncFile \/ Rename \/ FsyncDir \/ Fail \/ PowerLoss \/ Finished
Spec == Init /\ [][Next]_vars

TypeOK == crashContent \in {"none", "old", "new", "empty", "partial"}
\* A3: without power loss, readers see the complete old or the complete new content.
ReadersNeverSeePartial == pc /= "crashed" => VisibleContent \in {"old", "new"}
\* D-Atomic: after power loss the target holds complete old or complete new content.
DurableAtomic == crashContent \in {"none", "old", "new"}
\* D-Commit: once the write returned, power loss keeps the new content.
DurableCommit == (crashContent /= "none" /\ returnedOk) => crashContent = "new"
\* A returned error leaves no temp file behind.
TempRemovedOnError == returnedErr => ~tmpVisible

WitnessNoCrashAfterReturn == ~(crashContent /= "none" /\ returnedOk)
WitnessNoFailure == ~returnedErr
=============================================================================
