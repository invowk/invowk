// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"io"

	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	// IOContext carries host streams needed by dependency probes.
	//
	//goplint:ignore -- dependency-validation adapter DTO mirrors optional runtime streams.
	IOContext struct {
		Stdout io.Writer
		Stderr io.Writer
		Stdin  io.Reader
	}

	// ExecutionContext carries dependency-validation state without importing a
	// concrete runtime execution DTO.
	//
	//goplint:ignore -- dependency-validation adapter DTO is assembled at the service boundary.
	ExecutionContext struct {
		Context                 context.Context
		CommandName             invowkfile.CommandName //goplint:ignore -- empty value is allowed for discovery-only dependency checks.
		SelectedRuntime         invowkfile.RuntimeMode
		ImplementationDependsOn *invowkfile.DependsOn //goplint:no-delegate -- dependency payload is validated before this adapter DTO is assembled.
		RuntimeDependsOn        *invowkfile.DependsOn //goplint:no-delegate -- dependency payload is validated before this adapter DTO is assembled.
		IO                      IOContext
	}
)

// Validate returns nil when typed dependency context fields are valid.
func (ctx ExecutionContext) Validate() error {
	if ctx.CommandName != "" {
		if err := ctx.CommandName.Validate(); err != nil {
			return err
		}
	}
	if ctx.SelectedRuntime != "" {
		if err := ctx.SelectedRuntime.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// GoContext returns the cancellation context for dependency probes.
func (ctx ExecutionContext) GoContext() context.Context {
	if ctx.Context == nil {
		return context.Background()
	}
	return ctx.Context
}
