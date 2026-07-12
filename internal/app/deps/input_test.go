// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestValidateFlagValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		values         map[invowkfile.FlagName]string
		defs           []invowkfile.Flag
		wantSubstrings []string
		wantTypedError bool
	}{
		{name: "nil defs returns nil"},
		{name: "required flag missing", values: map[invowkfile.FlagName]string{}, defs: []invowkfile.Flag{{Name: "name", Required: true}}, wantSubstrings: []string{"required flag '--name'"}, wantTypedError: true},
		{name: "required flag empty", values: map[invowkfile.FlagName]string{"name": ""}, defs: []invowkfile.Flag{{Name: "name", Required: true}}, wantSubstrings: []string{"required flag"}},
		{name: "required flag with value passes", values: map[invowkfile.FlagName]string{"count": "42"}, defs: []invowkfile.Flag{{Name: "count", Required: true, Type: invowkfile.FlagTypeInt}}},
		{name: "optional flag empty", values: map[invowkfile.FlagName]string{"name": ""}, defs: []invowkfile.Flag{{Name: "name", Type: invowkfile.FlagTypeString}}},
		{name: "optional typed flag empty skips value validation", values: map[invowkfile.FlagName]string{"count": ""}, defs: []invowkfile.Flag{{Name: "count", Type: invowkfile.FlagTypeInt}}},
		{name: "valid value passes", values: map[invowkfile.FlagName]string{"count": "42"}, defs: []invowkfile.Flag{{Name: "count", Type: invowkfile.FlagTypeInt}}},
		{name: "invalid value fails regex", values: map[invowkfile.FlagName]string{"port": "bad"}, defs: []invowkfile.Flag{{Name: "port", Type: invowkfile.FlagTypeString, Validation: "^[0-9]+$"}}, wantSubstrings: []string{"port"}},
		{name: "multiple errors aggregated", values: map[invowkfile.FlagName]string{}, defs: []invowkfile.Flag{{Name: "first", Required: true}, {Name: "second", Required: true}}, wantSubstrings: []string{"--first", "--second"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateFlagValues("build", tt.values, tt.defs)
			wantErr := len(tt.wantSubstrings) > 0
			if (err != nil) != wantErr {
				t.Fatalf("ValidateFlagValues() error = %v, wantErr %v", err, wantErr)
			}
			if !wantErr {
				return
			}
			if !errors.Is(err, ErrFlagValidationFailed) {
				t.Fatalf("errors.Is(err, ErrFlagValidationFailed) = false for %v", err)
			}
			for _, want := range tt.wantSubstrings {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error = %v, want containing %q", err, want)
				}
			}
			if tt.wantTypedError {
				var flagErr *FlagValidationError
				if !errors.As(err, &flagErr) {
					t.Fatalf("errors.As(err, *FlagValidationError) = false for %v", err)
				}
				if flagErr.CommandName != "build" {
					t.Fatalf("CommandName = %q, want build", flagErr.CommandName)
				}
			}
		})
	}
}

func TestValidateArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		command        string
		provided       []string
		defs           []invowkfile.Argument
		wantErr        bool
		wantType       ArgErrType
		wantInvalidArg invowkfile.ArgumentName
	}{
		{name: "empty defs allows any args", command: "build", provided: []string{"a", "b"}},
		{name: "empty defs slice allows any args", command: "build", provided: []string{"a"}, defs: []invowkfile.Argument{}},
		{name: "no args with no required defs", command: "build", defs: []invowkfile.Argument{{Name: "target"}}},
		{name: "missing required arg", command: "build", defs: []invowkfile.Argument{{Name: "target", Required: true}, {Name: "extra"}}, wantErr: true, wantType: ArgErrMissingRequired},
		{name: "too many args non-variadic", command: "build", provided: []string{"a", "b", "c"}, defs: []invowkfile.Argument{{Name: "target", Required: true}}, wantErr: true, wantType: ArgErrTooMany},
		{name: "variadic allows overflow", command: "build", provided: []string{"a", "b", "c", "d"}, defs: []invowkfile.Argument{{Name: "target", Required: true}, {Name: "extras", Variadic: true}}},
		{name: "exact count matches", command: "copy", provided: []string{"a", "b"}, defs: []invowkfile.Argument{{Name: "src", Required: true}, {Name: "dst", Required: true}}},
		{name: "invalid value fails regex", command: "serve", provided: []string{"abc"}, defs: []invowkfile.Argument{{Name: "port", Required: true, Validation: "^[0-9]+$"}}, wantErr: true, wantType: ArgErrInvalidValue, wantInvalidArg: "port"},
		{name: "variadic value fails regex", command: "cat", provided: []string{"good.txt", "BAD.txt"}, defs: []invowkfile.Argument{{Name: "files", Required: true, Variadic: true, Validation: `^[a-z]+\.txt$`}}, wantErr: true, wantType: ArgErrInvalidValue},
		{name: "all valid args pass", command: "connect", provided: []string{"localhost", "8080"}, defs: []invowkfile.Argument{{Name: "host", Required: true, Validation: `^[a-z]+$`}, {Name: "port", Required: true, Validation: `^[0-9]+$`}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateArguments(tt.command, tt.provided, tt.defs)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ValidateArguments() = %v, want nil", err)
				}
				return
			}
			var argErr *ArgumentValidationError
			if !errors.As(err, &argErr) {
				t.Fatalf("error type = %T, want *ArgumentValidationError", err)
			}
			if argErr.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", argErr.Type, tt.wantType)
			}
			if tt.wantInvalidArg != "" && argErr.InvalidArg != tt.wantInvalidArg {
				t.Errorf("InvalidArg = %q, want %q", argErr.InvalidArg, tt.wantInvalidArg)
			}
		})
	}
}

func TestSummarizeArgDefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		defs       []invowkfile.Argument
		wantMin    int
		wantMax    int
		wantVariad bool
	}{
		{"empty", nil, 0, 0, false},
		{"one required", []invowkfile.Argument{{Name: "a", Required: true}}, 1, 1, false},
		{"one optional", []invowkfile.Argument{{Name: "a"}}, 0, 1, false},
		{"mixed", []invowkfile.Argument{
			{Name: "a", Required: true},
			{Name: "b"},
			{Name: "c", Required: true},
		}, 2, 3, false},
		{"with variadic", []invowkfile.Argument{
			{Name: "a", Required: true},
			{Name: "rest", Variadic: true},
		}, 1, 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotMin, gotMax, gotVariadic := summarizeArgDefs(tt.defs)
			if gotMin != tt.wantMin {
				t.Errorf("minArgs = %d, want %d", gotMin, tt.wantMin)
			}
			if gotMax != tt.wantMax {
				t.Errorf("maxArgs = %d, want %d", gotMax, tt.wantMax)
			}
			if gotVariadic != tt.wantVariad {
				t.Errorf("hasVariadic = %v, want %v", gotVariadic, tt.wantVariad)
			}
		})
	}
}

func TestValidateArgumentCount(t *testing.T) {
	t.Parallel()

	defs := []invowkfile.Argument{
		{Name: "src", Required: true},
		{Name: "dst"},
	}

	tests := []struct {
		name     string
		provided []string
		variadic bool
		wantErr  bool
		wantType ArgErrType
	}{
		{name: "below min", wantErr: true, wantType: ArgErrMissingRequired},
		{name: "at min", provided: []string{"a"}},
		{name: "at max", provided: []string{"a", "b"}},
		{name: "above max non-variadic", provided: []string{"a", "b", "c"}, wantErr: true, wantType: ArgErrTooMany},
		{name: "above max variadic", provided: []string{"a", "b", "c"}, variadic: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateArgumentCount("copy", tt.provided, defs, 1, 2, tt.variadic)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("validateArgumentCount() = %v, want nil", err)
				}
				return
			}
			var argErr *ArgumentValidationError
			if !errors.As(err, &argErr) {
				t.Fatalf("error type = %T, want *ArgumentValidationError", err)
			}
			if argErr.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", argErr.Type, tt.wantType)
			}
		})
	}
}

func TestValidateArgumentValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		command        string
		provided       []string
		defs           []invowkfile.Argument
		wantInvalidArg invowkfile.ArgumentName
		wantErr        bool
	}{
		{name: "all valid", command: "serve", provided: []string{"localhost", "8080"}, defs: []invowkfile.Argument{
			{Name: "host"},
			{Name: "port", Validation: `^[0-9]+$`},
		}},
		{name: "invalid second arg", command: "serve", provided: []string{"localhost", "bad"}, wantErr: true, wantInvalidArg: "port", defs: []invowkfile.Argument{
			{Name: "host"},
			{Name: "port", Validation: `^[0-9]+$`},
		}},
		{name: "extra args beyond defs ignored", command: "serve", provided: []string{"localhost", "extra1", "extra2"}, defs: []invowkfile.Argument{
			{Name: "host"},
		}},
		{name: "variadic delegates to variadic validator", command: "cat", provided: []string{"good", "UPPER"}, wantErr: true, defs: []invowkfile.Argument{
			{Name: "files", Variadic: true, Validation: `^[a-z]+$`},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateArgumentValues(tt.command, tt.provided, tt.defs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateArgumentValues() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantInvalidArg != "" {
				var argErr *ArgumentValidationError
				if !errors.As(err, &argErr) {
					t.Fatalf("error type = %T, want *ArgumentValidationError", err)
				}
				if argErr.InvalidArg != tt.wantInvalidArg {
					t.Errorf("InvalidArg = %q, want %q", argErr.InvalidArg, tt.wantInvalidArg)
				}
			}
		})
	}
}

