\* SPDX-License-Identifier: MPL-2.0
------------------------------- MODULE WatchTrace -------------------------------
(*
  Trace validation for Watch. Traces are recorded from the real Watcher.Run
  driven by a fake backend and a fake timer checked against time.AfterFunc.
  Once the loop leaves its select the timer reads "none"; `returned` becomes
  TRUE only when Run has returned, after in-flight work drained. Every model step that
  changes the projection must produce the next record.
*)
EXTENDS Watch, WatchTraces

CONSTANTS TraceSet, TraceIndex

VARIABLE i

INSTANCE TraceBase

Proj == [delivered |-> delivered, callbacks |-> callbacks,
         timer |-> IF loop = "select" THEN timer ELSE "none", returned |-> loop = "returned"]

TraceInit == Init /\ TraceStart(Proj)

TraceNext == Next /\ TraceAdvanceOnChange(Proj, Proj')

TraceSpec == TraceInit /\ [][TraceNext]_<<vars, i>>
=============================================================================
