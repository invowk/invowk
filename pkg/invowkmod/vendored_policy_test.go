// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"reflect"
	"testing"
)

func TestDeclaredLockedModuleEntryRejectsAmbiguousModuleID(t *testing.T) {
	t.Parallel()

	requirements := []ModuleRequirement{
		{GitURL: "https://example.com/dep-a.git", Version: "^1.0.0"},
		{GitURL: "https://example.com/dep-b.git", Version: "^1.0.0"},
	}
	lock := NewLockFile()
	lock.Modules["https://example.com/dep-a.git"] = LockedModule{
		GitURL:   "https://example.com/dep-a.git",
		ModuleID: "io.example.shared",
	}
	lock.Modules["https://example.com/dep-b.git"] = LockedModule{
		GitURL:   "https://example.com/dep-b.git",
		ModuleID: "io.example.shared",
	}

	if _, _, ok := DeclaredLockedModuleEntry(requirements, lock, "io.example.shared"); ok {
		t.Fatal("DeclaredLockedModuleEntry() ok = true, want false for ambiguous module ID")
	}

	got := AmbiguousDeclaredLockedModuleEntries(requirements, lock, "io.example.shared")
	want := []ModuleRefKey{
		"https://example.com/dep-a.git",
		"https://example.com/dep-b.git",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AmbiguousDeclaredLockedModuleEntries() = %v, want %v", got, want)
	}
}

func TestOrphanedLockedModuleEntries(t *testing.T) {
	t.Parallel()

	requirements := []ModuleRequirement{
		{GitURL: "https://example.com/dep.git", Version: "^1.0.0"},
		{GitURL: "https://example.com/mono.git", Version: "^1.0.0", Path: "tools"},
	}
	lock := NewLockFile()
	lock.Modules["https://example.com/dep.git"] = LockedModule{GitURL: "https://example.com/dep.git"}
	lock.Modules["https://example.com/mono.git#tools"] = LockedModule{GitURL: "https://example.com/mono.git", Path: "tools"}
	lock.Modules["https://example.com/stale-a.git"] = LockedModule{GitURL: "https://example.com/stale-a.git"}
	lock.Modules["https://example.com/stale-b.git"] = LockedModule{GitURL: "https://example.com/stale-b.git"}

	got := OrphanedLockedModuleEntries(requirements, lock)
	want := []ModuleRefKey{
		"https://example.com/stale-a.git",
		"https://example.com/stale-b.git",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("OrphanedLockedModuleEntries() = %v, want %v", got, want)
	}
}
