// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
)

func TestValidateContainerfilePath_Matrix(t *testing.T) {
	t.Parallel()

	reject := pathmatrix.Reject()
	pathmatrix.Validator(t, func(input string) error {
		return ValidateContainerfilePath(input, "unused")
	}, pathmatrix.Expectations{
		UnixAbsolute:       reject,
		WindowsDriveAbs:    reject,
		WindowsRooted:      reject,
		UNC:                reject,
		SlashTraversal:     reject,
		BackslashTraversal: reject,
		ValidRelative:      pathmatrix.PassAny(nil),
	})
}

func TestValidateEnvFilePath_Matrix(t *testing.T) {
	t.Parallel()

	reject := pathmatrix.Reject()
	pathmatrix.Validator(t, ValidateEnvFilePath, pathmatrix.Expectations{
		UnixAbsolute:       reject,
		WindowsDriveAbs:    reject,
		WindowsRooted:      reject,
		UNC:                reject,
		SlashTraversal:     reject,
		BackslashTraversal: reject,
		ValidRelative:      pathmatrix.PassAny(nil),
	})
}
