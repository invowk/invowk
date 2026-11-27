## ADDED Requirements

### Requirement: Revocation closes authenticated connections
Revoking a host-callback token SHALL close every SSH connection that authenticated with it, and a login racing the revocation SHALL either fail or have its connection closed.

#### Scenario: Execution ends with a session open
- **WHEN** a process authenticated with the execution's token and the execution revokes it
- **THEN** the connection SHALL close, and no new session SHALL open on it

#### Scenario: Token expired before revocation
- **WHEN** a token expires while its connection is open and the execution later revokes by command
- **THEN** that connection SHALL close

#### Scenario: Expiry alone
- **WHEN** a token's TTL passes
- **THEN** new logins with it SHALL fail and its open connections SHALL stay open

#### Scenario: Model check
- **WHEN** `make formal` runs
- **THEN** `HostCallbackToken.sessionLifetimeWithRevokeClosing` SHALL pass, and `mutantPreFixF7RevokeKeepsSessions` and `mutantKeepSessionsOnError` SHALL produce counterexamples
