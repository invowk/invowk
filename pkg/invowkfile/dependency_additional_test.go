// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

type commandDependencyRefParseTestCase struct {
	name          string
	ref           CommandDependencyRef
	wantQualified bool
	wantSource    CommandDependencySourceID
	wantCommand   CommandName
	wantErr       bool
}

func TestDependencyValidators_InvalidCases(t *testing.T) {
	t.Parallel()

	invalidCode := types.ExitCode(-1)

	tests := []struct {
		name     string
		err      error
		sentinel error
		checkAs  func(t *testing.T, err error)
	}{
		{
			name:     "tool dependency",
			err:      ToolDependency{Alternatives: []BinaryName{""}}.Validate(),
			sentinel: ErrInvalidToolDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidToolDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name:     "command dependency",
			err:      CommandDependency{Alternatives: []CommandDependencyRef{""}}.Validate(),
			sentinel: ErrInvalidCommandDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidCommandDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name:     "capability dependency",
			err:      CapabilityDependency{Alternatives: []CapabilityName{"bogus"}}.Validate(),
			sentinel: ErrInvalidCapabilityDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidCapabilityDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name:     "env var check",
			err:      EnvVarCheck{Name: "", Validation: "["}.Validate(),
			sentinel: ErrInvalidEnvVarCheck,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidEnvVarCheckError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name:     "env var dependency",
			err:      EnvVarDependency{Alternatives: []EnvVarCheck{{Name: ""}}}.Validate(),
			sentinel: ErrInvalidEnvVarDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidEnvVarDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name:     "filepath dependency",
			err:      FilepathDependency{Alternatives: []FilesystemPath{""}}.Validate(),
			sentinel: ErrInvalidFilepathDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidFilepathDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name: "custom check",
			err: CustomCheck{
				Name:           "",
				Script:         CustomCheckScript{Content: "   "},
				ExpectedCode:   &invalidCode,
				ExpectedOutput: "[",
			}.Validate(),
			sentinel: ErrInvalidCustomCheck,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidCustomCheckError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name: "custom check dependency direct fields",
			err: CustomCheckDependency{
				Name:           "",
				Script:         CustomCheckScript{Content: "   "},
				ExpectedCode:   &invalidCode,
				ExpectedOutput: "[",
			}.Validate(),
			sentinel: ErrInvalidCustomCheckDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidCustomCheckDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name: "custom check dependency alternatives",
			err: CustomCheckDependency{
				Alternatives: []CustomCheck{{Name: "", Script: CustomCheckScript{Content: "   "}}},
			}.Validate(),
			sentinel: ErrInvalidCustomCheckDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidCustomCheckDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name: "depends_on",
			err: DependsOn{
				Tools:        []ToolDependency{{Alternatives: []BinaryName{""}}},
				Commands:     []CommandDependency{{Alternatives: []CommandDependencyRef{""}}},
				Filepaths:    []FilepathDependency{{Alternatives: []FilesystemPath{""}}},
				Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{"bogus"}}},
				CustomChecks: []CustomCheckDependency{{Name: "", Script: CustomCheckScript{Content: "   "}}},
				EnvVars:      []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: ""}}}},
			}.Validate(),
			sentinel: ErrInvalidDependsOn,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidDependsOnError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err == nil {
				t.Fatal("Validate() returned nil, want error")
			}
			if !errors.Is(tt.err, tt.sentinel) {
				t.Fatalf("errors.Is(%v, %v) = false", tt.err, tt.sentinel)
			}
			tt.checkAs(t, tt.err)
		})
	}
}

func TestInvalidDependsOnError_UnwrapsFieldErrors(t *testing.T) {
	t.Parallel()

	err := DependsOn{
		Tools: []ToolDependency{{Alternatives: []BinaryName{""}}},
	}.Validate()
	if err == nil {
		t.Fatal("DependsOn.Validate() error = nil, want tool dependency error")
	}
	if !errors.Is(err, ErrInvalidDependsOn) {
		t.Fatalf("errors.Is(%v, ErrInvalidDependsOn) = false", err)
	}
	if !errors.Is(err, ErrInvalidToolDependency) {
		t.Fatalf("errors.Is(%v, ErrInvalidToolDependency) = false", err)
	}
}

