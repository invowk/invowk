// SPDX-License-Identifier: MPL-2.0

//go:build windows

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
		{name: "ERROR_TOO_MANY_OPEN_FILES is fatal", err: syscall.Errno(4), want: true},
		{name: "ERROR_INVALID_HANDLE is fatal", err: syscall.Errno(6), want: true},
		{name: "ERROR_NOT_ENOUGH_MEMORY is fatal", err: syscall.Errno(8), want: true},
		{name: "wrapped ERROR_INVALID_HANDLE is fatal", err: fmt.Errorf("fsnotify: %w", syscall.Errno(6)), want: true},
		{name: "ERROR_ACCESS_DENIED is not fatal", err: syscall.Errno(5), want: false},
		{name: "ERROR_FILE_NOT_FOUND is not fatal", err: syscall.Errno(2), want: false},
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
