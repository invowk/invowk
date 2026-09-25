\* SPDX-License-Identifier: MPL-2.0
--------------------------- MODULE LockIntegrityTrace ---------------------------
(*
  Trace validation for LockIntegrity. Traces are recorded from the real
  Resolver.Sync, LoadDeclaredFromLock followed by VendorModules, the
  discovery-time check of C's vendored copy, and command-scope admission
  through a sibling's copy, over local fixture trees served by commit. The
  projection carries every model variable, read from the lock file and the
  directories on disk (content hashes mapped to Good, Evil, and Other) and
  from the call results. Every model step that changes the projection must
  produce the next record.
*)
EXTENDS LockIntegrity, LockIntegrityTraces

CONSTANTS TraceSet, TraceIndex

VARIABLE i

INSTANCE TraceBase

Proj == [remoteCommit |-> remoteCommit, lockVer |-> lockVer, lockCommit |-> lockCommit,
         lockHash |-> lockHash, cache |-> cache, vendored |-> vendored, pVendored |-> pVendored,
         loaded |-> loaded, called |-> called, syncs |-> syncs]

TraceInit == Init /\ TraceStart(Proj)

TraceNext == Next /\ TraceAdvanceOnChange(Proj, Proj')

TraceSpec == TraceInit /\ [][TraceNext]_<<vars, i>>
=================================================================================
