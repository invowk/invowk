\* SPDX-License-Identifier: MPL-2.0
------------------------ MODULE HostCallbackTokenTrace ------------------------
(*
  Trace validation for HostCallbackToken. Traces are recorded from a started
  sshserver.Server on a fake clock, with real SSH clients logging in over
  loopback. The projection is flat: per execution its phase (the harness's
  bookkeeping of GetConnectionInfo and RevokeToken), whether the server would
  admit its token now, and whether an authenticated connection is still
  open; plus the server state. Execs holds model values, which a generated
  module cannot name, so the two executions are fixed by CHOOSE; the model is
  symmetric in Execs. Every model step that changes the projection must
  produce the next record.
*)
EXTENDS HostCallbackToken, HostCallbackTokenTraces

CONSTANTS TraceSet, TraceIndex

VARIABLE i

INSTANCE TraceBase

E1 == CHOOSE e \in Execs : TRUE
E2 == CHOOSE e \in Execs \ {E1} : TRUE

Proj == [e1_exec |-> exec[E1], e1_usable |-> token[E1] = "valid", e1_session |-> session[E1],
         e2_exec |-> exec[E2], e2_usable |-> token[E2] = "valid", e2_session |-> session[E2],
         server |-> server]

TraceInit == Init /\ TraceStart(Proj)

TraceNext == Next /\ TraceAdvanceOnChange(Proj, Proj')

TraceSpec == TraceInit /\ [][TraceNext]_<<vars, i>>
=============================================================================
