// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// HostProbe performs host-device checks for dependency validation.
	HostProbe interface {
		CheckTool(invowkfile.BinaryName) error
		CheckFilepath(displayPath, resolvedPath types.FilesystemPath, fp invowkfile.FilepathDependency) error
		RunCustomCheck(ctx context.Context, check invowkfile.CustomCheck) error
	}

	defaultHostProbe struct{}
)

func newDefaultHostProbe() HostProbe {
	return defaultHostProbe{}
}

func (defaultHostProbe) CheckTool(toolName invowkfile.BinaryName) error {
	return ValidateToolNative(toolName)
}

func (defaultHostProbe) CheckFilepath(displayPath, resolvedPath types.FilesystemPath, fp invowkfile.FilepathDependency) error {
	return ValidateSingleFilepath(displayPath, resolvedPath, fp)
}

func (defaultHostProbe) RunCustomCheck(ctx context.Context, check invowkfile.CustomCheck) error {
	return validateCustomCheckNative(ctx, check)
}
