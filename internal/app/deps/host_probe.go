// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// ErrHostProbeRequired is returned when host dependencies must be evaluated but
// the application layer did not inject an infrastructure host probe.
var ErrHostProbeRequired = errors.New("host dependency probe is required")

type (
	// HostProbe performs host-device checks for dependency validation.
	HostProbe interface {
		CheckTool(invowkfile.BinaryName) error
		CheckFilepath(displayPath, resolvedPath types.FilesystemPath, fp invowkfile.FilepathDependency) error
		RunCustomCheck(ctx context.Context, check invowkfile.CustomCheck) error
	}
)
