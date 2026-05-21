// SPDX-License-Identifier: MPL-2.0

package provisionenv

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestManifestRoundTripPreservesCommandNamespace(t *testing.T) {
	t.Parallel()

	entries := Entries{
		{
			Path:             container.MountTargetPath("/invowk/modules/p123/shared.invowkmod"),
			CommandNamespace: invowkmod.ModuleNamespace("shared"),
		},
		{
			Path:             container.MountTargetPath("/invowk/modules/p456/shared.invowkmod"),
			CommandNamespace: invowkmod.ModuleNamespace("aliased"),
		},
	}

	value, err := MarshalManifest(entries)
	if err != nil {
		t.Fatalf("MarshalManifest() error = %v", err)
	}
	got, err := ParseManifest(value)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if len(got) != len(entries) {
		t.Fatalf("entries = %d, want %d", len(got), len(entries))
	}
	for i := range got {
		if got[i] != entries[i] {
			t.Fatalf("entry[%d] = %#v, want %#v", i, got[i], entries[i])
		}
	}
}

func TestParseEnvironmentInvalidManifestDoesNotFallback(t *testing.T) {
	t.Parallel()

	entries, err := ParseEnvironment(Value("{not-json"), Value("/invowk/modules"))
	if err == nil {
		t.Fatal("ParseEnvironment() error = nil, want invalid manifest error")
	}
	if !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("ParseEnvironment() error = %v, want ErrInvalidManifest", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %#v, want none", entries)
	}
}

func TestParseEnvironmentFallsBackWhenManifestAbsent(t *testing.T) {
	t.Parallel()

	entries, err := ParseEnvironment("", Value("/invowk/modules"))
	if err != nil {
		t.Fatalf("ParseEnvironment() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Path != "/invowk/modules" {
		t.Fatalf("entry path = %q, want /invowk/modules", entries[0].Path)
	}
}
