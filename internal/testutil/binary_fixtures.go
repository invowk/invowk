// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// Fixture files for executable-format checks. They hold only the headers a
// format check reads, so tests on any host can stand in for binaries built for
// other operating systems and architectures (a Mach-O or PE invowk, a Linux
// build for another machine) without building them.

const (
	peSignatureOffset   = 0x40
	peOffsetField       = 0x3c
	peMachineAMD64      = 0x8664 // IMAGE_FILE_MACHINE_AMD64
	peCharacteristics   = 0x0022 // executable, large-address aware
	peCOFFHeaderSize    = 20
	peCharacteristicsAt = 18
	// pePadding pads the file past the 96-byte DOS header window debug/pe reads.
	pePadding = 64
)

var linuxELFFixtureMachines = map[string]elfFixtureMachine{
	"amd64":   {elf.ELFCLASS64, elf.ELFDATA2LSB, elf.EM_X86_64},
	"arm64":   {elf.ELFCLASS64, elf.ELFDATA2LSB, elf.EM_AARCH64},
	"386":     {elf.ELFCLASS32, elf.ELFDATA2LSB, elf.EM_386},
	"arm":     {elf.ELFCLASS32, elf.ELFDATA2LSB, elf.EM_ARM},
	"ppc64le": {elf.ELFCLASS64, elf.ELFDATA2LSB, elf.EM_PPC64},
	"s390x":   {elf.ELFCLASS64, elf.ELFDATA2MSB, elf.EM_S390},
	"riscv64": {elf.ELFCLASS64, elf.ELFDATA2LSB, elf.EM_RISCV},
}

type (
	// ELFFixture describes a minimal ELF file written by WriteELFFixture.
	ELFFixture struct {
		Class   elf.Class
		Data    elf.Data
		OSABI   elf.OSABI
		Type    elf.Type
		Machine elf.Machine
		// HeaderOnly writes the file header alone, with no program or section
		// headers, as UPX-compressed binaries have no section table.
		HeaderOnly bool
		// TruncateTo cuts the file to this many bytes when positive.
		TruncateTo int
		// Trailer is appended after the headers, so two otherwise equal
		// fixtures hash differently.
		Trailer []byte
	}

	elfFixtureMachine struct {
		class   elf.Class
		data    elf.Data
		machine elf.Machine
	}
)

// LinuxELFExecutable returns the fixture of a Linux ELF executable for the
// Go architecture name goarch (amd64, arm64, 386, arm, ppc64le, s390x or
// riscv64), with the OS ABI Go's linker writes for Linux (ELFOSABI_NONE). It
// fails the test for any other name.
func LinuxELFExecutable(t testing.TB, goarch string) ELFFixture {
	t.Helper()
	m, ok := linuxELFFixtureMachines[goarch]
	if !ok {
		t.Fatalf("LinuxELFExecutable: no fixture machine for architecture %q", goarch)
	}
	return ELFFixture{Class: m.class, Data: m.data, OSABI: elf.ELFOSABI_NONE, Type: elf.ET_EXEC, Machine: m.machine}
}

// WriteELFFixture writes the ELF fixture f to path and returns path.
func WriteELFFixture(t testing.TB, path string, f ELFFixture) string {
	t.Helper()
	data := f.bytes(t)
	if f.TruncateTo > 0 && f.TruncateTo < len(data) {
		data = data[:f.TruncateTo]
	}
	return writeFixture(t, path, data)
}

// WriteLinuxELFFixture writes a Linux ELF executable for goarch to path, with
// trailer appended, and returns path.
func WriteLinuxELFFixture(t testing.TB, path, goarch string, trailer []byte) string {
	t.Helper()
	f := LinuxELFExecutable(t, goarch)
	f.Trailer = trailer
	return WriteELFFixture(t, path, f)
}

// WriteMachOFixture writes the header of a 64-bit x86-64 Mach-O executable,
// as a macOS invowk build starts, to path and returns path.
func WriteMachOFixture(t testing.TB, path string) string {
	t.Helper()
	var buf bytes.Buffer
	for _, v := range []uint32{
		0xfeedfacf, // MH_MAGIC_64
		0x01000007, // CPU_TYPE_X86_64
		0x00000003, // CPU_SUBTYPE_X86_64_ALL
		0x00000002, // MH_EXECUTE
		0, 0, 0, 0, // ncmds, sizeofcmds, flags, reserved
	} {
		mustWriteBinary(t, &buf, binary.LittleEndian, v)
	}
	return writeFixture(t, path, buf.Bytes())
}

