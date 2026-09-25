\* SPDX-License-Identifier: MPL-2.0
------------------------------- MODULE WatchTrace -------------------------------
(*
  Trace validation for Watch. Traces are recorded from the real Watcher.Run
  driven by a fake backend and a fake timer checked against time.AfterFunc.
  Once the loop leaves its select the timer reads "none"; `returned` becomes
  TRUE only when Run has returned, after in-flight work drained. Every model step that
  changes the projection must produce the next record.
*)
EXTENDS Watch, WatchTraces, Sequences, Naturals

CONSTANTS TraceSet, TraceIndex

VARIABLE i

T == IF TraceSet = "accepted" THEN Accepted[TraceIndex] ELSE Rejected[TraceIndex]

Proj == [delivered |-> delivered, callbacks |-> callbacks,
         timer |-> IF loop = "select" THEN timer ELSE "none", returned |-> loop = "returned"]

TraceInit == Init /\ i = 1 /\ Proj = T[1]

TraceNext == Next /\
    IF Proj' = Proj THEN i' = i
    ELSE i < Len(T) /\ Proj' = T[i + 1] /\ i' = i + 1

TraceSpec == TraceInit /\ [][TraceNext]_<<vars, i>>

NotFullyConsumed == i < Len(T)
=============================================================================
