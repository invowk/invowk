// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"github.com/invowk/invowk/pkg/types"
)

// FilesystemPath in this package is a Go type alias for types.FilesystemPath.
// The original validator tests live in pkg/types/filesystem_path_test.go;
// keeping a duplicate here adds zero coverage. These compile-time assertions
// keep the alias contract enforced — if the alias is ever replaced by a
// distinct wrapper type, this file fails to build and the duplicated tests
// can be reintroduced as needed.
var (
	_ types.FilesystemPath              = FilesystemPath("")
	_ FilesystemPath                    = types.FilesystemPath("")
	_ *types.InvalidFilesystemPathError = (*InvalidFilesystemPathError)(nil)
	_ *InvalidFilesystemPathError       = (*types.InvalidFilesystemPathError)(nil)
)
