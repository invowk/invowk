## Why

Finding F7 of `adopt-formal-verification`: revoking an SSH host-callback token only blocked new logins. A connection that authenticated with the token stayed open after its execution ended. It could keep running processes and open more sessions without logging in again. The HostCallbackToken TLA+ model reproduced it.

## What Changes

- The SSH server wraps every accepted connection. A successful login binds the connection to its token, in the same critical section as the validity check.
- `RevokeToken` and `RevokeTokensForCommand` close the connections the token authenticated. That ends their sessions and, through the session context, the processes they started.
- TTL expiry still blocks only new logins. Open connections close when the owning execution revokes, including after expiry.
- Authentication fails closed on a connection the server does not track.
- The HostCallbackToken base configuration now mirrors the fix. The pre-fix behaviour becomes the regression mutant `mutantPreFixF7RevokeKeepsSessions`, and the rapid binding covers sessions.

## Capabilities

### New Capabilities

- `host-callback-session-lifetime`: no host-callback connection outlives the revocation of its token.

### Modified Capabilities

(none)

## Impact

- `internal/sshserver` (new `server_conns.go`; `server_auth.go`, `server.go`, `server_lifecycle.go`), tests; `golang.org/x/crypto` becomes a direct dependency for the SSH client test; container runtime docs (en, pt-BR) and README; `formal/`.
