\* SPDX-License-Identifier: MPL-2.0
---------------------------- MODULE ServerbaseTrace ----------------------------
(*
  Trace validation for Serverbase. Each trace is recorded from the real Base
  running operations one at a time; a record is the projection after each
  operation. Every point where all callers are idle or done must match the
  next record, so a trace cannot skip an observable state. A trace is
  ACCEPTED when TLC violates NotFullyConsumed (some behaviour consumes it).
*)
EXTENDS Serverbase, ServerbaseTraces, Sequences, Naturals, FiniteSets

CONSTANTS TraceSet, TraceIndex

VARIABLE i

T == IF TraceSet = "accepted" THEN Accepted[TraceIndex] ELSE Rejected[TraceIndex]

Proj == [state |-> state, started |-> startedCloses > 0, errClosed |-> errClosed]

Settled == \A p \in Procs : pc[p] \in {"begin", "done"}

TraceInit == Init /\ i = 1 /\ Proj = T[1]

\* The harness runs operations one at a time, so at most one caller is ever
\* mid-operation; without this, two concurrent operations could merge into
\* one record and hide a skipped state. A step that settles every caller must
\* reach the next record.
TraceNext == Next /\ Cardinality({p \in Procs : pc'[p] \notin {"begin", "done"}}) <= 1 /\
    IF Settled' /\ pc' /= pc
    THEN i < Len(T) /\ Proj' = T[i + 1] /\ i' = i + 1
    ELSE i' = i

TraceSpec == TraceInit /\ [][TraceNext]_<<vars, i>>

NotFullyConsumed == i < Len(T)
=============================================================================
