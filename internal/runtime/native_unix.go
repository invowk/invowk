// SPDX-License-Identifier: MPL-2.0

//go:build unix

package runtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
)

const (
	pgidPollInterval = 10 * time.Millisecond
	pgidKillWait     = 1 * time.Second
)

func configureNativeCmdCancellation(
	ctx context.Context,
	cmd *exec.Cmd,
	gracePeriod time.Duration,
	stdin io.Reader,
	stdout, stderr io.Writer,
) func() {
	hasTTY := hasDirectControllingTTY(stdin, stdout, stderr)
	if !hasTTY {
		// Isolate non-TTY/non-foreground native executions in their own Unix
		// process group so cancellation signals the entire child process tree.
		// Direct controlling-TTY runs stay in the foreground process group to
		// prevent SIGTTIN/SIGTTOU suspension in interactive tools.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	done := make(chan struct{})
	var killWg sync.WaitGroup
	var cancelOnce sync.Once
	var cleanupOnce sync.Once

	var stateMu sync.Mutex
	var cancelCalled bool
	var targetPGID int
	var sentSig syscall.Signal
	var cancelTime time.Time

	cmd.Cancel = func() error {
		var cancelErr error
		cancelOnce.Do(func() {
			if cmd.Process == nil || cmd.Process.Pid <= 0 {
				cancelErr = os.ErrProcessDone
				return
			}

			sig := resolveNativeTerminationSignal(ctx)
			targetPID := cmd.Process.Pid
			if !hasTTY {
				targetPID = -cmd.Process.Pid
			}

			stateMu.Lock()
			cancelCalled = true
			if !hasTTY {
				targetPGID = cmd.Process.Pid
			}
			sentSig = sig
			cancelTime = time.Now()
			stateMu.Unlock()

			cancelErr = syscall.Kill(targetPID, sig)
			if errors.Is(cancelErr, syscall.ESRCH) {
				cancelErr = os.ErrProcessDone
				return
			}

			killWg.Go(func() {
				timer := time.NewTimer(gracePeriod)
				defer timer.Stop()
				select {
				case <-done:
					return
				case <-timer.C:
					signalBestEffort(targetPID, syscall.SIGKILL)
				}
			})
		})
		return cancelErr
	}

	return func() {
		cleanupOnce.Do(func() {
			close(done)
			killWg.Wait()

			stateMu.Lock()
			wasCancelled := cancelCalled
			pgid := targetPGID
			sig := sentSig
			started := cancelTime
			stateMu.Unlock()

			if wasCancelled && !hasTTY && pgid > 0 {
				cleanupRemainingProcessGroup(pgid, sig, started, gracePeriod)
			}
		})
	}
}

//goplint:ignore -- POSIX process/pgid identifier at the syscall.Kill boundary.
func signalBestEffort(targetPID int, sig syscall.Signal) {
	if err := syscall.Kill(targetPID, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		slog.Debug("best-effort process signal failed", "target_pid", targetPID, "signal", sig, "error", err)
	}
}

// cleanupRemainingProcessGroup ensures descendants in an isolated process group
// do not survive after the direct child exits on cancellation (for example,
// when a non-interactive POSIX shell ignored SIGINT in `&` background jobs, or
// when inherited *os.File stdio lets cmd.Wait return before descendants exit).
//
//goplint:ignore -- POSIX process group identifier at the syscall.Kill boundary.
func cleanupRemainingProcessGroup(pgid int, initialSig syscall.Signal, cancelTime time.Time, gracePeriod time.Duration) {
	if errors.Is(syscall.Kill(-pgid, 0), syscall.ESRCH) {
		return
	}

	if initialSig == syscall.SIGINT {
		signalBestEffort(-pgid, syscall.SIGTERM)
	}

	deadline := cancelTime.Add(gracePeriod)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(-pgid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(pgidPollInterval)
	}

	signalBestEffort(-pgid, syscall.SIGKILL)
	killDeadline := time.Now().Add(pgidKillWait)
	for time.Now().Before(killDeadline) {
		if errors.Is(syscall.Kill(-pgid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(pgidPollInterval)
	}
}

func resolveNativeTerminationSignal(ctx context.Context) syscall.Signal {
	if ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return syscall.SIGTERM
	}
	if sig := TerminationSignalFromContext(ctx); sig != nil {
		if sysSig, ok := sig.(syscall.Signal); ok && sysSig > 0 {
			return sysSig
		}
	}
	return syscall.SIGTERM
}

func hasDirectControllingTTY(stdin io.Reader, stdout, stderr io.Writer) bool {
	return isTerminalStream(stdin) || isTerminalStream(stdout) || isTerminalStream(stderr)
}

func isTerminalStream(stream any) bool {
	file, ok := stream.(*os.File)
	if !ok || file == nil {
		return false
	}
	rawConn, err := file.SyscallConn()
	if err == nil {
		var isTerm bool
		if ctrlErr := rawConn.Control(func(fd uintptr) {
			isTerm = term.IsTerminal(int(fd))
		}); ctrlErr == nil {
			return isTerm
		}
	}
	return term.IsTerminal(int(file.Fd()))
}
