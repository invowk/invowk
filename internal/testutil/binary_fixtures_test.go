// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"os"
	"path/filepath"
	"testing"
)

func TestBinaryFixturesParseAsTheirFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for goarch, want := range linuxELFFixtureMachines {
		path := WriteLinuxELFFixture(t, filepath.Join(dir, "elf-"+goarch), goarch, []byte("trailer"))
		f, err := elf.Open(path)
		if err != nil {
			t.Fatalf("elf.Open(%s fixture) = %v", goarch, err)
		}
		if f.Class != want.class || f.Data != want.data || f.Machine != want.machine ||
			f.OSABI != elf.ELFOSABI_NONE || f.Type != elf.ET_EXEC || f.Entry != elfFixtureEntry ||
			len(f.Progs) != 1 || len(f.Sections) != 1 {
			t.Errorf("%s fixture header = %+v, progs %d, sections %d", goarch, f.FileHeader, len(f.Progs), len(f.Sections))
		}
		_ = f.Close()
	}

	headerOnly := LinuxELFExecutable(t, "amd64")
	headerOnly.HeaderOnly = true
	path := WriteELFFixture(t, filepath.Join(dir, "header-only"), headerOnly)
	f, err := elf.Open(path)
	if err != nil {
		t.Fatalf("elf.Open(header-only fixture) = %v", err)
	}
	if len(f.Progs) != 0 || len(f.Sections) != 0 {
		t.Errorf("header-only fixture progs %d, sections %d, want 0 and 0", len(f.Progs), len(f.Sections))
	}
	_ = f.Close()

	truncated := LinuxELFExecutable(t, "amd64")
	truncated.TruncateTo = 20
	path = WriteELFFixture(t, filepath.Join(dir, "truncated"), truncated)
	if info, statErr := os.Stat(path); statErr != nil || info.Size() != 20 {
		t.Fatalf("truncated fixture stat = %v, %v; want 20 bytes", info, statErr)
	}
	if truncatedFile, openErr := elf.Open(path); openErr == nil {
		_ = truncatedFile.Close()
		t.Error("elf.Open(truncated fixture) = nil error, want a format error")
	}

	machoFile, err := macho.Open(WriteMachOFixture(t, filepath.Join(dir, "macho")))
	if err != nil {
		t.Fatalf("macho.Open(fixture) = %v", err)
	}
	if machoFile.Cpu != macho.CpuAmd64 || machoFile.Type != macho.TypeExec {
		t.Errorf("Mach-O fixture header = %+v", machoFile.FileHeader)
	}
	_ = machoFile.Close()

	peFile, err := pe.Open(WritePEFixture(t, filepath.Join(dir, "pe")))
	if err != nil {
		t.Fatalf("pe.Open(fixture) = %v", err)
	}
	if peFile.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		t.Errorf("PE fixture machine = %#x, want AMD64", peFile.Machine)
	}
	_ = peFile.Close()

	if info, statErr := os.Stat(WriteEmptyFixture(t, filepath.Join(dir, "empty"))); statErr != nil || info.Size() != 0 {
		t.Errorf("empty fixture stat = %v, %v", info, statErr)
	}
	if data, readErr := os.ReadFile(WriteShellScriptFixture(t, filepath.Join(dir, "script"))); readErr != nil || string(data[:2]) != "#!" {
		t.Errorf("shell script fixture = %q, %v", data, readErr)
	}
}
