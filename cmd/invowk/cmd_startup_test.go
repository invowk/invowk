// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"path/filepath"
	"testing"
)

func TestShouldRegisterDiscoveredCommands(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "empty args",
			args: nil,
			want: false,
		},
		{
			name: "root version flag",
			args: []string{"--version"},
			want: false,
		},
		{
			name: "direct cmd invocation",
			args: []string{"cmd", "build"},
			want: true,
		},
		{
			name: "cmd with long config flag",
			args: []string{"--ivk-config", filepath.Join(tmpDir, "config.cue"), "cmd", "build"},
			want: true,
		},
		{
			name: "cmd with short config flag",
			args: []string{"-c", filepath.Join(tmpDir, "config.cue"), "cmd", "build"},
			want: true,
		},
		{
			name: "cmd with root bool flags",
			args: []string{"--ivk-verbose", "--ivk-interactive", "cmd", "build"},
			want: true,
		},
		{
			name: "shell completion for cmd",
			args: []string{"__complete", "cmd", "bu"},
			want: true,
		},
		{
			name: "shell completion no-desc for cmd",
			args: []string{"__completeNoDesc", "cmd", "bu"},
			want: true,
		},
		{
			name: "non-cmd command",
			args: []string{"init"},
			want: false,
		},
		{
			name: "arg terminator before cmd",
			args: []string{"--", "cmd", "build"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldRegisterDiscoveredCommands(tt.args)
			if got != tt.want {
				t.Fatalf("shouldRegisterDiscoveredCommands(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
