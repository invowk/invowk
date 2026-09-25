\* SPDX-License-Identifier: MPL-2.0
------------------------------- MODULE TraceBase -------------------------------
(*
  Shared trace-validation machinery. A trace spec `<Model>Trace.tla` EXTENDS
  its model and the generated `<Model>Traces` module (which defines Accepted
  and Rejected), declares TraceSet, TraceIndex, and the cursor i, and writes
  one unnamed `INSTANCE TraceBase`. It then defines only its projection Proj,
  its step constraint, and any helpers.

  A trace is ACCEPTED when TLC violates NotFullyConsumed: some behaviour of
  the model consumes every record.

  The operators are prefixed so they cannot clash with a model's own
  operators (HostCallbackToken defines Start); scripts/formal.py rejects any
  other module that defines one of these names.
*)
EXTENDS Sequences, Naturals

CONSTANTS Accepted, Rejected, TraceSet, TraceIndex

VARIABLE i

\* The trace selected by TraceSet and TraceIndex.
TraceRecord == IF TraceSet = "accepted" THEN Accepted[TraceIndex] ELSE Rejected[TraceIndex]

\* The initial state matches the first record.
TraceStart(p) == i = 1 /\ p = TraceRecord[1]

\* Advance when the observable projection changes; stutter otherwise. Every
\* step that changes the projection must produce the next record.
TraceAdvanceOnChange(p, q) ==
    IF q = p THEN i' = i
    ELSE i < Len(TraceRecord) /\ q = TraceRecord[i + 1] /\ i' = i + 1

\* Advance exactly at settle points, where the projection must match the next
\* record; stutter otherwise.
TraceAdvanceAt(settled, q) ==
    IF settled THEN i < Len(TraceRecord) /\ q = TraceRecord[i + 1] /\ i' = i + 1
    ELSE i' = i

NotFullyConsumed == i < Len(TraceRecord)
================================================================================
