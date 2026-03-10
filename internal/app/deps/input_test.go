// SPDX-License-Identifier: MPL-2.0

package deps

import (
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