// WritePEFixture writes a minimal PE file (an MZ stub pointing at a PE
// signature and an x86-64 COFF header), as a Windows invowk build starts, to
// path and returns path.
func WritePEFixture(t testing.TB, path string) string {
	t.Helper()
	dos := make([]byte, peSignatureOffset)
	copy(dos, "MZ")
	binary.LittleEndian.PutUint32(dos[peOffsetField:], peSignatureOffset)
	coff := make([]byte, peCOFFHeaderSize)
	binary.LittleEndian.PutUint16(coff[0:], peMachineAMD64)
	binary.LittleEndian.PutUint16(coff[peCharacteristicsAt:], peCharacteristics)

	var buf bytes.Buffer
	buf.Write(dos)
	buf.WriteString("PE\x00\x00")
	buf.Write(coff)
	buf.Write(make([]byte, pePadding))
	return writeFixture(t, path, buf.Bytes())
}

// WriteShellScriptFixture writes a small shell script to path and returns path.
func WriteShellScriptFixture(t testing.TB, path string) string {
	t.Helper()
	return writeFixture(t, path, []byte("#!/bin/sh\necho invowk\n"))
}

// WriteEmptyFixture writes an empty file to path and returns path.
func WriteEmptyFixture(t testing.TB, path string) string {
	t.Helper()
	return writeFixture(t, path, nil)
}

func (f ELFFixture) bytes(t testing.TB) []byte {
	t.Helper()
	var order binary.ByteOrder = binary.LittleEndian
	if f.Data == elf.ELFDATA2MSB {
		order = binary.BigEndian
	}
	ident := [elf.EI_NIDENT]byte{0x7f, 'E', 'L', 'F', byte(f.Class), byte(f.Data), byte(elf.EV_CURRENT), byte(f.OSABI)}

	var buf bytes.Buffer
	if f.Class == elf.ELFCLASS32 {
		const ehsize, phentsize, shentsize = 52, 32, 40
		hdr := elf.Header32{
			Ident: ident, Type: uint16(f.Type), Machine: uint16(f.Machine), Version: uint32(elf.EV_CURRENT),
			Ehsize: ehsize,
		}
		if !f.HeaderOnly {
			hdr.Phoff, hdr.Phentsize, hdr.Phnum = ehsize, phentsize, 1
			hdr.Shoff, hdr.Shentsize, hdr.Shnum = ehsize+phentsize, shentsize, 1
		}
		mustWriteBinary(t, &buf, order, hdr)
		if !f.HeaderOnly {
			mustWriteBinary(t, &buf, order, elf.Prog32{Type: uint32(elf.PT_LOAD), Flags: uint32(elf.PF_R | elf.PF_X)})
			mustWriteBinary(t, &buf, order, elf.Section32{})
		}
	} else {
		const ehsize, phentsize, shentsize = 64, 56, 64
		hdr := elf.Header64{
			Ident: ident, Type: uint16(f.Type), Machine: uint16(f.Machine), Version: uint32(elf.EV_CURRENT),
			Ehsize: ehsize,
		}
		if !f.HeaderOnly {
			hdr.Phoff, hdr.Phentsize, hdr.Phnum = ehsize, phentsize, 1
			hdr.Shoff, hdr.Shentsize, hdr.Shnum = ehsize+phentsize, shentsize, 1
		}
		mustWriteBinary(t, &buf, order, hdr)
		if !f.HeaderOnly {
			mustWriteBinary(t, &buf, order, elf.Prog64{Type: uint32(elf.PT_LOAD), Flags: uint32(elf.PF_R | elf.PF_X)})
			mustWriteBinary(t, &buf, order, elf.Section64{})
		}
	}
	buf.Write(f.Trailer)
	return buf.Bytes()
}

func mustWriteBinary(t testing.TB, buf *bytes.Buffer, order binary.ByteOrder, v any) {
	t.Helper()
	if err := binary.Write(buf, order, v); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
}

func writeFixture(t testing.TB, path string, data []byte) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
	return path
}
