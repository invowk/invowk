// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package watch

import (
	"fmt"
	"syscall"
	"testing"
)

func TestIsFatalFsnotifyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "ENOSPC is fatal", err: syscall.ENOSPC, want: true},
		{name: "EMFILE is fatal", err: syscall.EMFILE, want: true},
		{name: "ENFILE is fatal", err: syscall.ENFILE, want: true},
		{name: "wrapped ENOSPC is fatal", err: fmt.Errorf("fsnotify: %w", syscall.ENOSPC), want: true},
		{name: "EPERM is not fatal", err: syscall.EPERM, want: false},
		{name: "EACCES is not fatal", err: syscall.EACCES, want: false},
		{name: "generic error is not fatal", err: fmt.Errorf("something went wrong"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isFatalFsnotifyError(tt.err); got != tt.want {
				t.Errorf("isFatalFsnotifyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
