// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

const validationStructureCommandMutationFile = "/workspace/invowkfile.cue"

func TestStructureCommandMutationDiagnostics(t *testing.T) {
	t.Parallel()

	longCommandName := CommandName("a" + strings.Repeat("b", MaxNameLength))
	longScriptFile := strings.Repeat("f", MaxPathLength+1)

	tests := []struct {
		name        string
		mutate      func(*testing.T, *Invowkfile)
		wantField   string
		wantMessage string
		wantCause   error
	}{
		{
			name: "command name length",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Name = longCommandName
			},
			wantField:   "command '" + string(longCommandName) + "'",
			wantMessage: "command name too long (257 chars, max 256) in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "command description length",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Description = DescriptionText(strings.Repeat("d", MaxDescriptionLength+1))
			},
			wantField:   "command 'deploy'",
			wantMessage: "description too long (10241 chars, max 10240) in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "missing implementation",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Implementations = nil
			},
			wantField:   "command 'deploy'",
			wantMessage: "must have at least one implementation in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "duplicate platform runtime",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				impl := inv.Commands[0].Implementations[0]
				inv.Commands[0].Implementations = append(inv.Commands[0].Implementations, impl)
			},
			wantField: "command 'deploy'",
			wantMessage: "command 'deploy' has duplicate platform+runtime combination: " +
				"platform=linux, runtime=native (implementations #1 and #2)",
		},
		{
			name: "command env",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Env = &EnvConfig{Vars: map[EnvVarName]string{"1BAD": "x"}}
			},
			wantField: "command 'deploy' env.vars['1BAD']",
			wantMessage: `invalid environment variable name "1BAD" ` +
				`(must match [A-Za-z_][A-Za-z0-9_]*) in invowkfile at /workspace/invowkfile.cue`,
		},
		{
			name: "missing runtime",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Implementations[0].Runtimes = nil
			},
			wantField:   "command 'deploy' implementation #1",
			wantMessage: "must have at least one runtime in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "missing platform",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Implementations[0].Platforms = nil
			},
			wantField:   "command 'deploy' implementation #1",
			wantMessage: "must have at least one platform in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "implementation env",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Implementations[0].Env = &EnvConfig{Vars: map[EnvVarName]string{"1BAD": "x"}}
			},
			wantField: "command 'deploy' implementation #1 env.vars['1BAD']",
			wantMessage: `invalid environment variable name "1BAD" ` +
				`(must match [A-Za-z_][A-Za-z0-9_]*) in invowkfile at /workspace/invowkfile.cue`,
		},
		{
			name: "implementation timeout",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Implementations[0].Timeout = "0s"
			},
			wantField:   "command 'deploy' implementation #1 timeout",
			wantMessage: `invalid duration string "0s": must be a positive duration`,
		},
		{
			name: "missing script source",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Implementations[0].Script = ImplementationScript{}
			},
			wantField:   "command 'deploy' implementation #1 script",
			wantMessage: "invalid implementation script: 1 field error(s) in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "script content length",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Implementations[0].Script.Content = ScriptContent(strings.Repeat("s", MaxScriptLength+1))
			},
			wantField:   "command 'deploy' implementation #1 script content",
			wantMessage: "script.content too long (10485761 chars, max 10485760) in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "non-module script file",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				file := ScriptFilePath("scripts/deploy.sh")
				inv.Commands[0].Implementations[0].Script = ImplementationScript{File: &file}
			},
			wantField:   "command 'deploy' implementation #1 script file",
			wantMessage: "script file requires module invowkfile in invowkfile at /workspace/invowkfile.cue",
			wantCause:   ErrScriptFileRequiresModule,
		},
		{
			name: "script file length",
			mutate: func(t *testing.T, inv *Invowkfile) {
				t.Helper()

				inv.ModulePath = FilesystemPath(t.TempDir())
				file := ScriptFilePath(longScriptFile)
				inv.Commands[0].Implementations[0].Script = ImplementationScript{File: &file}
			},
			wantField:   "command 'deploy' implementation #1 script",
			wantMessage: "invalid implementation script: 1 field error(s) in invowkfile at /workspace/invowkfile.cue",
			wantCause:   ErrInvalidScriptFilePath,
		},
		{
			name: "runtime config",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Implementations[0].Runtimes[0].Containerfile = "Containerfile"
			},
			wantField:   "command 'deploy' implementation #1 runtime #1",
			wantMessage: "containerfile is only valid for container runtime",
		},
		{
			name: "containerfile path",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Implementations[0].Runtimes[0] = RuntimeConfig{
					Name:          RuntimeContainer,
					Containerfile: "bad:name",
				}
			},
			wantField:   "command 'deploy' implementation #1 runtime #1",
			wantMessage: "filename contains invalid character ':' in invowkfile at /workspace/invowkfile.cue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inv := validationStructureCommandMutationInvowkfile()
			tt.mutate(t, inv)

			errs := inv.Validate()
			got := requireValidationStructureCommandIssue(t, errs, tt.wantField, tt.wantMessage)
			if tt.wantCause != nil && !errors.Is(got.Cause, tt.wantCause) {
				t.Fatalf("validation cause = %v, want %v", got.Cause, tt.wantCause)
			}
		})
	}
}

