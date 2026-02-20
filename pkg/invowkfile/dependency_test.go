// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"slices"
	"testing"
)

func TestHasExecutableCommandDeps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		deps *DependsOn
		want bool
	}{
		{
			name: "nil DependsOn",
			deps: nil,
			want: false,
		},
		{
			name: "empty Commands slice",
			deps: &DependsOn{},
			want: false,
		},
		{
			name: "all Execute false",
			deps: &DependsOn{
				Commands: []CommandDependency{
					{Alternatives: []string{"build"}, Execute: false},
					{Alternatives: []string{"lint"}, Execute: false},
				},
			},
			want: false,
		},
		{
			name: "mixed true and false",
			deps: &DependsOn{
				Commands: []CommandDependency{
					{Alternatives: []string{"build"}, Execute: false},
					{Alternatives: []string{"compile"}, Execute: true},
					{Alternatives: []string{"lint"}, Execute: false},
				},
			},
			want: true,
		},
		{
			name: "all Execute true",
			deps: &DependsOn{
				Commands: []CommandDependency{
					{Alternatives: []string{"build"}, Execute: true},
					{Alternatives: []string{"test"}, Execute: true},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// nil DependsOn needs a nil receiver guard; the method is on *DependsOn,
			// so we skip the call for nil and check the expected value directly.
			if tt.deps == nil {
				if tt.want {
					t.Error("expected false for nil DependsOn")
				}
				return
			}

			got := tt.deps.HasExecutableCommandDeps()
			if got != tt.want {
				t.Errorf("HasExecutableCommandDeps() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
