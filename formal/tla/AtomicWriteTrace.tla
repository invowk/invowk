\* SPDX-License-Identifier: MPL-2.0
--------------------------- MODULE AtomicWriteTrace ---------------------------
(*
  Trace validation for AtomicWrite. Each trace is recorded from the real
  atomicWriteFile on the real filesystem; a record is written whenever the
  observable projection changes. Every model step that changes the projection
  must produce the next record, so no observable state can be skipped. A
  trace is ACCEPTED when TLC violates NotFullyConsumed (see TraceBase).
*)
EXTENDS AtomicWrite, AtomicWriteTraces

CONSTANTS TraceSet, TraceIndex

VARIABLE i

INSTANCE TraceBase

Proj == [tmp |-> tmpVisible, replaced |-> targetInode = "tmp", ok |-> returnedOk, err |-> returnedErr]

TraceInit == Init /\ TraceStart(Proj)

TraceNext == Next /\ pc' /= "crashed" /\ TraceAdvanceOnChange(Proj, Proj')

TraceSpec == TraceInit /\ [][TraceNext]_<<vars, i>>
=============================================================================
