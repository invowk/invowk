\* SPDX-License-Identifier: MPL-2.0
--------------------------------- MODULE Watch ---------------------------------
(*
  internal/watch.Watcher.Run: filesystem events, the debounce timer with the
  Reset/Stop return values of time.AfterFunc, fire goroutines tracked by a
  wait group, the skip-if-busy re-arm, callback errors, cancellation, and
  fatal backend errors.

  Weak fairness applies only to system actions (the loop taking an event, the
  timer firing, fire-goroutine steps, the callback returning, and shutdown).
  Environment actions (events, cancellation, fatal errors, callback errors)
  are never fair, so liveness cannot hold merely because the environment
  eventually stops the loop.
*)
\* Correspondence (checked by `scripts/formal.py correspondence`):
\*
\* | model element | Go symbol | file | binding | abstraction |
\* |---|---|---|---|---|
\* | loop | Watcher.Run | internal/watch/watcher.go | TestWatch_DeliversEveryBurstThroughSkipIfBusy | path filtering and ignores abstracted away |
\* | timer | debounceTimer | internal/watch/watcher.go | TestWatch_ManualTimerMatchesAfterFuncContract | durations abstracted; only Reset/Stop results matter |
\* | fire goroutine | Watcher.Run | internal/watch/watcher.go | TestWatch_DeliversEveryBurstThroughSkipIfBusy | fireBody and fireWrapped merged |
\* | backend | watcherBackend | internal/watch/watcher.go | - | closed channels modelled as fatal errors |
EXTENDS Naturals, FiniteSets

CONSTANTS Events, Goroutines, Mutant

VARIABLES
    arrived,      \* events the backend has produced
    queued,       \* produced but not yet taken by the loop
    pending,      \* the loop's pending set
    delivered,    \* events passed to a completed callback
    timer,        \* "none" | "armed" | "expired"
    fires,        \* fire goroutine ids in flight
    gpc,          \* per goroutine: "idle" | "start" | "callback"
    gset,         \* per goroutine: events taken for its callback
    wg,           \* WaitGroup counter
    running,      \* callback guard (atomic.Bool)
    callbacks,    \* callbacks currently executing
    cbErr,        \* a callback error is buffered in callbackErr
    cancelled,    \* ctx cancelled
    loop          \* "select" | "exiting" | "returned"

vars == <<arrived, queued, pending, delivered, timer, fires, gpc, gset, wg, running, callbacks, cbErr, cancelled, loop>>

Init ==
    /\ arrived = {} /\ queued = {} /\ pending = {} /\ delivered = {}
    /\ timer = "none" /\ fires = {}
    /\ gpc = [g \in Goroutines |-> "idle"] /\ gset = [g \in Goroutines |-> {}]
    /\ wg = 0 /\ running = FALSE /\ callbacks = 0 /\ cbErr = FALSE
    /\ cancelled = FALSE /\ loop = "select"

Selecting == loop = "select"

(* --- environment --- *)
Arrive(e) == e \notin arrived /\ loop /= "returned"
    /\ arrived' = arrived \cup {e} /\ queued' = queued \cup {e}
    /\ UNCHANGED <<pending, delivered, timer, fires, gpc, gset, wg, running, callbacks, cbErr, cancelled, loop>>
Cancel == ~cancelled /\ cancelled' = TRUE
    /\ UNCHANGED <<arrived, queued, pending, delivered, timer, fires, gpc, gset, wg, running, callbacks, cbErr, loop>>
Fatal == Selecting /\ loop' = "exiting"
    /\ UNCHANGED <<arrived, queued, pending, delivered, timer, fires, gpc, gset, wg, running, callbacks, cbErr, cancelled>>

(* --- event loop select --- *)
LoopCancel == Selecting /\ cancelled /\ loop' = "exiting"
    /\ UNCHANGED <<arrived, queued, pending, delivered, timer, fires, gpc, gset, wg, running, callbacks, cbErr, cancelled>>
LoopCallbackErr == Selecting /\ cbErr /\ loop' = "exiting"
    /\ UNCHANGED <<arrived, queued, pending, delivered, timer, fires, gpc, gset, wg, running, callbacks, cbErr, cancelled>>
\* Taking an event: add to pending, then schedule or Reset the timer.
LoopEvent == Selecting /\ \E e \in queued :
    /\ queued' = queued \ {e} /\ pending' = pending \cup {e}
    /\ CASE timer = "none"    -> timer' = "armed" /\ wg' = wg + 1
         [] timer = "armed"   -> UNCHANGED <<timer, wg>>          \* Reset returns true
         [] timer = "expired" -> timer' = "armed"                  \* Reset returns false
                                 /\ wg' = (IF Mutant = "reset_skips_add" THEN wg ELSE wg + 1)
    /\ UNCHANGED <<arrived, delivered, fires, gpc, gset, running, callbacks, cbErr, cancelled, loop>>

