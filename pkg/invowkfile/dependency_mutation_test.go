// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/types"
)

func TestDependencyValueErrorPayloads(t *testing.T) {
	t.Parallel()

	t.Run("binary name validation preserves value and reason", testBinaryNameValidationPayloads)
	t.Run("check name and script content validation preserve values", testCheckNameAndScriptContentPayloads)
	t.Run("source id validation preserves value and branch-specific reason", testSourceIDValidationPayloads)
}

func TestCommandDependencyRefMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("valid refs preserve parsed fields and rendering", testValidCommandDependencyRefs)
	t.Run("invalid refs preserve original ref and reason", testInvalidCommandDependencyRefs)
	t.Run("parts validation rejects inconsistent structured refs", testCommandDependencyRefParts)
	t.Run("default error reasons are user-facing", testCommandDependencyDefaultErrorReasons)
}

func testBinaryNameValidationPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      BinaryName
		wantReason string
	}{
		{name: "empty", value: "", wantReason: "must not be empty or whitespace-only"},
		{name: "too long", value: BinaryName(strings.Repeat("a", MaxNameLength+1)), wantReason: "exceeds maximum length of 256 runes"},
		{name: "path separator", value: "bin/tool", wantReason: "must not contain path separators"},
		{name: "invalid start", value: "-tool", wantReason: "must start with an alphanumeric character and contain only alphanumeric characters, '.', '_', '+', or '-'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			typed := requireDependencyMutationAs[*InvalidBinaryNameError](t, tt.value.Validate())
			if typed.Value != tt.value {
				t.Fatalf("Value = %q, want %q", typed.Value, tt.value)
			}
			if typed.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", typed.Reason, tt.wantReason)
			}
		})
	}
}

func testCheckNameAndScriptContentPayloads(t *testing.T) {
	t.Parallel()

	checkErr := requireDependencyMutationAs[*InvalidCheckNameError](t, CheckName(" \t").Validate())
	if checkErr.Value != " \t" {
		t.Fatalf("check name Value = %q, want original whitespace", checkErr.Value)
	}

	contentErr := requireDependencyMutationAs[*InvalidScriptContentError](t, ScriptContent("\n\t").Validate())
	if contentErr.Value != "\n\t" {
		t.Fatalf("script content Value = %q, want original whitespace", contentErr.Value)
	}
}

func testSourceIDValidationPayloads(t *testing.T) {
	t.Parallel()

	tooLong := CommandDependencySourceID(strings.Repeat("a", MaxNameLength+1))
	tests := []struct {
		name       string
		value      CommandDependencySourceID
		wantReason string
	}{
		{name: "empty", value: "", wantReason: invalidReasonMustNotBeEmpty},
		{name: "too long", value: tooLong, wantReason: "exceeds maximum length of 256 chars"},
		{name: "invalid start", value: "9tools", wantReason: "must start with a letter and contain only letters, digits, dots, underscores, or hyphens"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			typed := requireDependencyMutationAs[*InvalidCommandDependencySourceIDError](t, tt.value.Validate())
			if typed.Value != tt.value {
				t.Fatalf("Value = %q, want %q", typed.Value, tt.value)
			}
			if typed.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", typed.Reason, tt.wantReason)
			}
		})
	}
}

func testValidCommandDependencyRefs(t *testing.T) {
	t.Parallel()

	bare, err := CommandDependencyRef("build test").Parse()
	if err != nil {
		t.Fatalf("bare Parse() error = %v", err)
	}
	if bare.Qualified || bare.SourceID != "" || bare.Command != "build test" || bare.String() != "build test" {
		t.Fatalf("bare parts = %+v", bare)
	}

	qualified, err := CommandDependencyRef("@tools lint").Parse()
	if err != nil {
		t.Fatalf("qualified Parse() error = %v", err)
	}
	if !qualified.Qualified || qualified.SourceID != "tools" || qualified.Command != "lint" || qualified.String() != "@tools lint" {
		t.Fatalf("qualified parts = %+v", qualified)
	}
}

func testInvalidCommandDependencyRefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ref        CommandDependencyRef
		wantReason string
	}{
		{name: "empty", ref: "", wantReason: invalidReasonMustNotBeEmpty},
		{name: "invalid bare command", ref: "9build", wantReason: "expected bare command name or @source command reference"},
		{name: "qualified without separator", ref: "@tools", wantReason: "qualified references must use @source command"},
		{name: "qualified without command", ref: "@tools ", wantReason: "qualified references must include a command name after the source"},
		{name: "qualified invalid source", ref: "@9tools lint", wantReason: "invalid command dependency source id"},
		{name: "qualified invalid command", ref: "@tools 9lint", wantReason: "invalid command name after source"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.ref.Parse()
			typed := requireDependencyMutationAs[*InvalidCommandDependencyRefError](t, err)
			if typed.Value != tt.ref {
				t.Fatalf("Value = %q, want %q", typed.Value, tt.ref)
			}
			if !strings.Contains(typed.Reason, tt.wantReason) {
				t.Fatalf("Reason = %q, want containing %q", typed.Reason, tt.wantReason)
			}
		})
	}
}

func testCommandDependencyRefParts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		parts      CommandDependencyRefParts
		wantValue  CommandDependencyRef
		wantReason string
	}{
		{name: "invalid command", parts: CommandDependencyRefParts{Command: "9build"}, wantValue: "9build", wantReason: "invalid command name"},
		{name: "bare with source", parts: CommandDependencyRefParts{Command: "build", SourceID: "tools"}, wantValue: "build", wantReason: "bare references must not include a source"},
		{name: "qualified invalid source", parts: CommandDependencyRefParts{Command: "build", SourceID: "9tools", Qualified: true}, wantValue: "@9tools build", wantReason: "invalid command dependency source id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			typed := requireDependencyMutationAs[*InvalidCommandDependencyRefError](t, tt.parts.Validate())
			if typed.Value != tt.wantValue {
				t.Fatalf("Value = %q, want %q", typed.Value, tt.wantValue)
			}
			if !strings.Contains(typed.Reason, tt.wantReason) {
				t.Fatalf("Reason = %q, want containing %q", typed.Reason, tt.wantReason)
			}
		})
	}
}

func testCommandDependencyDefaultErrorReasons(t *testing.T) {
	t.Parallel()

	refErr := &InvalidCommandDependencyRefError{Value: "bad"}
	if !strings.Contains(refErr.Error(), "expected bare command name or @source command reference") {
		t.Fatalf("InvalidCommandDependencyRefError.Error() = %q", refErr.Error())
	}
	sourceErr := &InvalidCommandDependencySourceIDError{Value: "9bad"}
	if !strings.Contains(sourceErr.Error(), "must start with a letter") {
		t.Fatalf("InvalidCommandDependencySourceIDError.Error() = %q", sourceErr.Error())
	}
}