func TestValidateVariadicArgumentValues(t *testing.T) {
	t.Parallel()

	argDef := invowkfile.Argument{Name: "files", Variadic: true, Validation: `^[a-z]+$`}

	tests := []struct {
		name           string
		provided       []string
		wantErr        bool
		wantInvalidArg invowkfile.ArgumentName
	}{
		{name: "all valid", provided: []string{"abc", "def"}},
		{name: "first value invalid", provided: []string{"BAD", "good"}, wantErr: true, wantInvalidArg: "files"},
		{name: "last value invalid", provided: []string{"good", "BAD"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateVariadicArgumentValues("cat", tt.provided, []invowkfile.Argument{argDef}, 0, argDef)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateVariadicArgumentValues() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantInvalidArg != "" {
				argErr, ok := errors.AsType[*ArgumentValidationError](err)
				if !ok {
					t.Fatalf("expected *ArgumentValidationError, got %T: %v", err, err)
				}
				if argErr.InvalidArg != tt.wantInvalidArg {
					t.Errorf("InvalidArg = %q, want %q", argErr.InvalidArg, tt.wantInvalidArg)
				}
			}
		})
	}
}

func TestArgumentValidationErrorConstructorsPreservePayloads(t *testing.T) {
	t.Parallel()

	t.Run("count error", testArgumentValidationCountErrorPreservesPayload)
	t.Run("value error", testArgumentValidationValueErrorPreservesPayload)
}

func testArgumentValidationCountErrorPreservesPayload(t *testing.T) {
	t.Parallel()

	defs := []invowkfile.Argument{
		{Name: "src", Required: true},
		{Name: "dst", Required: true},
		{Name: "mode"},
	}
	provided := []string{"src.txt"}
	argErr := requireArgumentValidationError(t, newArgumentCountError(ArgErrMissingRequired, "copy", provided, defs, 2, 3))

	if argErr.Type != ArgErrMissingRequired {
		t.Fatalf("Type = %v, want %v", argErr.Type, ArgErrMissingRequired)
	}
	if argErr.CommandName != "copy" {
		t.Fatalf("CommandName = %q, want copy", argErr.CommandName)
	}
	if !slices.Equal(argErr.ArgDefs, defs) {
		t.Fatalf("ArgDefs = %+v, want %+v", argErr.ArgDefs, defs)
	}
	if !slices.Equal(argErr.ProvidedArgs, provided) {
		t.Fatalf("ProvidedArgs = %v, want %v", argErr.ProvidedArgs, provided)
	}
	if argErr.MinArgs != 2 || argErr.MaxArgs != 3 {
		t.Fatalf("argument bounds = %d/%d, want 2/3", argErr.MinArgs, argErr.MaxArgs)
	}
}

func testArgumentValidationValueErrorPreservesPayload(t *testing.T) {
	t.Parallel()

	defs := []invowkfile.Argument{
		{Name: "host"},
		{Name: "port", Type: invowkfile.ArgumentTypeInt},
	}
	provided := []string{"localhost", "bad"}
	valueErr := errors.New("port must be numeric")
	argErr := requireArgumentValidationError(t, newArgumentValueError("serve", provided, defs, "port", "bad", valueErr))

	if argErr.Type != ArgErrInvalidValue {
		t.Fatalf("Type = %v, want %v", argErr.Type, ArgErrInvalidValue)
	}
	if argErr.CommandName != "serve" {
		t.Fatalf("CommandName = %q, want serve", argErr.CommandName)
	}
	if !slices.Equal(argErr.ArgDefs, defs) {
		t.Fatalf("ArgDefs = %+v, want %+v", argErr.ArgDefs, defs)
	}
	if !slices.Equal(argErr.ProvidedArgs, provided) {
		t.Fatalf("ProvidedArgs = %v, want %v", argErr.ProvidedArgs, provided)
	}
	if argErr.InvalidArg != "port" || argErr.InvalidValue != "bad" {
		t.Fatalf("invalid argument payload = %q/%q, want port/bad", argErr.InvalidArg, argErr.InvalidValue)
	}
	if !errors.Is(argErr.ValueError, valueErr) {
		t.Fatalf("ValueError = %v, want %v", argErr.ValueError, valueErr)
	}
}

func TestArgumentValidationErrorMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *ArgumentValidationError
		want []string
	}{
		{name: "missing required error message", err: &ArgumentValidationError{
			Type:         ArgErrMissingRequired,
			CommandName:  "deploy",
			ProvidedArgs: []string{"a"},
			MinArgs:      3,
		}, want: []string{"missing required", "'deploy'"}},
		{name: "too many error message", err: &ArgumentValidationError{
			Type:         ArgErrTooMany,
			CommandName:  "build",
			ProvidedArgs: []string{"a", "b", "c"},
			MaxArgs:      1,
		}, want: []string{"too many"}},
		{name: "invalid value error message", err: &ArgumentValidationError{
			Type:         ArgErrInvalidValue,
			CommandName:  "serve",
			InvalidArg:   "port",
			InvalidValue: "abc",
			ValueError:   errors.New("must be numeric"),
		}, want: []string{"'port'", "must be numeric"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msg := tt.err.Error()
			for _, want := range tt.want {
				if !strings.Contains(msg, want) {
					t.Errorf("error message = %q, want containing %q", msg, want)
				}
			}
		})
	}
}

func requireArgumentValidationError(t *testing.T, err error) *ArgumentValidationError {
	t.Helper()

	argErr, ok := errors.AsType[*ArgumentValidationError](err)
	if !ok {
		t.Fatalf("error type = %T, want *ArgumentValidationError", err)
	}
	return argErr
}