func TestStructureCommandMutationDependencyDelegation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*Invowkfile)
		wantField string
	}{
		{
			name: "command dependencies",
			mutate: func(inv *Invowkfile) {
				inv.Commands[0].DependsOn = &DependsOn{Tools: []ToolDependency{{}}}
			},
			wantField: "command 'deploy' depends_on tools[1]",
		},
		{
			name: "implementation dependencies",
			mutate: func(inv *Invowkfile) {
				inv.Commands[0].Implementations[0].DependsOn = &DependsOn{Tools: []ToolDependency{{}}}
			},
			wantField: "command 'deploy' implementation #1 depends_on tools[1]",
		},
		{
			name: "container runtime dependencies",
			mutate: func(inv *Invowkfile) {
				inv.Commands[0].Implementations[0].Runtimes[0] = RuntimeConfig{
					Name:      RuntimeContainer,
					Image:     "debian:stable-slim",
					DependsOn: &DependsOn{Tools: []ToolDependency{{}}},
				}
			},
			wantField: "command 'deploy' implementation #1 runtime #1 depends_on tools[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inv := validationStructureCommandMutationInvowkfile()
			tt.mutate(inv)

			got := requireValidationStructureCommandIssue(
				t,
				inv.Validate(),
				tt.wantField,
				"invalid tool dependency: 1 field error(s) in invowkfile at /workspace/invowkfile.cue",
			)
			if !errors.Is(got.Cause, ErrMissingDependencyAlternatives) {
				t.Fatalf("validation cause = %v, want ErrMissingDependencyAlternatives", got.Cause)
			}
		})
	}
}

func TestStructureCommandMutationWatchConfigExpansion(t *testing.T) {
	t.Parallel()

	inv := validationStructureCommandMutationInvowkfile()
	inv.Commands[0].Watch = &WatchConfig{
		Patterns: []GlobPattern{""},
		Debounce: "0s",
	}

	errs := inv.Validate()
	requireValidationStructureCommandIssue(
		t,
		errs,
		"command 'deploy' watch",
		`invalid glob pattern "": must not be empty`,
	)
	requireValidationStructureCommandIssue(
		t,
		errs,
		"command 'deploy' watch",
		`invalid duration string "0s": must be a positive duration`,
	)
}

func validationStructureCommandMutationInvowkfile() *Invowkfile {
	return &Invowkfile{
		FilePath: validationStructureCommandMutationFile,
		Commands: []Command{{
			Name:        "deploy",
			Description: "Deploy the service",
			Implementations: []Implementation{{
				Script:    ImplementationScript{Content: "echo deploy"},
				Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			}},
		}},
	}
}

func requireValidationStructureCommandIssue(
	t *testing.T,
	errs ValidationErrors,
	wantField string,
	wantMessage string,
) ValidationError {
	t.Helper()

	for _, err := range errs {
		if err.Validator != structureValidatorName {
			continue
		}
		if err.Field == wantField && err.Message == wantMessage && err.Severity == SeverityError {
			return err
		}
	}

	t.Fatalf("missing validation issue field=%q message=%q; got:\n%s", wantField, wantMessage, errs.Error())
	return ValidationError{}
}