func TestDependencyFieldErrorPayloads(t *testing.T) {
	t.Parallel()

	t.Run("simple dependency validators preserve nested field errors", func(t *testing.T) {
		t.Parallel()

		toolErr := requireDependencyMutationAs[*InvalidToolDependencyError](t, ToolDependency{Alternatives: []BinaryName{""}}.Validate())
		requireDependencyMutationFieldErrors(t, toolErr.FieldErrors, 1, ErrInvalidBinaryName)

		commandErr := requireDependencyMutationAs[*InvalidCommandDependencyError](t, CommandDependency{Alternatives: []CommandDependencyRef{""}}.Validate())
		requireDependencyMutationFieldErrors(t, commandErr.FieldErrors, 1, ErrInvalidCommandDependencyRef)

		capabilityErr := requireDependencyMutationAs[*InvalidCapabilityDependencyError](t, CapabilityDependency{Alternatives: []CapabilityName{"bogus"}}.Validate())
		requireDependencyMutationFieldErrors(t, capabilityErr.FieldErrors, 1, ErrInvalidCapabilityName)

		envCheckErr := requireDependencyMutationAs[*InvalidEnvVarCheckError](t, EnvVarCheck{Name: "", Validation: "["}.Validate())
		requireDependencyMutationFieldErrors(t, envCheckErr.FieldErrors, 2, ErrInvalidEnvVarName, ErrInvalidRegexPattern)

		envDepErr := requireDependencyMutationAs[*InvalidEnvVarDependencyError](t, EnvVarDependency{Alternatives: []EnvVarCheck{{Name: ""}}}.Validate())
		requireDependencyMutationFieldErrors(t, envDepErr.FieldErrors, 1, ErrInvalidEnvVarCheck)

		pathErr := requireDependencyMutationAs[*InvalidFilepathDependencyError](t, FilepathDependency{Alternatives: []FilesystemPath{""}}.Validate())
		requireDependencyMutationFieldErrors(t, pathErr.FieldErrors, 1, ErrInvalidFilesystemPath)
	})

	t.Run("custom check validators preserve every nested invalid field", func(t *testing.T) {
		t.Parallel()

		invalidCode := types.ExitCode(-1)
		checkErr := requireDependencyMutationAs[*InvalidCustomCheckError](t, CustomCheck{
			Name:           "",
			Script:         CustomCheckScript{Content: " \t"},
			ExpectedCode:   &invalidCode,
			ExpectedOutput: "[",
		}.Validate())
		requireDependencyMutationFieldErrors(
			t,
			checkErr.FieldErrors,
			4,
			ErrInvalidCheckName,
			ErrInvalidCustomCheckScript,
			types.ErrInvalidExitCode,
			ErrInvalidRegexPattern,
		)

		depErr := requireDependencyMutationAs[*InvalidCustomCheckDependencyError](t, CustomCheckDependency{
			Name:           "",
			Script:         CustomCheckScript{Content: " \t"},
			ExpectedCode:   &invalidCode,
			ExpectedOutput: "[",
		}.Validate())
		requireDependencyMutationFieldErrors(
			t,
			depErr.FieldErrors,
			4,
			ErrInvalidCheckName,
			ErrInvalidCustomCheckScript,
			types.ErrInvalidExitCode,
			ErrInvalidRegexPattern,
		)

		altErr := requireDependencyMutationAs[*InvalidCustomCheckDependencyError](t, CustomCheckDependency{
			Name: "direct",
			Alternatives: []CustomCheck{{
				Name:   "",
				Script: CustomCheckScript{Content: " \t"},
			}},
		}.Validate())
		requireDependencyMutationFieldErrors(t, altErr.FieldErrors, 2, ErrMixedCustomCheckDependency, ErrInvalidCustomCheck)
	})

	t.Run("depends_on preserves category errors in declaration order", func(t *testing.T) {
		t.Parallel()

		err := DependsOn{
			Tools:        []ToolDependency{{Alternatives: []BinaryName{""}}},
			Commands:     []CommandDependency{{Alternatives: []CommandDependencyRef{""}}},
			Filepaths:    []FilepathDependency{{Alternatives: []FilesystemPath{""}}},
			Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{"bogus"}}},
			CustomChecks: []CustomCheckDependency{{Name: "", Script: CustomCheckScript{Content: " \t"}}},
			EnvVars:      []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: ""}}}},
		}.Validate()
		typed := requireDependencyMutationAs[*InvalidDependsOnError](t, err)
		requireDependencyMutationFieldErrors(
			t,
			typed.FieldErrors,
			6,
			ErrInvalidToolDependency,
			ErrInvalidCommandDependency,
			ErrInvalidFilepathDependency,
			ErrInvalidCapabilityDependency,
			ErrInvalidCustomCheckDependency,
			ErrInvalidEnvVarDependency,
		)
	})
}

func TestCustomCheckScriptMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("script source helpers distinguish content and file", testCustomCheckScriptSourceHelpers)
	t.Run("script file paths resolve relative and absolute forms", testCustomCheckScriptFilePaths)
	t.Run("script validation preserves source and optional field failures", testCustomCheckScriptValidationFailures)
	t.Run("resolve validates source shape before file IO", testCustomCheckScriptResolveValidatesSource)
}

func testCustomCheckScriptSourceHelpers(t *testing.T) {
	t.Parallel()

	file := FilesystemPath("scripts/check.sh")
	contentScript := CustomCheckScript{Content: "echo ok"}
	fileScript := CustomCheckScript{File: &file}

	if !contentScript.IsContent() || contentScript.IsFile() || contentScript.hasSource() != true {
		t.Fatalf("content script helpers returned IsContent=%v IsFile=%v hasSource=%v", contentScript.IsContent(), contentScript.IsFile(), contentScript.hasSource())
	}
	if fileScript.IsContent() || !fileScript.IsFile() || fileScript.hasSource() != true {
		t.Fatalf("file script helpers returned IsContent=%v IsFile=%v hasSource=%v", fileScript.IsContent(), fileScript.IsFile(), fileScript.hasSource())
	}
	if (CustomCheckScript{}).hasSource() {
		t.Fatal("empty script hasSource() = true, want false")
	}
}

func testCustomCheckScriptFilePaths(t *testing.T) {
	t.Parallel()

	modulePath := FilesystemPath(filepath.Join(t.TempDir(), "module.invowkmod"))
	relative := FilesystemPath("scripts/check.sh")
	relativeScript := CustomCheckScript{File: &relative}
	wantRelative := fspath.JoinStr(modulePath, filepath.FromSlash("scripts/check.sh"))
	if got := relativeScript.GetScriptFilePathWithModule(modulePath); got != wantRelative {
		t.Fatalf("relative script path = %q, want %q", got, wantRelative)
	}
	if got := relativeScript.GetScriptFilePathWithModule(""); got != "" {
		t.Fatalf("relative script with empty module = %q, want empty", got)
	}

	absolute := FilesystemPath(filepath.Join(t.TempDir(), "check.sh"))
	absoluteScript := CustomCheckScript{File: &absolute}
	if got := absoluteScript.GetScriptFilePathWithModule(modulePath); got != absolute {
		t.Fatalf("absolute script path = %q, want %q", got, absolute)
	}
	if got := (CustomCheckScript{Content: "echo ok"}).GetScriptFilePathWithModule(modulePath); got != "" {
		t.Fatalf("inline script path = %q, want empty", got)
	}
}

func testCustomCheckScriptValidationFailures(t *testing.T) {
	t.Parallel()

	file := FilesystemPath("")
	err := CustomCheckScript{
		Content:     " \t",
		File:        &file,
		Interpreter: " \t",
	}.Validate()
	typed := requireDependencyMutationAs[*InvalidCustomCheckScriptError](t, err)
	requireDependencyMutationFieldErrors(
		t,
		typed.FieldErrors,
		4,
		ErrMixedCustomCheckScriptSource,
		ErrInvalidScriptContent,
		ErrInvalidFilesystemPath,
		ErrInvalidInterpreterSpec,
	)

	missing := requireDependencyMutationAs[*InvalidCustomCheckScriptError](t, (CustomCheckScript{}).Validate())
	requireDependencyMutationFieldErrors(t, missing.FieldErrors, 1, ErrMissingCustomCheckScriptSource)
}

