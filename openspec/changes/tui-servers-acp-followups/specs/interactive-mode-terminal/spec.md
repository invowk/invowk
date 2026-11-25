# Spec Delta

## MODIFIED Requirements

### Requirement: Leaving the viewer early hangs up the command
On Unix, when the viewer ends before the command does, by Ctrl+\ or by invowk being cancelled, the command's terminal SHALL be hung up, so its session receives SIGHUP, and the viewer SHALL return to the outer terminal without waiting for the command. The hang-up SHALL happen when the viewer ends, also when the command never read the input the viewer sent it, and not only when invowk exits. For a container command the session is the engine CLI's: the hang-up SHALL reach the engine CLI, and what then happens to the processes inside the container is the engine's behavior.

#### Scenario: Force quit
- **WHEN** a native command `sleep 3011 & echo SLEEP_PID=$!; wait` runs with `-i` and the user presses Ctrl+\
- **THEN** invowk SHALL exit, and the process with the printed `SLEEP_PID` SHALL no longer be running shortly after

#### Scenario: Force quit of a command that never reads its input
- **GIVEN** an interactive session whose command runs `stty raw -echo; trap 'echo HUP > hup.txt; exit 1' HUP; while :; do sleep 1; done`, which never reads its input
- **WHEN** enough keys and terminal replies are sent to fill the command's input, the viewer is ended by Ctrl+\, and the process that hosted the viewer keeps running
- **THEN** the session SHALL return, `hup.txt` SHALL appear and the command SHALL exit shortly after, while that process is still running