func TestDependencyValidators_ValidCases(t *testing.T) {
	t.Parallel()

	validCode := types.ExitCode(0)
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "tool dependency",
			err:  ToolDependency{Alternatives: []BinaryName{"git"}}.Validate(),
		},
		{
			name: "command dependency",
			err:  CommandDependency{Alternatives: []CommandDependencyRef{"build"}}.Validate(),
		},
		{
			name: "capability dependency",
			err:  CapabilityDependency{Alternatives: []CapabilityName{CapabilityInternet}}.Validate(),
		},
		{
			name: "env var check",
			err:  EnvVarCheck{Name: "PATH", Validation: "^.+$"}.Validate(),
		},
		{
			name: "env var dependency",
			err:  EnvVarDependency{Alternatives: []EnvVarCheck{{Name: "PATH"}}}.Validate(),
		},
		{
			name: "filepath dependency",
			err:  FilepathDependency{Alternatives: []FilesystemPath{"scripts/install.sh"}}.Validate(),
		},
		{
			name: "custom check",
			err: CustomCheck{
				Name:           "shellcheck",
				Script:         CustomCheckScript{Content: "echo ok"},
				ExpectedCode:   &validCode,
				ExpectedOutput: "^ok$",
			}.Validate(),
		},
		{
			name: "custom check dependency direct",
			err: CustomCheckDependency{
				Name:           "shellcheck",
				Script:         CustomCheckScript{Content: "echo ok"},
				ExpectedCode:   &validCode,
				ExpectedOutput: "^ok$",
			}.Validate(),
		},
		{
			name: "custom check dependency alternatives",
			err: CustomCheckDependency{
				Alternatives: []CustomCheck{{Name: "shellcheck", Script: CustomCheckScript{Content: "echo ok"}}},
			}.Validate(),
		},
		{
			name: "depends_on",
			err: DependsOn{
				Tools:        []ToolDependency{{Alternatives: []BinaryName{"git"}}},
				Commands:     []CommandDependency{{Alternatives: []CommandDependencyRef{"build"}}},
				Filepaths:    []FilepathDependency{{Alternatives: []FilesystemPath{"scripts/install.sh"}}},
				Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityTTY}}},
				CustomChecks: []CustomCheckDependency{{Name: "shellcheck", Script: CustomCheckScript{Content: "echo ok"}}},
				EnvVars:      []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "PATH"}}}},
			}.Validate(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err != nil {
				t.Fatalf("Validate() error = %v, want nil", tt.err)
			}
		})
	}
}

func TestCommandDependencyRef_Parse(t *testing.T) {
	t.Parallel()

	tests := []commandDependencyRefParseTestCase{
		{name: "bare", ref: "build", wantCommand: "build"},
		{name: "bare with spaces", ref: "test unit", wantCommand: "test unit"},
		{name: "qualified", ref: "@tools lint", wantQualified: true, wantSource: "tools", wantCommand: "lint"},
		{name: "qualified dotted source", ref: "@com.company.tools test unit", wantQualified: true, wantSource: "com.company.tools", wantCommand: "test unit"},
		{name: "empty", ref: "", wantErr: true},
		{name: "missing command", ref: "@tools", wantErr: true},
		{name: "invalid source", ref: "@9tools lint", wantErr: true},
		{name: "invalid command", ref: "@tools 9lint", wantErr: true},
		{name: "old dotted prefix syntax", ref: "com.company.tools lint", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertCommandDependencyRefParse(t, tt)
		})
	}
}

func assertCommandDependencyRefParse(t *testing.T, tt commandDependencyRefParseTestCase) {
	t.Helper()

	got, err := tt.ref.Parse()
	if tt.wantErr {
		assertInvalidCommandDependencyRefParse(t, err)
		return
	}
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Qualified != tt.wantQualified || got.SourceID != tt.wantSource || got.Command != tt.wantCommand {
		t.Fatalf("Parse() = %+v, want qualified=%v source=%q command=%q", got, tt.wantQualified, tt.wantSource, tt.wantCommand)
	}
}

func assertInvalidCommandDependencyRefParse(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !errors.Is(err, ErrInvalidCommandDependencyRef) {
		t.Fatalf("Parse() error = %v, want ErrInvalidCommandDependencyRef", err)
	}
}

