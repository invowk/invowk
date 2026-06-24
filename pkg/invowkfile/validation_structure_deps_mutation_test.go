// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const validationStructureDepsMutationFile = "/workspace/invowkfile.cue"

func TestStructureDependsOnMutationDiagnostics(t *testing.T) {
	t.Parallel()

	longScriptFile := strings.Repeat("f", MaxPathLength+1)

	tests := []struct {
		name        string
		mutate      func(*testing.T, *Invowkfile)
		wantField   string
		wantMessage string
		wantCause   error
	}{
		{
			name: "tool dependency shape",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{Tools: []ToolDependency{{}}}
			},
			wantField:   "root depends_on tools[1]",
			wantMessage: "invalid tool dependency: 1 field error(s) in invowkfile at /workspace/invowkfile.cue",
			wantCause:   ErrMissingDependencyAlternatives,
		},
		{
			name: "tool dependency continues after shape error",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{Tools: []ToolDependency{{}, {}}}
			},
			wantField:   "root depends_on tools[2]",
			wantMessage: "invalid tool dependency: 1 field error(s) in invowkfile at /workspace/invowkfile.cue",
			wantCause:   ErrMissingDependencyAlternatives,
		},
		{
			name: "command dependency continues after shape error",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{Commands: []CommandDependency{{}, {}}}
			},
			wantField:   "root depends_on cmds[2]",
			wantMessage: "invalid command dependency: 1 field error(s) in invowkfile at /workspace/invowkfile.cue",
			wantCause:   ErrMissingDependencyAlternatives,
		},
		{
			name: "command dependency index",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{
					Commands: []CommandDependency{
						{Alternatives: []CommandDependencyRef{"build"}},
						{},
					},
				}
			},
			wantField:   "root depends_on cmds[2]",
			wantMessage: "invalid command dependency: 1 field error(s) in invowkfile at /workspace/invowkfile.cue",
			wantCause:   ErrMissingDependencyAlternatives,
		},
		{
			name: "filepath dependency shape",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{Filepaths: []FilepathDependency{{}}}
			},
			wantField:   "root depends_on filepaths[1]",
			wantMessage: "invalid filepath dependency: 1 field error(s) in invowkfile at /workspace/invowkfile.cue",
			wantCause:   ErrMissingDependencyAlternatives,
		},
		{
			name: "filepath dependency null byte",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{
					Filepaths: []FilepathDependency{{
						Alternatives: []FilesystemPath{"config.json", "bad\x00path"},
					}},
				}
			},
			wantField:   "root filepaths[1]",
			wantMessage: "filepath alternative #2 contains null byte in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "capability dependency index",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{
					Capabilities: []CapabilityDependency{
						{Alternatives: []CapabilityName{CapabilityContainers}},
						{},
					},
				}
			},
			wantField:   "root depends_on capabilities[2]",
			wantMessage: "invalid capability dependency: 1 field error(s) in invowkfile at /workspace/invowkfile.cue",
			wantCause:   ErrMissingDependencyAlternatives,
		},
		{
			name: "env var dependency index",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{
					EnvVars: []EnvVarDependency{
						{Alternatives: []EnvVarCheck{{Name: "HOME"}}},
						{},
					},
				}
			},
			wantField:   "root depends_on env_vars[2]",
			wantMessage: "invalid env var dependency: 1 field error(s) in invowkfile at /workspace/invowkfile.cue",
			wantCause:   ErrMissingDependencyAlternatives,
		},
		{
			name: "custom check dependency message detail",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{CustomChecks: []CustomCheckDependency{{}}}
			},
			wantField: "root custom_check #1 alternative #1",
			wantMessage: "invalid custom check dependency: 3 field error(s): " +
				"dependency alternatives must contain at least one item: " +
				"invalid check name \"\": must be non-empty and not whitespace-only: " +
				"invalid custom check script: 1 field error(s): " +
				"custom check script must set content or file in invowkfile at /workspace/invowkfile.cue",
			wantCause: ErrMissingDependencyAlternatives,
		},
		{
			name: "custom check dependency index",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{
					CustomChecks: []CustomCheckDependency{
						{Name: "ok", Script: CustomCheckScript{Content: "echo ok"}},
						{},
					},
				}
			},
			wantField: "root custom_check #2 alternative #1",
			wantMessage: "invalid custom check dependency: 3 field error(s): " +
				"dependency alternatives must contain at least one item: " +
				"invalid check name \"\": must be non-empty and not whitespace-only: " +
				"invalid custom check script: 1 field error(s): " +
				"custom check script must set content or file in invowkfile at /workspace/invowkfile.cue",
			wantCause: ErrMissingDependencyAlternatives,
		},
		{
			name: "custom check name length",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{
					CustomChecks: []CustomCheckDependency{{
						Name:   CheckName(strings.Repeat("n", MaxNameLength+1)),
						Script: CustomCheckScript{Content: "echo ok"},
					}},
				}
			},
			wantField:   "root custom_check #1 alternative #1",
			wantMessage: "custom_check name too long (257 chars, max 256) in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "custom check script content length",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.DependsOn = &DependsOn{
					CustomChecks: []CustomCheckDependency{{
						Name:   "long-content",
						Script: CustomCheckScript{Content: ScriptContent(strings.Repeat("s", MaxScriptLength+1))},
					}},
				}
			},
			wantField:   "root custom_check #1 alternative #1",
			wantMessage: "script.content too long (10485761 chars, max 10485760) in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "custom check script file length",
			mutate: func(t *testing.T, inv *Invowkfile) {
				t.Helper()

				inv.ModulePath = FilesystemPath(t.TempDir())
				file := ScriptFilePath(longScriptFile)
				inv.DependsOn = &DependsOn{
					CustomChecks: []CustomCheckDependency{{
						Name:   "long-file",
						Script: CustomCheckScript{File: &file},
					}},
				}
			},
			wantField: "root custom_check #1 alternative #1",
			wantMessage: fmt.Sprintf(
				"invalid custom check dependency: 1 field error(s): invalid custom check script: 1 field error(s): "+
					"invalid script file path %q: path too long (%d chars, max %d) in invowkfile at /workspace/invowkfile.cue",
				longScriptFile,
				len(longScriptFile),
				MaxPathLength,
			),
			wantCause: ErrInvalidScriptFilePath,
		},
		{
			name: "custom check script file requires module",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				file := ScriptFilePath("scripts/check.sh")
				inv.DependsOn = &DependsOn{
					CustomChecks: []CustomCheckDependency{{
						Name:   "module-check",
						Script: CustomCheckScript{File: &file},
					}},
				}
			},
			wantField:   "root custom_check #1 alternative #1 script file",
			wantMessage: "script file requires module invowkfile in invowkfile at /workspace/invowkfile.cue",
			wantCause:   ErrScriptFileRequiresModule,
		},
		{
			name: "command env file path",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Env = &EnvConfig{
					Files: []DotenvFilePath{"service.env", "../secret.env"},
				}
			},
			wantField:   "command 'deploy' env.files[2]",
			wantMessage: "env file path cannot contain '..': ../secret.env in invowkfile at /workspace/invowkfile.cue",
		},
		{
			name: "command env var value length",
			mutate: func(_ *testing.T, inv *Invowkfile) {
				inv.Commands[0].Env = &EnvConfig{
					Vars: map[EnvVarName]string{
						"TOKEN": strings.Repeat("v", MaxEnvVarValueLength+1),
					},
				}
			},
			wantField:   "command 'deploy' env.vars['TOKEN']",
			wantMessage: "value too long (32769 chars, max 32768) in invowkfile at /workspace/invowkfile.cue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inv := validationStructureDepsMutationInvowkfile()
			tt.mutate(t, inv)

			got := requireValidationStructureDepsIssue(t, inv.Validate(), tt.wantField, tt.wantMessage)
			if tt.wantCause != nil && !errors.Is(got.Cause, tt.wantCause) {
				t.Fatalf("validation cause = %v, want %v", got.Cause, tt.wantCause)
			}
		})
	}
}

