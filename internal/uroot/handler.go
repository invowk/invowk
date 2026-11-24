// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"mvdan.cc/sh/v3/interp"
)

var (
	// errNoFollowValidatorMissing reports a validating context that sets only
	// ValidatePath: it cannot give the no-follow form, and the followed form
	// would act on a link's target instead of the link.
	errNoFollowValidatorMissing = errors.New("no-follow path validator missing")
	// errFollowValidatorMissing reports a validating context that sets only
	// ValidateNoFollowPath: a path it does not validate must not be used.
	errFollowValidatorMissing = errors.New("path validator missing")
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
		// ValidatePath resolves and validates a path before filesystem access,
		// following its last component (ResolvePath).
		ValidatePath func(cwd, path string) (string, error)
		// ValidateNoFollowPath resolves and validates a path for an operation
		// that acts on its last component itself and does not follow it: rm
		// operands, mv sources and ln's link name (ResolveNoFollowPath).
		ValidateNoFollowPath func(cwd, path string) (string, error)
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

// ResolvePath resolves path relative to the handler workdir, using ValidatePath
// when the embedding runtime provides a virtual filesystem policy. The last
// component is followed. A context that sets only ValidateNoFollowPath refuses
// with errFollowValidatorMissing instead of using the path unvalidated.
func (h *HandlerContext) ResolvePath(path string) (string, error) {
	if h != nil && h.ValidatePath != nil {
		return h.ValidatePath(h.Dir, path)
	}
	if h != nil && h.ValidateNoFollowPath != nil {
		return "", fmt.Errorf("%s: %w", path, errFollowValidatorMissing)
	}
	return h.joinDir(path), nil
}

// ResolveNoFollowPath resolves path for an operation that acts on its last
// component itself, using ValidateNoFollowPath when the embedding runtime
// provides a virtual filesystem policy. A context that sets only ValidatePath
// refuses with errNoFollowValidatorMissing: it never falls back to the form
// that follows the last component. With neither hook set, a relative path is
// joined to the handler workdir, as ResolvePath does.
func (h *HandlerContext) ResolveNoFollowPath(path string) (string, error) {
	if h != nil && h.ValidateNoFollowPath != nil {
		return h.ValidateNoFollowPath(h.Dir, path)
	}
	if h != nil && h.ValidatePath != nil {
		return "", fmt.Errorf("%s: %w", path, errNoFollowValidatorMissing)
	}
	return h.joinDir(path), nil
}

func (h *HandlerContext) joinDir(path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(h.Dir, path)
	}
	return path
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

// pathValidatingHandlerContext returns the injected handler context and
// whether it validates paths: it sets ValidatePath, ValidateNoFollowPath or
// both.
func pathValidatingHandlerContext(ctx context.Context) (*HandlerContext, bool) {
	hc, ok := ctx.Value(handlerContextKey{}).(*HandlerContext)
	return hc, ok && hc != nil && (hc.ValidatePath != nil || hc.ValidateNoFollowPath != nil)
}