func TestCustomCheckDependencyValidateShape(t *testing.T) {
	t.Parallel()

	validCode := types.ExitCode(0)
	tests := []struct {
		name    string
		dep     CustomCheckDependency
		wantErr error
	}{
		{
			name: "direct check requires script",
			dep: CustomCheckDependency{
				Name: "shellcheck",
			},
			wantErr: ErrMissingCustomCheckScriptSource,
		},
		{
			name: "alternative check requires script",
			dep: CustomCheckDependency{
				Alternatives: []CustomCheck{{Name: "shellcheck"}},
			},
			wantErr: ErrMissingCustomCheckScriptSource,
		},
		{
			name: "alternatives reject direct fields",
			dep: CustomCheckDependency{
				Name:   "direct",
				Script: CustomCheckScript{Content: "true"},
				Alternatives: []CustomCheck{
					{Name: "alt", Script: CustomCheckScript{Content: "true"}},
				},
			},
			wantErr: ErrMixedCustomCheckDependency,
		},
		{
			name: "alternatives reject direct expected code",
			dep: CustomCheckDependency{
				ExpectedCode: &validCode,
				Alternatives: []CustomCheck{
					{Name: "alt", Script: CustomCheckScript{Content: "true"}},
				},
			},
			wantErr: ErrMixedCustomCheckDependency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.dep.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDependencyValidators_EmptyAlternativesInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		sentinel error
	}{
		{"tool", ToolDependency{}.Validate(), ErrInvalidToolDependency},
		{"command", CommandDependency{}.Validate(), ErrInvalidCommandDependency},
		{"capability", CapabilityDependency{}.Validate(), ErrInvalidCapabilityDependency},
		{"env var", EnvVarDependency{}.Validate(), ErrInvalidEnvVarDependency},
		{"filepath", FilepathDependency{}.Validate(), ErrInvalidFilepathDependency},
		{"custom check alternative shape", CustomCheckDependency{}.Validate(), ErrInvalidCustomCheckDependency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err == nil {
				t.Fatal("Validate() returned nil, want error")
			}
			if !errors.Is(tt.err, tt.sentinel) {
				t.Fatalf("errors.Is(%v, %v) = false", tt.err, tt.sentinel)
			}
			if !errors.Is(tt.err, ErrMissingDependencyAlternatives) {
				t.Fatalf("errors.Is(%v, ErrMissingDependencyAlternatives) = false", tt.err)
			}
		})
	}
}

func TestDependencyErrorStringsAndUnwrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		sentinel  error
		wantError string
	}{
		{
			name:      "invalid binary name",
			err:       &InvalidBinaryNameError{Value: "/bin/sh", Reason: "must not contain path separators"},
			sentinel:  ErrInvalidBinaryName,
			wantError: `invalid binary name "/bin/sh": must not contain path separators`,
		},
		{
			name:      "invalid check name",
			err:       &InvalidCheckNameError{Value: ""},
			sentinel:  ErrInvalidCheckName,
			wantError: `invalid check name "": must be non-empty and not whitespace-only`,
		},
		{
			name:      "invalid script content",
			err:       &InvalidScriptContentError{Value: "   "},
			sentinel:  ErrInvalidScriptContent,
			wantError: `invalid script content: non-empty value must not be whitespace-only (got "   ")`,
		},
		{
			name:      "invalid tool dependency",
			err:       &InvalidToolDependencyError{FieldErrors: []error{errors.New("one"), errors.New("two")}},
			sentinel:  ErrInvalidToolDependency,
			wantError: "invalid tool dependency: 2 field error(s)",
		},
		{
			name:      "invalid command dependency",
			err:       &InvalidCommandDependencyError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidCommandDependency,
			wantError: "invalid command dependency: 1 field error(s)",
		},
		{
			name:      "invalid capability dependency",
			err:       &InvalidCapabilityDependencyError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidCapabilityDependency,
			wantError: "invalid capability dependency: 1 field error(s)",
		},
		{
			name:      "invalid env var check",
			err:       &InvalidEnvVarCheckError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidEnvVarCheck,
			wantError: "invalid env var check: 1 field error(s)",
		},
		{
			name:      "invalid env var dependency",
			err:       &InvalidEnvVarDependencyError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidEnvVarDependency,
			wantError: "invalid env var dependency: 1 field error(s)",
		},
		{
			name:      "invalid filepath dependency",
			err:       &InvalidFilepathDependencyError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidFilepathDependency,
			wantError: "invalid filepath dependency: 1 field error(s)",
		},
		{
			name:      "invalid custom check",
			err:       &InvalidCustomCheckError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidCustomCheck,
			wantError: "invalid custom check: 1 field error(s)",
		},
		{
			name:      "invalid custom check dependency",
			err:       &InvalidCustomCheckDependencyError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidCustomCheckDependency,
			wantError: "invalid custom check dependency: 1 field error(s)",
		},
		{
			name:      "invalid depends_on",
			err:       &InvalidDependsOnError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidDependsOn,
			wantError: "invalid depends_on: 1 field error(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err.Error() != tt.wantError {
				t.Fatalf("Error() = %q, want %q", tt.err.Error(), tt.wantError)
			}
			if !errors.Is(tt.err, tt.sentinel) {
				t.Fatalf("errors.Is(%v, %v) = false", tt.err, tt.sentinel)
			}
		})
	}
}
