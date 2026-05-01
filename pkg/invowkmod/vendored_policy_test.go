// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"reflect"
	"testing"
)

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
