// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidFilesystemPath is the sentinel error re-exported for backward compatibility.
var ErrInvalidFilesystemPath = types.ErrInvalidFilesystemPath

type (
	// FilesystemPath is a type alias for the cross-cutting DDD Value Type defined
	// in pkg/types. All invowkfile consumers use this alias so they don't need to
	// import pkg/types directly.
	FilesystemPath = types.FilesystemPath

	// InvalidFilesystemPathError is a type alias re-exported for backward compatibility.
	InvalidFilesystemPathError = types.InvalidFilesystemPathError
)
