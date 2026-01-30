// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"io"

	"mvdan.cc/sh/v3/interp"
)

type (
	// HandlerContext provides execution context for u-root commands.
	// This is extracted from mvdan/sh's interp.HandlerCtx.
	HandlerContext struct {
		// Stdin is the input stream for the command.
		Stdin io.Reader
		// Stdout is the output stream for the command.
		Stdout io.Writer
		// Stderr is the error output stream for the command.
		Stderr io.Writer
		// Dir is the current working directory.
		Dir string
		// LookupEnv retrieves environment variables.
		LookupEnv func(string) (string, bool)
	}

	// handlerContextKey is the context key for storing HandlerContext.
	handlerContextKey struct{}
)

// ExtractHandlerContext extracts the HandlerContext from mvdan/sh's context.
// This bridges the shell interpreter's context to our u-root command execution.
func ExtractHandlerContext(ctx context.Context) *HandlerContext {
	hc := interp.HandlerCtx(ctx)
	return &HandlerContext{
		Stdin:  hc.Stdin,
		Stdout: hc.Stdout,
		Stderr: hc.Stderr,
		Dir:    hc.Dir,
		// Wrap expand.Environ.Get() to match our simpler LookupEnv signature.
		// expand.Variable.Set indicates if the variable was set.
		LookupEnv: func(name string) (string, bool) {
			v := hc.Env.Get(name)
			return v.Str, v.Set
		},
	}
}

// WithHandlerContext stores a HandlerContext in the context.
// This is primarily used for testing where we need to inject a custom HandlerContext.
func WithHandlerContext(ctx context.Context, hc *HandlerContext) context.Context {
	return context.WithValue(ctx, handlerContextKey{}, hc)
}

// GetHandlerContext retrieves the HandlerContext from the context.
// If the context was created with WithHandlerContext, it returns that value.
// Otherwise, it extracts from mvdan/sh's handler context.
func GetHandlerContext(ctx context.Context) *HandlerContext {
	if hc, ok := ctx.Value(handlerContextKey{}).(*HandlerContext); ok {
		return hc
	}
	return ExtractHandlerContext(ctx)
}
