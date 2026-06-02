// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

func TestStructureValidatorValidateFlagDiagnosticsContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		flag        Flag
		index       int
		wantField   string
		wantMessage []string
		wantCause   bool
	}{
		{
			name:      "empty name uses indexed flag path",
			flag:      Flag{Description: "Needs a name"},
			index:     2,
			wantField: "command 'deploy' flag #3",
			wantMessage: []string{
				"must have a name",
				"flags.cue",
			},
		},
		{
			name: "overlong name reports length error",
			flag: Flag{
				Name:        FlagName(strings.Repeat("a", MaxNameLength+1)),
				Description: "Too long",
			},
			wantField: "command 'deploy' flag '" + strings.Repeat("a", MaxNameLength+1) + "'",
			wantMessage: []string{
				"flag name",
				"too long",
				"flags.cue",
			},
		},
		{
			name: "blank description reports non-empty requirement",
			flag: Flag{
				Name:        "target",
				Description: " \t\n ",
			},
			wantField: "command 'deploy' flag 'target'",
			wantMessage: []string{
				"must have a non-empty description",
				"flags.cue",
			},
		},
		{
			name: "overlong description reports length error",
			flag: Flag{
				Name:        "target",
				Description: DescriptionText(strings.Repeat("a", MaxDescriptionLength+1)),
			},
			wantField: "command 'deploy' flag 'target'",
			wantMessage: []string{
				"flag description",
				"too long",
				"flags.cue",
			},
		},
		{
			name: "ivk prefix reports reserved namespace",
			flag: Flag{
				Name:        "ivk-custom",
				Description: "Reserved prefix",
			},
			wantField: "command 'deploy' flag 'ivk-custom'",
			wantMessage: []string{
				"ivk-",
				"reserved for system flags",
				"flags.cue",
			},
		},
		{
			name: "invowk prefix reports reserved namespace",
			flag: Flag{
				Name:        "invowk-custom",
				Description: "Reserved prefix",
			},
			wantField: "command 'deploy' flag 'invowk-custom'",
			wantMessage: []string{
				"invowk-",
				"reserved for system flags",
				"flags.cue",
			},
		},
		{
			name: "i prefix reports reserved namespace",
			flag: Flag{
				Name:        "i-custom",
				Description: "Reserved prefix",
			},
			wantField: "command 'deploy' flag 'i-custom'",
			wantMessage: []string{
				"i-",
				"reserved for system flags",
				"flags.cue",
			},
		},
		{
			name: "reserved flag name reports long flag",
			flag: Flag{
				Name:        "help",
				Description: "Reserved name",
			},
			wantField: "command 'deploy' flag 'help'",
			wantMessage: []string{
				"'help'",
				"reserved system flag",
				"flags.cue",
			},
		},
		{
			name: "reserved short alias reports owning system flag",
			flag: Flag{
				Name:        "target",
				Description: "Reserved short",
				Short:       "w",
			},
			wantField: "command 'deploy' flag 'target'",
			wantMessage: []string{
				"short alias 'w'",
				"--ivk-workdir",
				"flags.cue",
			},
		},
		{
			name: "unsafe regex preserves cause",
			flag: Flag{
				Name:        "target",
				Description: "Unsafe regex",
				Validation:  "[z-a]",
			},
			wantField: "command 'deploy' flag 'target'",
			wantMessage: []string{
				"unsafe validation regex",
				"invalid regex",
				"flags.cue",
			},
			wantCause: true,
		},
		{
			name: "default value validation reports compatibility",
			flag: Flag{
				Name:         "dry-run",
				Description:  "Boolean flag",
				Type:         FlagTypeBool,
				DefaultValue: "not-bool",
			},
			wantField: "command 'deploy' flag 'dry-run'",
			wantMessage: []string{
				"default_value",
				"not compatible with type",
				"flags.cue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := validateSingleFlagForMutation(tt.flag, tt.index)
			if len(errs) != 1 {
				t.Fatalf("validateFlag() returned %d errors, want 1: %v", len(errs), errs)
			}
			assertStructureFlagError(t, errs[0], tt.wantField, tt.wantMessage, tt.wantCause)
		})
	}
}

func TestStructureValidatorValidateFlagShortAliasBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		short     FlagShorthand
		wantError bool
	}{
		{name: "lowercase a accepted", short: "a"},
		{name: "lowercase z accepted", short: "z"},
		{name: "uppercase A accepted", short: "A"},
		{name: "uppercase Z accepted", short: "Z"},
		{name: "before uppercase range rejected", short: "@", wantError: true},
		{name: "after uppercase range rejected", short: "[", wantError: true},
		{name: "before lowercase range rejected", short: "`", wantError: true},
		{name: "after lowercase range rejected", short: "{", wantError: true},
		{name: "digit rejected", short: "0", wantError: true},
		{name: "multiple ASCII letters rejected", short: "aa", wantError: true},
		{name: "non-ASCII letter rejected", short: "é", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := validateSingleFlagForMutation(Flag{
				Name:        "target",
				Description: "Short alias boundary",
				Short:       tt.short,
			}, 0)
			hasInvalidShort := validationErrorsContainMessage(errs, "invalid short alias")
			if hasInvalidShort != tt.wantError {
				t.Fatalf("invalid short alias error = %v, want %v; errors: %v", hasInvalidShort, tt.wantError, errs)
			}
			if tt.wantError {
				assertStructureFlagError(t, errs[0], "command 'deploy' flag 'target'", []string{
					"invalid short alias",
					string(tt.short),
					"flags.cue",
				}, false)
				return
			}
			if len(errs) != 0 {
				t.Fatalf("validateFlag() errors = %v, want none", errs)
			}
		})
	}
}

func TestStructureValidatorValidateFlagsTracksDuplicateNamesAndShorts(t *testing.T) {
	t.Parallel()

	cmd := structureFlagMutationCommand()
	cmd.Flags = []Flag{
		{Name: "target", Description: "First target", Short: "a"},
		{Name: "target", Description: "Second target", Short: "b"},
		{Name: "output", Description: "First output", Short: "x"},
		{Name: "format", Description: "Second output", Short: "x"},
	}

	errs := NewStructureValidator().validateFlags(structureFlagMutationContext(), &cmd)
	if len(errs) != 2 {
		t.Fatalf("validateFlags() returned %d errors, want 2: %v", len(errs), errs)
	}

	assertStructureFlagError(t, errs[0], "command 'deploy'", []string{
		"duplicate flag name",
		"'target'",
		"flags.cue",
	}, false)
	assertStructureFlagError(t, errs[1], "command 'deploy'", []string{
		"duplicate short alias",
		"'x'",
		"flags.cue",
	}, false)
}

func validateSingleFlagForMutation(flag Flag, index int) []ValidationError {
	cmd := structureFlagMutationCommand()
	return NewStructureValidator().validateFlag(
		structureFlagMutationContext(),
		&cmd,
		&flag,
		index,
		make(map[string]bool),
		make(map[string]bool),
	)
}

func structureFlagMutationCommand() Command {
	return Command{Name: "deploy", Description: "Deploy the service"}
}

func structureFlagMutationContext() *ValidationContext {
	return &ValidationContext{FilePath: "flags.cue"}
}

func assertStructureFlagError(t *testing.T, err ValidationError, wantField string, wantMessage []string, wantCause bool) {
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

func validationErrorsContainMessage(errs []ValidationError, messagePart string) bool {
	for _, err := range errs {
		if strings.Contains(err.Message, messagePart) {
			return true
		}
	}
	return false
}
