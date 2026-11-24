// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"os"

	"mvdan.cc/sh/v3/interp"
)

// accessHandler returns the access handler of both virtual-sh runner setups:
// `cd`'s access check and the -r, -w and -x tests (design D3, "The access
// handler", of virtual-path-no-follow-operands).
//
// It validates path with validate: full resolution, following the last
// component, with the allowed-root check under restricted access. A path that
// fails is refused with an *os.PathError (Op "access") and never reaches next,
// so the test is false and `cd` fails, and no host access check runs on it.
// Otherwise it calls next with the same context and the resolved path. On
// Unix next is access(2); on other platforms mvdan emulates it with the stat
// handler and followSymlinks false, which on the resolved path (it holds no
// link) reads the target's permission bits, so `cd` into a directory link or
// junction keeps working. -L and -h do not come here: they stay with the stat
// handler's no-follow form.
func (v virtualPathValidator) accessHandler(next interp.AccessHandlerFunc) interp.AccessHandlerFunc {
	return func(ctx context.Context, path string, mode interp.AccessMode) error {
		hc := interp.HandlerCtx(ctx)
		resolved, err := v.validate(hc.Dir, path)
		if err != nil {
			return &os.PathError{Op: "access", Path: path, Err: err}
		}
		return next(ctx, resolved, mode)
	}
}
