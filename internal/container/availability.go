// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"time"
)

const (
	availabilityProbeAttempts    = 3
	availabilityProbeBaseBackoff = 250 * time.Millisecond
)

type availabilityProbe func(ctx context.Context) error

func probeEngineAvailability(probe availabilityProbe) bool {
	return probeEngineAvailabilityWithRetryConfig(
		probe,
		time.Sleep,
	)
}

func probeEngineAvailabilityWithRetryConfig(
	probe availabilityProbe,
	sleepFn func(time.Duration),
) bool {
	if probe == nil {
		return false
	}
	if sleepFn == nil {
		sleepFn = time.Sleep
	}

	for attempt := range availabilityProbeAttempts {
		ctx, cancel := context.WithTimeout(context.Background(), availabilityTimeout)
		err := probe(ctx)
		cancel()

		if err == nil {
			return true
		}
		if !isTransientAvailabilityError(err) {
			return false
		}

		if attempt < availabilityProbeAttempts-1 {
			sleepFn(availabilityProbeBaseBackoff * time.Duration(attempt+1))
		}
	}

	return false
}

func isTransientAvailabilityError(err error) bool {
	if err == nil {
		return false
	}
	// For availability probes, timeout/cancel commonly means the engine is
	// cold-starting or under load. Retry before declaring it unavailable.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	return IsTransientError(err)
}
