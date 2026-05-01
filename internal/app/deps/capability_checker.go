// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// ErrCapabilityCheckerRequired is returned when host capability dependencies
// must be evaluated but the application layer did not inject an infrastructure
// checker.
var ErrCapabilityCheckerRequired = errors.New("host capability checker is required")

type (
	// CapabilityChecker checks host capabilities for dependency validation.
	CapabilityChecker interface {
		Check(context.Context, IOContext, invowkfile.CapabilityName) error
	}
)
