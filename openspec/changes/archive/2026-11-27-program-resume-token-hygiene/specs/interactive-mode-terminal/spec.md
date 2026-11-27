# Spec Delta

## ADDED Requirements

### Requirement: The viewer strips the outer terminal's program resume token
When a command runs with `-i`, the viewer SHALL remove `OSC7577_TOKEN` from the child process environment (including serialized virtual-runtime environments and nested subprocesses) across all runtimes, because the viewer's emulated terminal does not forward OSC 7577 sequences to the outer terminal.

#### Scenario: Native interactive command does not inherit OSC7577_TOKEN
- **GIVEN** `OSC7577_TOKEN` is set in Invowk's outer environment
- **WHEN** a native command runs with `-i` and inspects `OSC7577_TOKEN`
- **THEN** `OSC7577_TOKEN` SHALL be unset inside the interactive session

#### Scenario: Virtual interactive command does not inherit OSC7577_TOKEN
- **GIVEN** `OSC7577_TOKEN` is set in Invowk's outer environment
- **WHEN** a `virtual-sh` or `virtual-lua` command runs with `-i` and inspects `OSC7577_TOKEN`
- **THEN** `OSC7577_TOKEN` SHALL be unset inside both the virtual runtime environment and the hidden helper subprocess environment
