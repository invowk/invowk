\* SPDX-License-Identifier: MPL-2.0
--------------------------- MODULE HostCallbackToken ---------------------------
(*
  Lifetime of SSH host-callback bearer tokens (internal/sshserver) across
  container executions sharing one server.

  Each execution generates a token, runs, and ends on success, error, or
  cancellation; the container runtime revokes the token on every path
  (a deferred cleanup-on-error plus the caller's deferred prep.cleanup()).
  A process inside the container may authenticate with the token and open a
  session, which lasts until the process ends it or the server stops. Tokens
  also expire by TTL, which blocks new logins but closes nothing. Revocation
  also closes the connections the token authenticated (the fix for finding
  F7); login and revocation are atomic with respect to each other.

  Stopping the server closes its listener but no open connection
  (ssh.Server.Shutdown waits for them and returns at its deadline), so a
  session opened before Stop stays open: finding F11, with StopClosesSessions
  as the fix configuration. Production stops the server after executions
  end, when revocation has already closed their connections, so F11 matters
  only for executions still running at Stop.
*)
\* Correspondence (checked by `scripts/formal.py correspondence`):
\*
\* | model element | Go symbol | file | binding | abstraction |
\* |---|---|---|---|---|
\* | Generate | Server.GenerateToken | internal/sshserver/server_auth.go | TestHostCallbackToken_LifecycleMatchesModel | token values abstracted to one per execution |
\* | Auth | Server.ValidateToken | internal/sshserver/server_auth.go | TestHostCallbackToken_LifecycleMatchesModel | TTL expiry is a nondeterministic action |
\* | Start/Auth/End/EndSession/Expire/Stop | Server.GetConnectionInfo | internal/sshserver/server_auth.go | TestHostCallbackToken_TraceHarness | two executions; real SSH clients over loopback on a fake clock |
\* | Revoke | Server.RevokeToken | internal/sshserver/server_auth.go | TestRevokeTokenClosesAuthenticatedConnection | sessions abstracted to the connection that carries them |
\* | Auth/End atomicity | Server.admitConn | internal/sshserver/server_conns.go | TestRevocationRacesAuthenticationSafely | login and revocation are single atomic actions |
\* | End paths | ContainerRuntime.prepareContainerExecution | internal/runtime/container_exec.go | - | success, error, and cancel all run the deferred revoke |
\* | Stop | Server.Stop | internal/sshserver/server_lifecycle.go | TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen | characterisation: closes the listener; open connections stay open (F11) |
EXTENDS Naturals

CONSTANTS Execs, Mutant, RevokeClosesSessions, StopClosesSessions

VARIABLES
    exec,          \* per execution: "idle" | "running" | "ended"
    token,         \* per execution: "none" | "valid" | "revoked" | "expired"
    session,       \* per execution: a session authenticated with its token is open
    server,        \* "running" | "stopped"
    authAfterEnd,  \* some login succeeded after its execution ended
    authEver       \* some login succeeded

vars == <<exec, token, session, server, authAfterEnd, authEver>>

Init ==
    /\ exec = [e \in Execs |-> "idle"]
    /\ token = [e \in Execs |-> "none"]
    /\ session = [e \in Execs |-> FALSE]
    /\ server = "running"
    /\ authAfterEnd = FALSE /\ authEver = FALSE

Start(e) == server = "running" /\ exec[e] = "idle"
    /\ exec' = [exec EXCEPT ![e] = "running"] /\ token' = [token EXCEPT ![e] = "valid"]
    /\ UNCHANGED <<session, server, authAfterEnd, authEver>>

\* Success, error, and cancellation paths. The deferred cleanup revokes the
\* token on each; the mutant forgets it on the success path.
End(e) == exec[e] = "running"
    /\ \E path \in {"success", "error", "cancel"} :
         /\ exec' = [exec EXCEPT ![e] = "ended"]
         /\ token' = [token EXCEPT ![e] =
                IF path = "success" /\ Mutant = "no_revoke_on_success" THEN token[e]
                ELSE IF token[e] = "valid" THEN "revoked" ELSE token[e]]
         /\ session' = [session EXCEPT ![e] = session[e] /\ ~(RevokeClosesSessions
                /\ ~(path = "error" /\ Mutant = "keep_sessions_on_error"))]
    /\ UNCHANGED <<server, authAfterEnd, authEver>>

Expire(e) == token[e] = "valid" /\ token' = [token EXCEPT ![e] = "expired"]
    /\ UNCHANGED <<exec, session, server, authAfterEnd, authEver>>

\* A process in the container logs in with the execution's token.
Auth(e) == (server = "running" \/ Mutant = "listener_left_open") /\ token[e] = "valid"
    /\ session' = [session EXCEPT ![e] = TRUE]
    /\ authEver' = TRUE
    /\ authAfterEnd' = (authAfterEnd \/ exec[e] = "ended")
    /\ UNCHANGED <<exec, token, server>>

EndSession(e) == session[e] /\ session' = [session EXCEPT ![e] = FALSE]
    /\ UNCHANGED <<exec, token, server, authAfterEnd, authEver>>

Stop == server = "running" /\ server' = "stopped"
    /\ session' = (IF StopClosesSessions /\ Mutant /= "listener_left_open" THEN [e \in Execs |-> FALSE] ELSE session)
    /\ UNCHANGED <<exec, token, authAfterEnd, authEver>>

Idle == UNCHANGED vars \* explicit stuttering

Next == Stop \/ Idle \/ \E e \in Execs : Start(e) \/ End(e) \/ Expire(e) \/ Auth(e) \/ EndSession(e)
Spec == Init /\ [][Next]_vars

TypeOK == server \in {"running", "stopped"}
\* No login succeeds once the execution that owns the token has ended.
NoAuthAfterExecution == ~authAfterEnd
\* No session is open once the server stopped: none survives Stop, and no login
\* succeeds afterwards even with tokens left in the map.
NoSessionAfterStop == server = "stopped" => \A e \in Execs : ~session[e]
\* F7: a session never outlives the execution whose token opened it.
NoSessionAfterExecution == \A e \in Execs : session[e] => exec[e] /= "ended"

WitnessNoAuth == ~authEver
WitnessNoOverlap == ~(\A e \in Execs : exec[e] = "running")
=============================================================================
