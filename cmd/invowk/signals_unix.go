// SPDX-License-Identifier: MPL-2.0

//go:build unix

package cmd

import (
	"os"
	"syscall"

	"github.com/invowk/invowk/pkg/types"
)

const posixSignalExitOffset = 128

func trappedTerminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}

func signalExitCode(sig os.Signal) (types.ExitCode, bool) {
	if sig == nil {
		return 0, false
	}
	sysSig, ok := sig.(syscall.Signal)
	if !ok || sysSig <= 0 {
		return types.ExitCode(1), true
	}
	code := types.ExitCode(posixSignalExitOffset + int(sysSig))
	if err := code.Validate(); err != nil {
		return types.ExitCode(1), true
	}
	return code, true
}

func isWatchPreserveSignal(sig os.Signal) bool {
	return sig == syscall.SIGHUP || sig == syscall.SIGTERM
}
