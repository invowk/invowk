// SPDX-License-Identifier: MPL-2.0

package types

import "fmt"

// FormatFieldErrors formats the standard error message used by DDD Invalid*Error types.
// It produces a human-readable summary like "invalid foo: 3 field error(s)" that is
// consistent across all composite struct validation errors. Only the count of
// fieldErrors is included in the output; individual errors are exposed by callers
// through Unwrap() or multi-error composition.
//
//goplint:ignore -- label is a human-readable display string for error formatting, not a domain value.
func FormatFieldErrors(label string, fieldErrors []error) string {
	return fmt.Sprintf("invalid %s: %d field error(s)", label, len(fieldErrors))
}
