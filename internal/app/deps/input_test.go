// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestValidateFlagValues(t *testing.T) {
	t.Parallel()

	t.Run("nil defs returns nil", func(t *testing.T) {
		t.Parallel()
		if err := ValidateFlagValues("build", nil, nil); err != nil {
			t.Fatalf("ValidateFlagValues() = %v, want nil", err)
		}
	})

	t.Run("required flag missing", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Flag{
			{Name: "name", Required: true},
		}
		err := ValidateFlagValues("build", map[invowkfile.FlagName]string{}, defs)
		if err == nil {
			t.Fatal("ValidateFlagValues() = nil, want error")
		}
		if !strings.Contains(err.Error(), "required flag '--name'") {
			t.Fatalf("error = %v, want error containing %q", err, "required flag '--name'")
		}
	})

	t.Run("required flag empty", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Flag{
			{Name: "name", Required: true},
		}
		err := ValidateFlagValues("build", map[invowkfile.FlagName]string{"name": ""}, defs)
		if err == nil {
			t.Fatal("ValidateFlagValues() = nil, want error")
		}
		if !strings.Contains(err.Error(), "required flag") {
			t.Fatalf("error = %v, want error containing %q", err, "required flag")
		}
	})

	t.Run("optional flag empty", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Flag{
			{Name: "name", Required: false, Type: invowkfile.FlagTypeString},
		}
		if err := ValidateFlagValues("build", map[invowkfile.FlagName]string{"name": ""}, defs); err != nil {
			t.Fatalf("ValidateFlagValues() = %v, want nil", err)
		}
	})

	t.Run("valid value passes", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Flag{
			{Name: "count", Type: invowkfile.FlagTypeInt},
		}
		if err := ValidateFlagValues("build", map[invowkfile.FlagName]string{"count": "42"}, defs); err != nil {
			t.Fatalf("ValidateFlagValues() = %v, want nil", err)
		}
	})

	t.Run("invalid value fails regex", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Flag{
			{Name: "port", Type: invowkfile.FlagTypeString, Validation: "^[0-9]+$"},
		}
		err := ValidateFlagValues("build", map[invowkfile.FlagName]string{"port": "bad"}, defs)
		if err == nil {
			t.Fatal("ValidateFlagValues() = nil, want error")
		}
		if !strings.Contains(err.Error(), "port") {
			t.Fatalf("error = %v, want error mentioning flag name", err)
		}
	})

	t.Run("multiple errors aggregated", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Flag{
			{Name: "first", Required: true},
			{Name: "second", Required: true},
		}
		err := ValidateFlagValues("build", map[invowkfile.FlagName]string{}, defs)
		if err == nil {
			t.Fatal("ValidateFlagValues() = nil, want error")
		}
		if !strings.Contains(err.Error(), "--first") || !strings.Contains(err.Error(), "--second") {
			t.Fatalf("error = %v, want error containing both flag names", err)
		}
	})
}

