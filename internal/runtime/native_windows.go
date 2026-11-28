// SPDX-License-Identifier: MPL-2.0

//go:build windows

package runtime

import (
	"context"
	"io"
	"os/exec"
	"time"
)

func configureNativeCmdCancellation(
	_ context.Context,
	_ *exec.Cmd,
	_ time.Duration,
	_ io.Reader,
	_, _ io.Writer,
) func() {
	return func() {}
}
