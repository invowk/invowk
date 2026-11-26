# Spec Delta

## MODIFIED Requirements

### Requirement: ACP policy callbacks
Invowk SHALL mediate ACP agent callbacks through explicit internal policy interfaces rather than allowing the ACP peer to access host resources implicitly.

#### Scenario: Permission request is delegated to policy
- **WHEN** an ACP agent requests permission for a tool call
- **THEN** the ACP foundation SHALL delegate the request to the configured Invowk policy
- **THEN** the selected permission outcome SHALL be sent back to the ACP agent

#### Scenario: Filesystem callback is delegated to policy
- **WHEN** an ACP agent requests a text-file read or write
- **THEN** the ACP foundation SHALL delegate the operation to the configured Invowk filesystem policy
- **THEN** the foundation SHALL NOT perform direct filesystem access unless the configured policy performs it

#### Scenario: Unsupported terminal callback is explicit
- **WHEN** an ACP agent requests terminal functionality and the configured policy does not support terminals
- **THEN** the ACP foundation SHALL return an unsupported-operation error to the ACP agent

#### Scenario: Elicitation is not offered
- **WHEN** the ACP foundation initializes a connection, and an ACP agent later sends an elicitation request or an elicitation-complete notification
- **THEN** the foundation SHALL NOT advertise the elicitation capability, SHALL answer the elicitation request with a method-not-found error, SHALL send no reply to the notification (a notification has none), SHALL ask the user nothing, and SHALL keep the session usable
