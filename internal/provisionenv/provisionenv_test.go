// SPDX-License-Identifier: MPL-2.0

package provisionenv

import (
	"errors"
	"path/filepath"
	"strings"
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

func TestNameContracts(t *testing.T) {
	t.Parallel()

	if got, want := ModuleManifestName.String(), "INVOWK_MODULE_MANIFEST"; got != want {
		t.Fatalf("ModuleManifestName.String() = %q, want %q", got, want)
	}
	if err := Name("").Validate(); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("Name(\"\").Validate() error = %v, want ErrInvalidName", err)
	}
	if err := ModulePathName.Validate(); err != nil {
		t.Fatalf("ModulePathName.Validate() error = %v, want nil", err)
	}
}

func TestEntriesValidateChecksEveryEntry(t *testing.T) {
	t.Parallel()

	err := Entries{
		{Path: "/invowk/modules/valid.invowkmod"},
		{Path: ""},
	}.Validate()
	if err == nil {
		t.Fatal("Entries.Validate() error = nil, want invalid second entry")
	}
	if !errors.Is(err, container.ErrInvalidMountTargetPath) {
		t.Fatalf("Entries.Validate() error = %v, want ErrInvalidMountTargetPath", err)
	}
	if !strings.Contains(err.Error(), "[1]") {
		t.Fatalf("Entries.Validate() error = %q, want second-entry index", err.Error())
	}
}

func TestMarshalManifestRejectsInvalidEntries(t *testing.T) {
	t.Parallel()

	value, err := MarshalManifest(Entries{{Path: ""}})
	if err == nil {
		t.Fatalf("MarshalManifest() = %q, nil error; want invalid manifest", value)
	}
	if !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("MarshalManifest() error = %v, want ErrInvalidManifest", err)
	}
	if !errors.Is(err, container.ErrInvalidMountTargetPath) {
		t.Fatalf("MarshalManifest() error = %v, want ErrInvalidMountTargetPath", err)
	}
}

func TestParseManifestBlankValue(t *testing.T) {
	t.Parallel()

	entries, err := ParseManifest(Value(" \t "))
	if err != nil {
		t.Fatalf("ParseManifest(blank) error = %v, want nil", err)
	}
	if entries != nil {
		t.Fatalf("ParseManifest(blank) entries = %#v, want nil", entries)
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

func TestEntriesFromPathListSkipsBlankAndInvalidSegments(t *testing.T) {
	t.Parallel()

	separator := string(filepath.ListSeparator)
	value := Value(strings.Join([]string{
		"/invowk/modules/first.invowkmod",
		" ",
		"/invowk/modules/second.invowkmod",
	}, separator))

	entries := EntriesFromPathList(value)
	if len(entries) != 2 {
		t.Fatalf("EntriesFromPathList() entries = %#v, want two valid absolute paths", entries)
	}
	if entries[0].Path != "/invowk/modules/first.invowkmod" {
		t.Fatalf("entry[0].Path = %q, want first module path", entries[0].Path)
	}
	if entries[1].Path != "/invowk/modules/second.invowkmod" {
		t.Fatalf("entry[1].Path = %q, want second module path", entries[1].Path)
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