(* --- timer: time.AfterFunc starts a goroutine when it expires --- *)
TimerFire == timer = "armed" /\ \E g \in Goroutines :
    /\ gpc[g] = "idle"
    /\ timer' = "expired" /\ fires' = fires \cup {g} /\ gpc' = [gpc EXCEPT ![g] = "start"]
    /\ UNCHANGED <<arrived, queued, pending, delivered, gset, wg, running, callbacks, cbErr, cancelled, loop>>

(* --- fire goroutine --- *)
Finish(g) == fires' = fires \ {g} /\ gpc' = [gpc EXCEPT ![g] = "idle"] /\ gset' = [gset EXCEPT ![g] = {}]

FireStart(g) == gpc[g] = "start" /\
    CASE cancelled ->
            Finish(g) /\ wg' = wg - 1
            /\ UNCHANGED <<arrived, queued, pending, delivered, timer, running, callbacks, cbErr, cancelled, loop>>
      [] running /\ Mutant /= "no_busy_guard" ->
            \* skip-if-busy: re-arm the timer unless shutdown cleared it
            /\ Finish(g)
            /\ CASE timer = "expired" -> timer' = "armed" /\ wg' = wg     \* Reset false: +1, then Done: -1
                 [] OTHER             -> UNCHANGED timer /\ wg' = wg - 1
            /\ UNCHANGED <<arrived, queued, pending, delivered, running, callbacks, cbErr, cancelled, loop>>
      [] pending = {} ->
            Finish(g) /\ wg' = wg - 1
            /\ UNCHANGED <<arrived, queued, pending, delivered, timer, running, callbacks, cbErr, cancelled, loop>>
      [] OTHER ->
            /\ running' = TRUE /\ callbacks' = callbacks + 1
            /\ gset' = [gset EXCEPT ![g] = pending] /\ pending' = {}
            /\ gpc' = [gpc EXCEPT ![g] = "callback"]
            /\ UNCHANGED <<arrived, queued, delivered, timer, fires, wg, cbErr, cancelled, loop>>

\* OnChange returns, successfully or with an error the environment chooses.
CallbackReturn(g) == gpc[g] = "callback" /\ \E failed \in BOOLEAN :
    /\ delivered' = delivered \cup gset[g]
    /\ cbErr' = (cbErr \/ failed)
    /\ running' = FALSE /\ callbacks' = callbacks - 1
    /\ Finish(g) /\ wg' = wg - 1
    /\ UNCHANGED <<arrived, queued, pending, timer, cancelled, loop>>

(* --- Run's deferred shutdown: stop the timer, compensate, wait, return --- *)
Shutdown == loop = "exiting"
    /\ timer' = "none"
    /\ wg' = (IF timer = "armed" THEN wg - 1 ELSE wg)   \* Stop returns true: no goroutine will run
    /\ loop' = "draining"
    /\ UNCHANGED <<arrived, queued, pending, delivered, fires, gpc, gset, running, callbacks, cbErr, cancelled>>
Return == loop = "draining" /\ (wg = 0 \/ Mutant = "no_wait") /\ loop' = "returned"
    /\ UNCHANGED <<arrived, queued, pending, delivered, timer, fires, gpc, gset, wg, running, callbacks, cbErr, cancelled>>

Finished == loop = "returned" /\ fires = {} /\ UNCHANGED vars
Quiet == loop = "select" /\ arrived = Events /\ queued = {} /\ timer /= "armed" /\ fires = {} /\ cancelled
    /\ UNCHANGED vars

System == LoopCancel \/ LoopCallbackErr \/ LoopEvent \/ TimerFire \/ Shutdown \/ Return
    \/ \E g \in Goroutines : FireStart(g) \/ CallbackReturn(g)
Environment == Cancel \/ Fatal \/ \E e \in Events : Arrive(e)

Next == System \/ Environment \/ Finished \/ Quiet

Fairness == WF_vars(LoopEvent) /\ WF_vars(TimerFire) /\ WF_vars(Shutdown) /\ WF_vars(Return)
    /\ WF_vars(LoopCancel) /\ WF_vars(LoopCallbackErr)
    /\ \A g \in Goroutines : WF_vars(FireStart(g)) /\ WF_vars(CallbackReturn(g))

Spec == Init /\ [][Next]_vars /\ Fairness
UnfairSpec == Init /\ [][Next]_vars

(* --- safety --- *)
TypeOK == loop \in {"select", "exiting", "draining", "returned"} /\ wg \in Nat
NoOverlappingCallbacks == callbacks <= 1
\* Every WaitGroup count belongs to an armed timer or a live fire goroutine.
WaitGroupBalanced == wg = (IF timer = "armed" THEN 1 ELSE 0) + Cardinality(fires)
NothingAfterReturn == loop = "returned" => (fires = {} /\ callbacks = 0)

(* --- liveness --- *)
Terminated == loop /= "select"
NoLostBurst == \A e \in Events : (e \in arrived) ~> (e \in delivered \/ Terminated)

(* --- witnesses --- *)
WitnessNoBusySkip == ~(\E g \in Goroutines : gpc[g] = "start" /\ running)
WitnessNoDelivery == delivered = {}
=============================================================================
