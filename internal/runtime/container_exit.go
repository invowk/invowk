// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

// IsTransientContainerEngineExitCode reports whether an exit code represents
// transient container engine infrastructure failure at the runtime boundary.
func IsTransientContainerEngineExitCode(code types.ExitCode) bool {
	return container.IsTransientEngineExitCode(code)
}
