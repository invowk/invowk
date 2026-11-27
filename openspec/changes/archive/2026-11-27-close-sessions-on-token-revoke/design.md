## Decisions

- **Close the connection, not the session.** SSH authenticates once per connection, and an authenticated client can open more channels without logging in again. Closing only the sessions would leave the gap open.
- **Bind at login under the token lock.** `admitConn` checks the token and registers the connection in one critical section, and revocation deletes and detaches in one critical section. So the interleavings the model treats as atomic stay atomic.
- **ConnCallback wrapper.** The password callback has no access to the gossh connection, but `ConnCallback` shares its `ssh.Context`. The wrapper is stored there, and its `Close` unregisters itself.
- **Expiry does not close.** Expiry bounds new logins, while execution end bounds sessions. A connection's command ID lets `RevokeTokensForCommand` close it even after the expiry sweep dropped its token.

## Risks / Trade-offs

- Callback processes still running at execution end are terminated. Previously they continued, which was the defect.
