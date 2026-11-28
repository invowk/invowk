// SPDX-License-Identifier: MPL-2.0

//go:build !unix

package cmd

import (
	"os"

	"github.com/invowk/invowk/pkg/types"
)

func trappedTerminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

func signalExitCode(sig os.Signal) (types.ExitCode, bool) {
	if sig == nil {
		return 0, false
	}
	return types.ExitCode(1), true
}

func isWatchPreserveSignal(os.Signal) bool {
	return false
}
