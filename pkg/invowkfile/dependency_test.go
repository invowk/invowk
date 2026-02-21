// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"slices"
	"testing"
)

func TestGetExecutableCommandDeps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		deps           *DependsOn
		wantNil        bool
		wantAlts       [][]string // expected Alternatives for each returned dep
		wantExecuteAll bool       // all returned deps should have Execute: true
	}{
		{
			name:    "nil DependsOn",
			deps:    nil,
			wantNil: true,
		},
		{
			name:    "empty Commands slice",
			deps:    &DependsOn{},
			wantNil: true,
		},
		{
			name: "all Execute false returns nil",
			deps: &DependsOn{
				Commands: []CommandDependency{
					{Alternatives: []string{"build"}, Execute: false},
					{Alternatives: []string{"lint"}, Execute: false},
				},
			},
			wantNil: true,
		},
		{
			name: "mixed true and false returns only true",
			deps: &DependsOn{
				Commands: []CommandDependency{
					{Alternatives: []string{"build"}, Execute: false},
					{Alternatives: []string{"compile"}, Execute: true},
					{Alternatives: []string{"lint"}, Execute: false},
					{Alternatives: []string{"test"}, Execute: true},
				},
			},
			wantAlts:       [][]string{{"compile"}, {"test"}},
			wantExecuteAll: true,
		},
		{
			name: "order preservation verified",
			deps: &DependsOn{
				Commands: []CommandDependency{
					{Alternatives: []string{"alpha"}, Execute: true},
					{Alternatives: []string{"beta"}, Execute: false},
					{Alternatives: []string{"gamma"}, Execute: true},
					{Alternatives: []string{"delta"}, Execute: true},
				},
			},
			wantAlts:       [][]string{{"alpha"}, {"gamma"}, {"delta"}},
			wantExecuteAll: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.deps == nil {
				if !tt.wantNil {
					t.Error("expected non-nil result for nil DependsOn")
				}
				return
			}

			got := tt.deps.GetExecutableCommandDeps()

			if tt.wantNil {
				if got != nil {
					t.Errorf("GetExecutableCommandDeps() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.wantAlts) {
				t.Fatalf("GetExecutableCommandDeps() returned %d deps, want %d", len(got), len(tt.wantAlts))
			}

			for i, dep := range got {
				if tt.wantExecuteAll && !dep.Execute {
					t.Errorf("GetExecutableCommandDeps()[%d].Execute = false, want true", i)
				}
				if !slices.Equal(dep.Alternatives, tt.wantAlts[i]) {
					t.Errorf("GetExecutableCommandDeps()[%d].Alternatives = %v, want %v", i, dep.Alternatives, tt.wantAlts[i])
				}
			}
		})
	}
}

func TestMergeDependsOnAll_ExecuteDepsPreserveOrder(t *testing.T) {
	t.Parallel()

	rootDeps := &DependsOn{
		Commands: []CommandDependency{
			{Alternatives: []string{"setup"}, Execute: true},
		},
	}
	cmdDeps := &DependsOn{
		Commands: []CommandDependency{
			{Alternatives: []string{"build"}, Execute: true},
			{Alternatives: []string{"lint"}, Execute: false},
		},
	}
	implDeps := &DependsOn{
		Commands: []CommandDependency{
			{Alternatives: []string{"compile"}, Execute: true},
			// Duplicate of root-level dep to verify both appear in merged result.
			{Alternatives: []string{"setup"}, Execute: true},
		},
	}

	merged := MergeDependsOnAll(rootDeps, cmdDeps, implDeps)
	if merged == nil {
		t.Fatal("MergeDependsOnAll() returned nil")
	}

	execDeps := merged.GetExecutableCommandDeps()
	// Expected order: root(setup), cmd(build), impl(compile), impl(setup)
	wantAlts := [][]string{{"setup"}, {"build"}, {"compile"}, {"setup"}}
	if len(execDeps) != len(wantAlts) {
		t.Fatalf("got %d exec deps, want %d", len(execDeps), len(wantAlts))
	}
	for i, dep := range execDeps {
		if !slices.Equal(dep.Alternatives, wantAlts[i]) {
			t.Errorf("exec dep[%d].Alternatives = %v, want %v", i, dep.Alternatives, wantAlts[i])
		}
		if !dep.Execute {
			t.Errorf("exec dep[%d].Execute = false, want true", i)
		}
	}

	// Non-execute deps should also be present.
	allCmds := merged.Commands
	foundLint := false
	for _, cmd := range allCmds {
		if len(cmd.Alternatives) > 0 && cmd.Alternatives[0] == "lint" && !cmd.Execute {
			foundLint = true
		}
	}
	if !foundLint {
		t.Error("non-execute dep 'lint' missing from merged result")
	}
}

func TestMergeDependsOnAll_NilLevelsPreserveExecuteDeps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		root     *DependsOn
		cmd      *DependsOn
		impl     *DependsOn
		wantExec int
	}{
		{
			name:     "only root has execute deps",
			root:     &DependsOn{Commands: []CommandDependency{{Alternatives: []string{"a"}, Execute: true}}},
			wantExec: 1,
		},
		{
			name:     "only impl has execute deps",
			impl:     &DependsOn{Commands: []CommandDependency{{Alternatives: []string{"a"}, Execute: true}}},
			wantExec: 1,
		},
		{
			name:     "all levels have execute deps",
			root:     &DependsOn{Commands: []CommandDependency{{Alternatives: []string{"a"}, Execute: true}}},
			cmd:      &DependsOn{Commands: []CommandDependency{{Alternatives: []string{"b"}, Execute: true}}},
			impl:     &DependsOn{Commands: []CommandDependency{{Alternatives: []string{"c"}, Execute: true}}},
			wantExec: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			merged := MergeDependsOnAll(tt.root, tt.cmd, tt.impl)
			if merged == nil {
				t.Fatal("MergeDependsOnAll() returned nil")
			}
			got := len(merged.GetExecutableCommandDeps())
			if got != tt.wantExec {
				t.Errorf("got %d execute deps, want %d", got, tt.wantExec)
			}
		})
	}
}
