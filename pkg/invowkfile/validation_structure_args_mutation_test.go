// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

const validationStructureArgsMutationFile FilesystemPath = "args.cue"

func TestStructureValidatorValidateArgsNoArgsReturnsNil(t *testing.T) {
	t.Parallel()

	cmd := structureArgMutationCommand()
	errs := NewStructureValidator().validateArgs(structureArgMutationContext(), &cmd)
	if errs != nil {
		t.Fatalf("validateArgs() = %v, want nil for commands without args", errs)
	}
}

func TestStructureValidatorValidateArgDiagnosticsContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		arg         Argument
		index       int
		wantField   string
		wantMessage []string
		wantCause   bool
	}{
		{
			name: "overlong name reports length error",
			arg: Argument{
				Name:        ArgumentName(strings.Repeat("a", MaxNameLength+1)),
				Description: "Too long",
			},
			wantField: "command 'deploy' argument '" + strings.Repeat("a", MaxNameLength+1) + "'",
			wantMessage: []string{
				"argument name",
				"too long",
				"args.cue",
			},
		},
		{
			name: "overlong description reports length error",
			arg: Argument{
				Name:        "target",
				Description: DescriptionText(strings.Repeat("a", MaxDescriptionLength+1)),
			},
			wantField: "command 'deploy' argument 'target'",
			wantMessage: []string{
				"argument description",
				"too long",
				"args.cue",
			},
		},
		{
			name: "unsafe regex preserves cause",
			arg: Argument{
				Name:        "target",
				Description: "Unsafe regex",
				Validation:  "[z-a]",
			},
			wantField: "command 'deploy' argument 'target'",
			wantMessage: []string{
				"unsafe validation regex",
				"invalid regex",
				"args.cue",
			},
			wantCause: true,
		},
		{
			name: "default value validation reports compatibility",
			arg: Argument{
				Name:         "count",
				Description:  "Retry count",
				Type:         ArgumentTypeInt,
				DefaultValue: "many",
			},
			wantField: "command 'deploy' argument 'count'",
			wantMessage: []string{
				"default_value",
				"not compatible with type",
				"args.cue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := validateSingleArgForMutation(tt.arg, tt.index)
			if len(errs) != 1 {
				t.Fatalf("validateArg() returned %d errors, want 1: %v", len(errs), errs)
			}
			assertStructureArgError(t, errs[0], tt.wantField, tt.wantMessage, tt.wantCause)
		})
	}
}

func TestStructureValidatorValidateArgsTracksDuplicateOrderingAndVariadicState(t *testing.T) {
	t.Parallel()

	cmd := structureArgMutationCommand()
	cmd.Args = []Argument{
		{Name: "target", Description: "Optional target"},
		{Name: "target", Description: "Required duplicate target", Required: true},
		{Name: "files", Description: "Files to upload", Variadic: true},
		{Name: "format", Description: "Output format"},
	}

	errs := NewStructureValidator().validateArgs(structureArgMutationContext(), &cmd)
	if len(errs) != 3 {
		t.Fatalf("validateArgs() returned %d errors, want 3: %v", len(errs), errs)
	}

	assertStructureArgError(t, errs[0], "command 'deploy'", []string{
		"duplicate argument name",
		"'target'",
		"args.cue",
	}, false)
	assertStructureArgError(t, errs[1], "command 'deploy' argument 'target'", []string{
		"required arguments must come before optional arguments",
		"args.cue",
	}, false)
	assertStructureArgError(t, errs[2], "command 'deploy' argument 'format'", []string{
		"only the last argument can be variadic",
		"args.cue",
	}, false)
}

func validateSingleArgForMutation(arg Argument, index int) []ValidationError {
	cmd := structureArgMutationCommand()
	errs, _, _ := NewStructureValidator().validateArg(
		structureArgMutationContext(),
		&cmd,
		&arg,
		index,
		make(map[string]bool),
		false,
		false,
	)
	return errs
}

func structureArgMutationCommand() Command {
	return Command{Name: "deploy", Description: "Deploy the service"}
}

func structureArgMutationContext() *ValidationContext {
	return &ValidationContext{FilePath: validationStructureArgsMutationFile}
}

func assertStructureArgError(t *testing.T, err ValidationError, wantField string, wantMessage []string, wantCause bool) {
	t.Helper()

	if err.Validator != structureValidatorName {
		t.Fatalf("Validator = %q, want %q", err.Validator, structureValidatorName)
	}
	if err.Field != wantField {
		t.Fatalf("Field = %q, want %q", err.Field, wantField)
	}
	for _, part := range wantMessage {
		if !strings.Contains(err.Message, part) {
			t.Fatalf("Message = %q, want substring %q", err.Message, part)
		}
	}
	if err.Severity != SeverityError {
		t.Fatalf("Severity = %v, want %v", err.Severity, SeverityError)
	}
	if (err.Cause != nil) != wantCause {
		t.Fatalf("Cause = %v, want present=%v", err.Cause, wantCause)
	}
}
