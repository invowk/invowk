\* SPDX-License-Identifier: MPL-2.0
------------------------------ MODULE Serverbase ------------------------------
(*
  Lifecycle of internal/core/serverbase.Base under concurrent callers.

  Each exported transition is split into the atomic steps the Go code
  performs: lock-free compare-and-swap or Store on `state`, critical sections
  under `stateMu`, and the `cancel()` call made after unlocking. Every caller
  runs one transition, chosen nondeterministically, so TLC explores every
  interleaving of two concurrent transitions.

  See the correspondence table below.
*)
\* Correspondence (checked by `scripts/formal.py correspondence`):
\*
\* | model element | Go symbol | file | binding | abstraction |
\* |---|---|---|---|---|
\* | Starting steps | Base.TransitionToStarting | internal/core/serverbase/base.go | TestServerbase_SequentialMatchesModel | parent-context cancellation is a nondeterministic boolean |
\* | Running steps | Base.TransitionToRunning | internal/core/serverbase/base.go | TestServerbase_SequentialMatchesModel | - |
\* | Failed steps | Base.TransitionToFailed | internal/core/serverbase/base.go | TestServerbase_SequentialMatchesModel | error values abstracted |
\* | Stopping steps | Base.TransitionToStopping | internal/core/serverbase/base.go | TestServerbase_SequentialMatchesModel | CAS retry loop modelled as a back edge |
\* | Stopped steps | Base.TransitionToStopped | internal/core/serverbase/base.go | TestServerbase_SequentialMatchesModel | - |
\* | SendError steps | Base.SendError | internal/core/serverbase/base.go | TestServerbase_ConcurrentSafetyInvariants | channel capacity abstracted |
\* | errCloseCount | Base.closeErrChannelLocked | internal/core/serverbase/base.go | TestServerbase_ConcurrentSafetyInvariants | - |
\* | State | State | internal/core/serverbase/state.go | - | - |
EXTENDS Naturals, FiniteSets

CONSTANTS Procs, Mutant, FixStartOrdering, FixTerminalCAS

States == {"Created", "Starting", "Running", "Stopping", "Stopped", "Failed"}
Terminal == {"Stopped", "Failed"}
Ops == {"Start", "Run", "Fail", "Stop", "Stopped", "Send"}

VARIABLES
    state,          \* Base.state (atomic.Int32)
    mu,             \* holder of Base.stateMu, or "none"
    ctxCreated,     \* TransitionToStarting stored ctx/cancel
    ctxCancelled,   \* the stored cancel func has been called
    errClosed,      \* Base.errClosed
    errCloseCount,  \* times close(errCh) ran
    sentAfterClose, \* an error was sent on a closed channel
    startedCloses,  \* times close(startedCh) ran
    startedByWinner,\* every close(startedCh) followed a winning Starting->Running CAS
    stopWon,        \* TransitionToStopping calls that returned true
    firstTerminal,  \* first terminal state reached, or "none"
    op, pc, seen, cancelRead

vars == <<state, mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
          startedCloses, startedByWinner, stopWon, firstTerminal, op, pc, seen, cancelRead>>

\* Record the first terminal state ever reached, so absorption can be checked.
Mark(s) == IF firstTerminal = "none" /\ s \in Terminal THEN s ELSE firstTerminal

Init ==
    /\ state = "Created"
    /\ mu = "none"
    /\ ctxCreated = FALSE /\ ctxCancelled = FALSE
    /\ errClosed = FALSE /\ errCloseCount = 0 /\ sentAfterClose = FALSE
    /\ startedCloses = 0 /\ startedByWinner = TRUE
    /\ stopWon = 0
    /\ firstTerminal = "none"
    /\ op \in [Procs -> Ops]
    /\ pc = [p \in Procs |-> "begin"]
    /\ seen = [p \in Procs |-> "none"]
    /\ cancelRead = [p \in Procs |-> FALSE]

Goto(p, l) == pc' = [pc EXCEPT ![p] = l]
Lock(p) == mu = "none" /\ mu' = p
Unlock(p) == mu = p /\ mu' = "none"
CloseErrLocked == \* closeErrChannelLocked: sync.Once + errClosed
    IF Mutant = "close_twice" \/ ~errClosed
    THEN /\ errClosed' = TRUE /\ errCloseCount' = errCloseCount + 1
    ELSE UNCHANGED <<errClosed, errCloseCount>>

