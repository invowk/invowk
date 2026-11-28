# signal-propagation-and-cleanup Specification

## Purpose

Defines Unix `SIGTERM`/`SIGHUP`/`SIGINT` signal propagation, non-TTY native process-group termination, ephemeral container daemon cleanup on cancellation, and the `128 + signal` exit-code contract updates.

## Requirements

### Requirement: Unix termination signals trigger context cancellation and resource cleanup
On Unix platforms, `invowk` SHALL trap `SIGINT` (`os.Interrupt`), `SIGTERM`, and `SIGHUP`, cancel the active execution context, remove runtime temporary script files, and run registered cleanup hooks before exiting. On Windows, `invowk` SHALL trap `os.Interrupt`.

#### Scenario: SIGTERM cancels execution and cleans up temporary files
- **WHEN** a running `invowk cmd` execution on Unix receives `SIGTERM`
- **THEN** the execution context SHALL be cancelled, temporary script files created by Invowk SHALL be removed, and `invowk` SHALL exit cleanly after cleanup

### Requirement: Non-TTY native commands clean up their Unix process group
When running a native command on Unix without direct controlling-TTY inheritance (such as redirected/CI/systemd/captured runs), Invowk SHALL isolate the child in a new process group (`Setpgid: true`) and signal `-pgid` on cancellation with a bounded grace period before `SIGKILL`. Direct controlling-TTY runs SHALL remain in the foreground terminal process group so interactive tools do not block on `SIGTTIN`/`SIGTTOU`.

#### Scenario: Grandchild process spawned by non-TTY command is terminated on SIGTERM
- **WHEN** a non-TTY native command spawns a background subprocess and `invowk` receives `SIGTERM`
- **THEN** Invowk SHALL signal the child process group (`-pgid`) so the grandchild does not remain orphaned

### Requirement: Cancelled ephemeral container runs stop and remove daemon containers
When a non-persistent container run is cancelled via signal or context cancellation, Invowk SHALL stop and remove the ephemeral container on the container daemon within a bounded `context.WithoutCancel` cleanup window rather than leaving the container running after the CLI client exits.

#### Scenario: Cancelled ephemeral container does not remain running
- **WHEN** an ephemeral `container` runtime command is running and `invowk` receives `SIGTERM`
- **THEN** Invowk SHALL issue daemon cleanup (`stop`/`rm`) for that container identifier before exiting
