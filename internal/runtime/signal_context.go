// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"os"
)

type terminationSignalLookupKey struct{}

// ContextWithTerminationSignalLookup attaches a callback that returns the
// process termination signal recorded on the root context, if any.
func ContextWithTerminationSignalLookup(ctx context.Context, lookup func() os.Signal) context.Context {
	if ctx == nil || lookup == nil {
		return ctx
	}
	return context.WithValue(ctx, terminationSignalLookupKey{}, lookup)
}

// TerminationSignalFromContext returns the process termination signal recorded
// on ctx, or nil if no termination signal has been observed.
func TerminationSignalFromContext(ctx context.Context) os.Signal {
	if ctx == nil {
		return nil
	}
	lookup, ok := ctx.Value(terminationSignalLookupKey{}).(func() os.Signal)
	if !ok || lookup == nil {
		return nil
	}
	return lookup()
}
