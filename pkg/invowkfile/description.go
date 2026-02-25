// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidDescriptionText is the sentinel error re-exported for backward compatibility.
var ErrInvalidDescriptionText = types.ErrInvalidDescriptionText

type (
	// DescriptionText is a type alias for the cross-cutting DDD Value Type defined
	// in pkg/types. All invowkfile consumers (Command, Flag, Argument, ModuleMetadata)
	// use this alias so they don't need to import pkg/types directly.
	DescriptionText = types.DescriptionText

	// InvalidDescriptionTextError is a type alias re-exported for backward compatibility.
	InvalidDescriptionTextError = types.InvalidDescriptionTextError
)
