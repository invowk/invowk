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

func TestDeclaredLockedModuleEntryMutationContracts(t *testing.T) {
	t.Parallel()

	requirements := []ModuleRequirement{
		{GitURL: "https://example.com/dep.git", Version: "^1.0.0"},
		{GitURL: "https://example.com/mono.git", Version: "^2.0.0", Path: "tools"},
		{GitURL: "https://example.com/empty.git", Version: "^3.0.0"},
	}
	lock := NewLockFile()
	lock.Modules["https://example.com/dep.git"] = LockedModule{
		GitURL:          "https://example.com/dep.git",
		ResolvedVersion: "1.2.3",
		Namespace:       "io.example.dep@1.2.3",
		ModuleID:        "io.example.dep",
	}
	lock.Modules["https://example.com/mono.git#tools"] = LockedModule{
		GitURL:          "https://example.com/mono.git",
		ResolvedVersion: "2.0.1",
		Path:            "tools",
		Namespace:       "io.example.tools@2.0.1",
		ModuleID:        "io.example.tools",
	}
	lock.Modules["https://example.com/empty.git"] = LockedModule{
		GitURL: "https://example.com/empty.git",
	}

	key, locked, ok := DeclaredLockedModuleEntry(requirements, lock, "io.example.tools")
	if !ok {
		t.Fatal("DeclaredLockedModuleEntry() ok = false, want true for exactly one declared locked module")
	}
	if key != "https://example.com/mono.git#tools" {
		t.Fatalf("DeclaredLockedModuleEntry() key = %q, want monorepo key", key)
	}
	if locked.ModuleID != "io.example.tools" || locked.Path != "tools" || locked.ResolvedVersion != "2.0.1" {
		t.Fatalf("DeclaredLockedModuleEntry() locked = %+v, want tools module payload preserved", locked)
	}

	if ambiguous := AmbiguousDeclaredLockedModuleEntries(requirements, lock, "io.example.tools"); ambiguous != nil {
		t.Fatalf("AmbiguousDeclaredLockedModuleEntries() = %v, want nil for a single match", ambiguous)
	}
	if _, _, ok := DeclaredLockedModuleEntry(requirements, nil, "io.example.tools"); ok {
		t.Fatal("DeclaredLockedModuleEntry(nil lock) ok = true, want false")
	}
	if _, _, ok := DeclaredLockedModuleEntry(requirements, lock, ""); ok {
		t.Fatal("DeclaredLockedModuleEntry(empty module ID) ok = true, want false")
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

func TestMissingLockedModuleRequirementKeys(t *testing.T) {
	t.Parallel()

	requirements := []ModuleRequirement{
		{GitURL: "https://example.com/dep.git", Version: "^1.0.0"},
		{GitURL: "https://example.com/missing.git", Version: "^1.0.0"},
		{GitURL: "https://example.com/mono.git", Version: "^1.0.0", Path: `modules\tools`},
	}
	lock := NewLockFile()
	lock.Modules["https://example.com/dep.git"] = LockedModule{GitURL: "https://example.com/dep.git"}
	lock.Modules["https://example.com/mono.git#modules/tools"] = LockedModule{GitURL: "https://example.com/mono.git", Path: "modules/tools"}

	got := MissingLockedModuleRequirementKeys(requirements, lock)
	want := []ModuleRefKey{"https://example.com/missing.git"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingLockedModuleRequirementKeys() = %v, want %v", got, want)
	}

	got = MissingLockedModuleRequirementKeys(requirements, nil)
	want = []ModuleRefKey{
		"https://example.com/dep.git",
		"https://example.com/missing.git",
		"https://example.com/mono.git#modules/tools",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingLockedModuleRequirementKeys(nil lock) = %v, want %v", got, want)
	}
}

func TestIsDeclaredLockedCommandSource(t *testing.T) {
	t.Parallel()

	req := ModuleRequirement{
		GitURL:  "https://example.com/tools.git",
		Version: "^1.0.0",
		Alias:   "tools",
	}
	lock := NewLockFile()
	lock.Modules[ModuleRef(req).Key()] = LockedModule{
		GitURL:          req.GitURL,
		Version:         req.Version,
		ResolvedVersion: "1.2.3",
		Namespace:       "tools",
		ModuleID:        "io.example.tools",
		CommandSourceID: "tools",
	}

	if !IsDeclaredLockedCommandSource([]ModuleRequirement{req}, lock, "io.example.tools", "tools") {
		t.Fatal("IsDeclaredLockedCommandSource() = false, want true for matching identity and source")
	}
	if IsDeclaredLockedCommandSource([]ModuleRequirement{req}, nil, "io.example.tools", "tools") {
		t.Fatal("IsDeclaredLockedCommandSource() = true with nil lock")
	}
	if IsDeclaredLockedCommandSource([]ModuleRequirement{req}, lock, "io.example.other", "tools") {
		t.Fatal("IsDeclaredLockedCommandSource() = true for mismatched module ID")
	}
	if IsDeclaredLockedCommandSource([]ModuleRequirement{req}, lock, "io.example.tools", "other") {
		t.Fatal("IsDeclaredLockedCommandSource() = true for mismatched source")
	}

	emptyIDLock := NewLockFile()
	emptyIDLock.Modules[ModuleRef(req).Key()] = LockedModule{CommandSourceID: "tools"}
	if IsDeclaredLockedCommandSource([]ModuleRequirement{req}, emptyIDLock, "", "tools") {
		t.Fatal("IsDeclaredLockedCommandSource() = true with empty module ID")
	}

	emptySourceReq := ModuleRequirement{}
	emptySourceLock := NewLockFile()
	emptySourceLock.Modules[ModuleRef(emptySourceReq).Key()] = LockedModule{
		Namespace: "io.example.tools@1.2.3",
	}
	if IsDeclaredLockedCommandSource([]ModuleRequirement{emptySourceReq}, emptySourceLock, "io.example.tools", "") {
		t.Fatal("IsDeclaredLockedCommandSource() = true with empty source ID")
	}
}