(* --- TransitionToStarting --- *)
StartBegin(p) == op[p] = "Start" /\ pc[p] = "begin" /\
    \/ /\ Goto(p, "fail_lock")   \* ctx already cancelled: TransitionToFailed
       /\ op' = [op EXCEPT ![p] = "Fail"]
       /\ UNCHANGED <<state, mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                      startedCloses, startedByWinner, stopWon, firstTerminal, seen, cancelRead>>
    \/ /\ ~FixStartOrdering
       /\ IF state = "Created"
          THEN state' = "Starting" /\ Goto(p, "start_store")
          ELSE UNCHANGED state /\ Goto(p, "done")
       /\ UNCHANGED <<mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                      startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
    \/ /\ FixStartOrdering        \* fix: CAS and ctx store in one critical section
       /\ Lock(p)
       /\ IF state = "Created"
          THEN state' = "Starting" /\ ctxCreated' = TRUE /\ Goto(p, "start_unlock")
          ELSE UNCHANGED <<state, ctxCreated>> /\ Goto(p, "start_unlock")
       /\ UNCHANGED <<ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                      startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>

StartStore(p) == pc[p] = "start_store" /\ Lock(p) /\ ctxCreated' = TRUE /\ Goto(p, "start_unlock")
    /\ UNCHANGED <<state, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
StartUnlock(p) == pc[p] = "start_unlock" /\ Unlock(p) /\ Goto(p, "done")
    /\ UNCHANGED <<state, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>

(* --- TransitionToRunning --- *)
RunCAS(p) == op[p] = "Run" /\ pc[p] = "begin" /\
    IF state = "Starting"
    THEN state' = "Running" /\ Goto(p, "run_close")
         /\ UNCHANGED <<mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                        startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
    ELSE IF Mutant = "close_started_unconditionally"
         THEN Goto(p, "run_close_loser")
              /\ UNCHANGED <<state, mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                             startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
         ELSE Goto(p, "done")
              /\ UNCHANGED <<state, mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                             startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
RunClose(p) == pc[p] = "run_close" /\ startedCloses' = startedCloses + 1 /\ Goto(p, "done")
    /\ UNCHANGED <<state, mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
RunCloseLoser(p) == pc[p] = "run_close_loser" /\ startedCloses' = startedCloses + 1
    /\ startedByWinner' = FALSE /\ Goto(p, "done")
    /\ UNCHANGED <<state, mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   stopWon, firstTerminal, op, seen, cancelRead>>

(* --- TransitionToFailed / TransitionToStopped: lock, read, store, close --- *)
TermBegin(p) == op[p] \in {"Fail", "Stopped"} /\ pc[p] \in {"begin", "fail_lock"} /\ Lock(p)
    /\ Goto(p, "term_read")
    /\ UNCHANGED <<state, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
TermRead(p) == pc[p] = "term_read" /\ seen' = [seen EXCEPT ![p] = state]
    /\ cancelRead' = [cancelRead EXCEPT ![p] = ctxCreated]
    /\ Goto(p, IF state \in Terminal /\ Mutant /= "no_terminal_check" THEN "term_unlock" ELSE "term_store")
    /\ UNCHANGED <<state, mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op>>
TermStore(p) == pc[p] = "term_store" /\
    LET target == IF op[p] = "Fail" THEN "Failed" ELSE "Stopped" IN
    \* Without the fix, the Go code Stores unconditionally. The fix
    \* compare-and-swaps from the observed state and, if a lock-free CAS changed
    \* it meanwhile, re-reads it (still holding stateMu) and tries again.
    IF FixTerminalCAS /\ state /= seen[p]
    THEN Goto(p, "term_read")
         /\ UNCHANGED <<state, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                        startedCloses, startedByWinner, stopWon, firstTerminal, mu, op, seen, cancelRead>>
    ELSE /\ state' = target /\ firstTerminal' = Mark(target) /\ CloseErrLocked
         /\ Goto(p, "term_unlock")
         /\ UNCHANGED <<mu, ctxCreated, ctxCancelled, sentAfterClose, startedCloses, startedByWinner,
                        stopWon, op, seen, cancelRead>>
TermUnlock(p) == pc[p] = "term_unlock" /\ Unlock(p)
    /\ Goto(p, IF op[p] = "Fail" /\ seen[p] \notin Terminal THEN "term_cancel" ELSE "done")
    /\ UNCHANGED <<state, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
TermCancel(p) == pc[p] = "term_cancel"
    /\ ctxCancelled' = (ctxCancelled \/ cancelRead[p]) /\ Goto(p, "done")
    /\ UNCHANGED <<state, mu, ctxCreated, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>

(* --- TransitionToStopping: lock-free CAS retry loop --- *)
StopRead(p) == op[p] = "Stop" /\ pc[p] = "begin"
    /\ seen' = [seen EXCEPT ![p] = state]
    /\ Goto(p, CASE state = "Created" -> "stop_cas_created"
                 [] state \in {"Starting", "Running"} -> "stop_cas"
                 [] OTHER -> "done")
    /\ UNCHANGED <<state, mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, cancelRead>>
StopCASCreated(p) == pc[p] = "stop_cas_created" /\
    IF state = seen[p]
    THEN state' = "Stopped" /\ firstTerminal' = Mark("Stopped") /\ Goto(p, "stop_close_lock")
         /\ UNCHANGED <<mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                        startedCloses, startedByWinner, stopWon, op, seen, cancelRead>>
    ELSE Goto(p, "begin")
         /\ UNCHANGED <<state, mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                        startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
