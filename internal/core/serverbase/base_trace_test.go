// SPDX-License-Identifier: MPL-2.0

package serverbase

import (
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/tlatrace"
)

// serverbaseProjection is ServerbaseTrace.tla's Proj for the real Base.
func serverbaseProjection(b *Base) tlatrace.Record {
	name := b.State().String()
	return tlatrace.Record{
		"state":     tlatrace.Str(strings.ToUpper(name[:1]) + name[1:]),
		"started":   tlatrace.Bool(startedChannelClosed(b)),
		"errClosed": tlatrace.Bool(errChannelClosed(b)),
	}
}

func serverbaseRecord(state string, started, errClosed bool) tlatrace.Record {
	return tlatrace.Record{"state": tlatrace.Str(state), "started": tlatrace.Bool(started), "errClosed": tlatrace.Bool(errClosed)}
}

// TestServerbase_TraceHarness records every ordered pair of operations run
// one at a time on the real Base, for trace validation against
// Serverbase.tla, plus targeted mutations that validation must reject.
func TestServerbase_TraceHarness(t *testing.T) {
	t.Parallel()
	dir := tlatrace.Dir(t)

	ops := []serverbaseOp{opStart, opStartCancelled, opRun, opFail, opStop, opStopped, opSend}
	var traces tlatrace.Traces
	for _, first := range ops {
		for _, second := range ops {
			b := NewBase()
			trace := []tlatrace.Record{serverbaseProjection(b)}
			for _, op := range []serverbaseOp{first, second} {
				applyServerbaseOp(t.Context(), b, op)
				trace = append(trace, serverbaseProjection(b))
			}
			traces.Accepted = append(traces.Accepted, trace)
		}
	}
	traces.Rejected = [][]tlatrace.Record{
		// One operation cannot move Created straight to Running.
		{serverbaseRecord("Created", false, false), serverbaseRecord("Running", true, false)},
		// A failed server never becomes Running again.
		{serverbaseRecord("Created", false, false), serverbaseRecord("Failed", false, true), serverbaseRecord("Running", true, true)},
		// Stopping a never-started server closes the error channel.
		{serverbaseRecord("Created", false, false), serverbaseRecord("Stopped", false, false)},
		// Readiness is never signalled before Running.
		{serverbaseRecord("Created", false, false), serverbaseRecord("Starting", true, false)},
	}
	tlatrace.Write(t, dir, "ServerbaseTraces", traces)
}
