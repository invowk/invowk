// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"fmt"
	"time"
)

// RetryWithBackoff retries op up to maxAttempts times with exponential backoff.
// It checks ctx.Err() between retries to respect cancellation immediately,
// preventing wasted work when the caller has already abandoned the operation.
//
// op returns (shouldRetry bool, err error). If shouldRetry is false, err is
// returned immediately (nil on success, non-nil on permanent failure).
// On retry exhaustion, the last error is returned.
func RetryWithBackoff(
	ctx context.Context,
	maxAttempts int,
	baseBackoff time.Duration,
	op func(attempt int) (retry bool, err error),
) error {
	var lastErr error
	for attempt := range maxAttempts {
		if attempt > 0 {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("retry aborted: %w", err)
			}
			time.Sleep(baseBackoff * time.Duration(1<<(attempt-1)))
		}

		retry, err := op(attempt)
		if err == nil {
			return nil
		}
		if !retry {
			return err
		}
		lastErr = err
	}
	return lastErr
}