func TestValidateArguments(t *testing.T) {
	t.Parallel()

	t.Run("empty defs allows any args", func(t *testing.T) {
		t.Parallel()
		if err := ValidateArguments("build", []string{"a", "b"}, nil); err != nil {
			t.Fatalf("ValidateArguments() = %v, want nil", err)
		}
	})

	t.Run("empty defs slice allows any args", func(t *testing.T) {
		t.Parallel()
		if err := ValidateArguments("build", []string{"a"}, []invowkfile.Argument{}); err != nil {
			t.Fatalf("ValidateArguments() = %v, want nil", err)
		}
	})

	t.Run("no args with no required defs", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "target", Required: false},
		}
		if err := ValidateArguments("build", nil, defs); err != nil {
			t.Fatalf("ValidateArguments() = %v, want nil", err)
		}
	})

	t.Run("missing required arg", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "target", Required: true},
			{Name: "extra", Required: false},
		}
		err := ValidateArguments("build", nil, defs)
		if err == nil {
			t.Fatal("ValidateArguments() = nil, want error")
		}
		var argErr *ArgumentValidationError
		if !errors.As(err, &argErr) {
			t.Fatalf("error type = %T, want *ArgumentValidationError", err)
		}
		if argErr.Type != ArgErrMissingRequired {
			t.Errorf("Type = %v, want ArgErrMissingRequired", argErr.Type)
		}
	})

	t.Run("too many args non-variadic", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "target", Required: true},
		}
		err := ValidateArguments("build", []string{"a", "b", "c"}, defs)
		if err == nil {
			t.Fatal("ValidateArguments() = nil, want error")
		}
		var argErr *ArgumentValidationError
		if !errors.As(err, &argErr) {
			t.Fatalf("error type = %T, want *ArgumentValidationError", err)
		}
		if argErr.Type != ArgErrTooMany {
			t.Errorf("Type = %v, want ArgErrTooMany", argErr.Type)
		}
	})

	t.Run("variadic allows overflow", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "target", Required: true},
			{Name: "extras", Variadic: true},
		}
		if err := ValidateArguments("build", []string{"a", "b", "c", "d"}, defs); err != nil {
			t.Fatalf("ValidateArguments() = %v, want nil", err)
		}
	})

	t.Run("exact count matches", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "src", Required: true},
			{Name: "dst", Required: true},
		}
		if err := ValidateArguments("copy", []string{"a", "b"}, defs); err != nil {
			t.Fatalf("ValidateArguments() = %v, want nil", err)
		}
	})

	t.Run("invalid value fails regex", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "port", Required: true, Validation: "^[0-9]+$"},
		}
		err := ValidateArguments("serve", []string{"abc"}, defs)
		if err == nil {
			t.Fatal("ValidateArguments() = nil, want error")
		}
		var argErr *ArgumentValidationError
		if !errors.As(err, &argErr) {
			t.Fatalf("error type = %T, want *ArgumentValidationError", err)
		}
		if argErr.Type != ArgErrInvalidValue {
			t.Errorf("Type = %v, want ArgErrInvalidValue", argErr.Type)
		}
		if argErr.InvalidArg != "port" {
			t.Errorf("InvalidArg = %q, want %q", argErr.InvalidArg, "port")
		}
	})

	t.Run("variadic value fails regex", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "files", Required: true, Variadic: true, Validation: `^[a-z]+\.txt$`},
		}
		err := ValidateArguments("cat", []string{"good.txt", "BAD.txt"}, defs)
		if err == nil {
			t.Fatal("ValidateArguments() = nil, want error")
		}
		var argErr *ArgumentValidationError
		if !errors.As(err, &argErr) {
			t.Fatalf("error type = %T, want *ArgumentValidationError", err)
		}
		if argErr.Type != ArgErrInvalidValue {
			t.Errorf("Type = %v, want ArgErrInvalidValue", argErr.Type)
		}
	})

	t.Run("all valid args pass", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "host", Required: true, Validation: `^[a-z]+$`},
			{Name: "port", Required: true, Validation: `^[0-9]+$`},
		}
		if err := ValidateArguments("connect", []string{"localhost", "8080"}, defs); err != nil {
			t.Fatalf("ValidateArguments() = %v, want nil", err)
		}
	})
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

	t.Run("below min", func(t *testing.T) {
		t.Parallel()
		err := validateArgumentCount("copy", nil, defs, 1, 2, false)
		if err == nil {
			t.Fatal("expected error for below min")
		}
		var argErr *ArgumentValidationError
		if !errors.As(err, &argErr) {
			t.Fatalf("error type = %T, want *ArgumentValidationError", err)
		}
		if argErr.Type != ArgErrMissingRequired {
			t.Errorf("Type = %v, want ArgErrMissingRequired", argErr.Type)
		}
	})

	t.Run("at min", func(t *testing.T) {
		t.Parallel()
		if err := validateArgumentCount("copy", []string{"a"}, defs, 1, 2, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("at max", func(t *testing.T) {
		t.Parallel()
		if err := validateArgumentCount("copy", []string{"a", "b"}, defs, 1, 2, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("above max non-variadic", func(t *testing.T) {
		t.Parallel()
		err := validateArgumentCount("copy", []string{"a", "b", "c"}, defs, 1, 2, false)
		if err == nil {
			t.Fatal("expected error for above max")
		}
		var argErr *ArgumentValidationError
		if !errors.As(err, &argErr) {
			t.Fatalf("error type = %T, want *ArgumentValidationError", err)
		}
		if argErr.Type != ArgErrTooMany {
			t.Errorf("Type = %v, want ArgErrTooMany", argErr.Type)
		}
	})

	t.Run("above max variadic", func(t *testing.T) {
		t.Parallel()
		if err := validateArgumentCount("copy", []string{"a", "b", "c"}, defs, 1, 2, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateArgumentValues(t *testing.T) {
	t.Parallel()

	t.Run("all valid", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "host"},
			{Name: "port", Validation: `^[0-9]+$`},
		}
		if err := validateArgumentValues("serve", []string{"localhost", "8080"}, defs); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid second arg", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "host"},
			{Name: "port", Validation: `^[0-9]+$`},
		}
		err := validateArgumentValues("serve", []string{"localhost", "bad"}, defs)
		if err == nil {
			t.Fatal("expected error for invalid arg value")
		}
		var argErr *ArgumentValidationError
		if !errors.As(err, &argErr) {
			t.Fatalf("error type = %T, want *ArgumentValidationError", err)
		}
		if argErr.InvalidArg != "port" {
			t.Errorf("InvalidArg = %q, want %q", argErr.InvalidArg, "port")
		}
	})

	t.Run("extra args beyond defs ignored", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "host"},
		}
		if err := validateArgumentValues("serve", []string{"localhost", "extra1", "extra2"}, defs); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("variadic delegates to variadic validator", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{
			{Name: "files", Variadic: true, Validation: `^[a-z]+$`},
		}
		err := validateArgumentValues("cat", []string{"good", "UPPER"}, defs)
		if err == nil {
			t.Fatal("expected error for invalid variadic arg value")
		}
	})
}

func TestValidateVariadicArgumentValues(t *testing.T) {
	t.Parallel()

	argDef := invowkfile.Argument{Name: "files", Variadic: true, Validation: `^[a-z]+$`}

	t.Run("all valid", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{argDef}
		if err := validateVariadicArgumentValues("cat", []string{"abc", "def"}, defs, 0, argDef); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("first value invalid", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{argDef}
		err := validateVariadicArgumentValues("cat", []string{"BAD", "good"}, defs, 0, argDef)
		if err == nil {
			t.Fatal("expected error for invalid variadic value")
		}
		argErr, ok := errors.AsType[*ArgumentValidationError](err)
		if !ok {
			t.Fatalf("expected *ArgumentValidationError, got %T: %v", err, err)
		}
		if argErr.InvalidArg != "files" {
			t.Errorf("InvalidArg = %q, want %q", argErr.InvalidArg, "files")
		}
	})

	t.Run("last value invalid", func(t *testing.T) {
		t.Parallel()
		defs := []invowkfile.Argument{argDef}
		err := validateVariadicArgumentValues("cat", []string{"good", "BAD"}, defs, 0, argDef)
		if err == nil {
			t.Fatal("expected error for invalid variadic value")
		}
	})
}

func TestArgumentValidationErrorMessages(t *testing.T) {
	t.Parallel()

	t.Run("missing required error message", func(t *testing.T) {
		t.Parallel()
		err := &ArgumentValidationError{
			Type:         ArgErrMissingRequired,
			CommandName:  "deploy",
			ProvidedArgs: []string{"a"},
			MinArgs:      3,
		}
		msg := err.Error()
		if !strings.Contains(msg, "missing required") {
			t.Errorf("error message = %q, want 'missing required'", msg)
		}
		if !strings.Contains(msg, "'deploy'") {
			t.Errorf("error message = %q, want command name 'deploy'", msg)
		}
	})

	t.Run("too many error message", func(t *testing.T) {
		t.Parallel()
		err := &ArgumentValidationError{
			Type:         ArgErrTooMany,
			CommandName:  "build",
			ProvidedArgs: []string{"a", "b", "c"},
			MaxArgs:      1,
		}
		msg := err.Error()
		if !strings.Contains(msg, "too many") {
			t.Errorf("error message = %q, want 'too many'", msg)
		}
	})

	t.Run("invalid value error message", func(t *testing.T) {
		t.Parallel()
		err := &ArgumentValidationError{
			Type:         ArgErrInvalidValue,
			CommandName:  "serve",
			InvalidArg:   "port",
			InvalidValue: "abc",
			ValueError:   errors.New("must be numeric"),
		}
		msg := err.Error()
		if !strings.Contains(msg, "'port'") {
			t.Errorf("error message = %q, want arg name 'port'", msg)
		}
		if !strings.Contains(msg, "must be numeric") {
			t.Errorf("error message = %q, want value error text", msg)
		}
	})
}