func TestStructureDependsOnMutationEnvVarBoundary(t *testing.T) {
	t.Parallel()

	inv := validationStructureDepsMutationInvowkfile()
	inv.Commands[0].Env = &EnvConfig{
		Vars: map[EnvVarName]string{
			"TOKEN": strings.Repeat("v", MaxEnvVarValueLength),
		},
	}

	if errs := inv.Validate(); errs.HasErrors() {
		t.Fatalf("Validate() errors = %v, want none", errs)
	}
}

func TestStructureDependsOnMutationCustomCheckExpectedOutputHelper(t *testing.T) {
	t.Parallel()

	validator := NewStructureValidator()
	ctx := &ValidationContext{FilePath: validationStructureDepsMutationFile}
	check := CustomCheck{
		Name:           "output-check",
		Script:         CustomCheckScript{Content: "echo ok"},
		ExpectedOutput: "(a+)+",
	}

	errs := validator.validateCustomCheck(ctx, validationStructureDepsMutationInvowkfile(), check, NewFieldPath().Root().CustomCheck(0, 0))
	got := requireValidationStructureDepsIssue(
		t,
		errs,
		"root custom_check #1 alternative #1",
		"expected_output: nested quantifiers: regex pattern may cause performance issues in invowkfile at /workspace/invowkfile.cue",
	)
	if !errors.Is(got.Cause, ErrNestedQuantifiers) {
		t.Fatalf("validation cause = %v, want ErrNestedQuantifiers", got.Cause)
	}
}

func validationStructureDepsMutationInvowkfile() *Invowkfile {
	return &Invowkfile{
		FilePath: validationStructureDepsMutationFile,
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

func requireValidationStructureDepsIssue(
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