\* With F2 fixed only one caller wins a terminal transition, so sync.Once is
\* defense in depth: the close_twice mutant models a path that closes twice
\* without it (for example CloseErrChannel plus a duplicated deferred close).
StopCloseLock(p) == pc[p] = "stop_close_lock" /\ Lock(p) /\ Goto(p, "stop_close_unlock")
    /\ (IF Mutant = "close_twice"
        THEN errClosed' = TRUE /\ errCloseCount' = errCloseCount + 2
        ELSE CloseErrLocked)
    /\ UNCHANGED <<state, ctxCreated, ctxCancelled, sentAfterClose, startedCloses, startedByWinner,
                   stopWon, firstTerminal, op, seen, cancelRead>>
StopCloseUnlock(p) == pc[p] = "stop_close_unlock" /\ Unlock(p) /\ Goto(p, "done")
    /\ UNCHANGED <<state, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
StopCAS(p) == pc[p] = "stop_cas" /\
    IF state = seen[p]
    THEN state' = "Stopping" /\ stopWon' = stopWon + 1 /\ Goto(p, "stop_read_cancel")
         /\ UNCHANGED <<mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                        startedCloses, startedByWinner, firstTerminal, op, seen, cancelRead>>
    ELSE Goto(p, "begin")
         /\ UNCHANGED <<state, mu, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                        startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
StopReadCancel(p) == pc[p] = "stop_read_cancel" /\ Lock(p)
    /\ cancelRead' = [cancelRead EXCEPT ![p] = ctxCreated] /\ Goto(p, "stop_unlock")
    /\ UNCHANGED <<state, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, seen>>
StopUnlock(p) == pc[p] = "stop_unlock" /\ Unlock(p) /\ Goto(p, "stop_cancel")
    /\ UNCHANGED <<state, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
StopCancel(p) == pc[p] = "stop_cancel"
    /\ ctxCancelled' = (ctxCancelled \/ (cancelRead[p] /\ Mutant /= "stop_skips_cancel")) /\ Goto(p, "done")
    /\ UNCHANGED <<state, mu, ctxCreated, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>

(* --- SendError: under stateMu, skipped once the channel is closed --- *)
SendError(p) == op[p] = "Send" /\ pc[p] = "begin" /\ Lock(p)
    /\ sentAfterClose' = (sentAfterClose \/ (errClosed /\ Mutant = "send_without_check"))
    /\ Goto(p, "send_unlock")
    /\ UNCHANGED <<state, ctxCreated, ctxCancelled, errClosed, errCloseCount, startedCloses,
                   startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>
SendUnlock(p) == pc[p] = "send_unlock" /\ Unlock(p) /\ Goto(p, "done")
    /\ UNCHANGED <<state, ctxCreated, ctxCancelled, errClosed, errCloseCount, sentAfterClose,
                   startedCloses, startedByWinner, stopWon, firstTerminal, op, seen, cancelRead>>

Quiescent == \A p \in Procs : pc[p] = "done"

\* Explicit terminal stuttering, so deadlock checking stays on.
Finished == Quiescent /\ UNCHANGED vars

Next ==
    \/ \E p \in Procs :
        \/ StartBegin(p) \/ StartStore(p) \/ StartUnlock(p)
        \/ RunCAS(p) \/ RunClose(p) \/ RunCloseLoser(p)
        \/ TermBegin(p) \/ TermRead(p) \/ TermStore(p) \/ TermUnlock(p) \/ TermCancel(p)
        \/ StopRead(p) \/ StopCASCreated(p) \/ StopCloseLock(p) \/ StopCloseUnlock(p)
        \/ StopCAS(p) \/ StopReadCancel(p) \/ StopUnlock(p) \/ StopCancel(p)
        \/ SendError(p) \/ SendUnlock(p)
    \/ Finished

Spec == Init /\ [][Next]_vars

(* --- Safety properties --- *)
TypeOK == state \in States /\ mu \in Procs \cup {"none"}
ErrClosedAtMostOnce == errCloseCount <= 1
NoSendAfterClose == ~sentAfterClose
StartedClosedByWinner == startedCloses <= 1 /\ startedByWinner
\* F2: once terminal, the state must never change.
TerminalAbsorbing == firstTerminal /= "none" => state = firstTerminal
\* F1: after every caller returned and a stopper won, the server context is cancelled.
StopCancelsContext == (Quiescent /\ stopWon > 0 /\ ctxCreated) => ctxCancelled

(* --- Witnesses: each must be violated, proving the situation is reachable --- *)
WitnessNoConcurrentStop == ~(\A p \in Procs : op[p] = "Stop" /\ pc[p] /= "begin")
WitnessNeverRunning == state /= "Running"
WitnessNeverStartedClosed == startedCloses = 0
=============================================================================
