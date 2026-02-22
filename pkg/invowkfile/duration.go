// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"time"
)

// parseDuration parses a Go duration string and rejects empty, zero, or negative values.
// Returns (0, nil) when value is empty (caller should apply default).
// The fieldName is used in error messages (e.g., "debounce", "timeout").
func parseDuration(fieldName, value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", fieldName, value, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid %s %q: must be a positive duration", fieldName, value)
	}
	return d, nil
}