func testCustomCheckScriptResolveValidatesSource(t *testing.T) {
	t.Parallel()

	readCalled := false
	_, err := CustomCheckScript{}.ResolveWithFSAndModule("module.invowkmod", func(string) ([]byte, error) {
		readCalled = true
		return nil, errors.New("should not read")
	})
	if !errors.Is(err, ErrMissingCustomCheckScriptSource) {
		t.Fatalf("ResolveWithFSAndModule() error = %v, want ErrMissingCustomCheckScriptSource", err)
	}
	if readCalled {
		t.Fatal("ResolveWithFSAndModule read file before validating script source")
	}

	file := FilesystemPath("scripts/check.sh")
	_, err = CustomCheckScript{File: &file}.ResolveWithFSAndModule("", nil)
	if !errors.Is(err, ErrScriptFileRequiresModule) {
		t.Fatalf("file script without module error = %v, want ErrScriptFileRequiresModule", err)
	}
	_, err = CustomCheckScript{File: &file}.ResolveWithFSAndModule("module.invowkmod", nil)
	if !errors.Is(err, ErrScriptReaderRequired) {
		t.Fatalf("file script without reader error = %v, want ErrScriptReaderRequired", err)
	}
}

func TestCustomCheckDependencyMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("direct get checks preserves every field", func(t *testing.T) {
		t.Parallel()

		expectedCode := types.ExitCode(7)
		dep := CustomCheckDependency{
			Name:           "direct",
			Script:         CustomCheckScript{Content: "echo direct", Interpreter: "sh"},
			ExpectedCode:   &expectedCode,
			ExpectedOutput: "^direct$",
		}
		checks := dep.GetChecks()
		if len(checks) != 1 {
			t.Fatalf("GetChecks() returned %d checks, want 1", len(checks))
		}
		got := checks[0]
		if got.Name != dep.Name || got.Script != dep.Script || got.ExpectedCode != dep.ExpectedCode || got.ExpectedOutput != dep.ExpectedOutput {
			t.Fatalf("GetChecks()[0] = %+v, want direct fields from %+v", got, dep)
		}
	})

	t.Run("alternatives get checks returns original alternatives", func(t *testing.T) {
		t.Parallel()

		dep := CustomCheckDependency{
			Alternatives: []CustomCheck{
				{Name: "one", Script: CustomCheckScript{Content: "echo one"}},
				{Name: "two", Script: CustomCheckScript{Content: "echo two"}},
			},
		}
		checks := dep.GetChecks()
		if len(checks) != len(dep.Alternatives) || checks[0].Name != "one" || checks[1].Name != "two" {
			t.Fatalf("GetChecks() = %+v, want original alternatives", checks)
		}
	})

	t.Run("direct dependency validates optional expected output only when set", func(t *testing.T) {
		t.Parallel()

		err := CustomCheckDependency{
			Name:           "regex",
			Script:         CustomCheckScript{Content: "echo ok"},
			ExpectedOutput: "[",
		}.Validate()
		typed := requireDependencyMutationAs[*InvalidCustomCheckDependencyError](t, err)
		requireDependencyMutationFieldErrors(t, typed.FieldErrors, 1, ErrInvalidRegexPattern)

		if err := (CustomCheckDependency{
			Name:   "empty-regex",
			Script: CustomCheckScript{Content: "echo ok"},
		}).Validate(); err != nil {
			t.Fatalf("empty ExpectedOutput Validate() error = %v, want nil", err)
		}
	})
}

func requireDependencyMutationAs[T any](t *testing.T, err error) T {
	t.Helper()

	var target T
	if !errors.As(err, &target) {
		t.Fatalf("errors.As(%T) = false for %v", target, err)
	}
	return target
}

func requireDependencyMutationFieldErrors(t *testing.T, fieldErrors []error, wantLen int, sentinels ...error) {
	t.Helper()

	if len(fieldErrors) != wantLen {
		t.Fatalf("FieldErrors = %v, want %d entries", fieldErrors, wantLen)
	}
	for _, sentinel := range sentinels {
		if !dependencyMutationErrorsContain(fieldErrors, sentinel) {
			t.Fatalf("FieldErrors = %v, want error wrapping %v", fieldErrors, sentinel)
		}
	}
}

func dependencyMutationErrorsContain(errs []error, sentinel error) bool {
	for _, err := range errs {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}
