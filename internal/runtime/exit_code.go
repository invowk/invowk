// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidExitCode is the sentinel error re-exported for backward compatibility.
var ErrInvalidExitCode = types.ErrInvalidExitCode

type (
	// ExitCode is a type alias for the cross-cutting DDD Value Type defined
	// in pkg/types. Both runtime.Result and container.RunResult use ExitCode.
	ExitCode = types.ExitCode

	// InvalidExitCodeError is a type alias re-exported for backward compatibility.
	InvalidExitCodeError = types.InvalidExitCodeError
)
