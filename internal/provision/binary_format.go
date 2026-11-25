// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

// Kinds of BinaryRejection (the zero value is not a rejection), and the
// Mach-O magic numbers, read big-endian from the first four bytes: thin
// 32-bit and 64-bit files in either byte order, and universal (fat) files.
const (
	// BinaryRejectionUnreadable: the file could not be opened or read.
	BinaryRejectionUnreadable BinaryRejectionKind = 1
	// BinaryRejectionNotELF: the file is not an ELF file (Mach-O, PE, a script...).
	BinaryRejectionNotELF BinaryRejectionKind = 2
	// BinaryRejectionMalformedELF: the file starts with the ELF magic but its
	// header does not parse (for example a truncated file).
	BinaryRejectionMalformedELF BinaryRejectionKind = 3
	// BinaryRejectionOtherOS: the ELF OS ABI is neither none nor Linux.
	BinaryRejectionOtherOS BinaryRejectionKind = 4
	// BinaryRejectionNotExecutable: the ELF is not an executable or PIE.
	BinaryRejectionNotExecutable BinaryRejectionKind = 5
	// BinaryRejectionMachine: the ELF is for another architecture than the container's.
	BinaryRejectionMachine BinaryRejectionKind = 6

	machOMagic32        uint32 = 0xfeedface
	machOMagic64        uint32 = 0xfeedfacf
	machOCigam32        uint32 = 0xcefaedfe
	machOCigam64        uint32 = 0xcffaedfe
	machOFatMagic       uint32 = 0xcafebabe
	machOFatCigam       uint32 = 0xbebafeca
	machOMagicByteCount        = 4
)

// linuxELFMachines maps Go/OCI architecture names, as engines report them for
// images, to the ELF identity of a Linux executable for that architecture.
var linuxELFMachines = map[container.ImageArchitecture]elfMachine{
	"amd64":   {elf.ELFCLASS64, elf.ELFDATA2LSB, elf.EM_X86_64},
	"arm64":   {elf.ELFCLASS64, elf.ELFDATA2LSB, elf.EM_AARCH64},
	"386":     {elf.ELFCLASS32, elf.ELFDATA2LSB, elf.EM_386},
	"arm":     {elf.ELFCLASS32, elf.ELFDATA2LSB, elf.EM_ARM},
	"ppc64le": {elf.ELFCLASS64, elf.ELFDATA2LSB, elf.EM_PPC64},
	"s390x":   {elf.ELFCLASS64, elf.ELFDATA2MSB, elf.EM_S390},
	"riscv64": {elf.ELFCLASS64, elf.ELFDATA2LSB, elf.EM_RISCV},
}

type (
	// BinaryRejectionKind classifies why a file cannot run as invowk inside a
	// Linux container.
	BinaryRejectionKind int

	// BinaryRejection says why a candidate invowk binary is not a Linux
	// executable for the container architecture. Its String is the reason
	// that provisioning warnings and errors repeat.
	BinaryRejection struct {
		Kind BinaryRejectionKind
		// Format names a recognised non-ELF format ("Mach-O", "PE"), when
		// Kind is BinaryRejectionNotELF.
		Format string //goplint:ignore -- display name of a recognised file format, for messages only.
		// Detail is the OS ABI, ELF type or read/parse error behind the
		// rejection, when there is one.
		Detail string //goplint:ignore -- OS ABI, ELF type or error text, for messages only.
		// BinaryArchitecture is the binary's architecture, when Kind is
		// BinaryRejectionMachine.
		BinaryArchitecture container.ImageArchitecture
		// ContainerArchitecture is the architecture the binary was checked
		// against, when Kind is BinaryRejectionMachine.
		ContainerArchitecture container.ImageArchitecture
	}

	// elfMachine is the ELF class, byte order and machine of an architecture.
	elfMachine struct {
		class   elf.Class
		data    elf.Data
		machine elf.Machine
	}
)

// String returns the kind's name.
func (k BinaryRejectionKind) String() string {
	switch k {
	case BinaryRejectionUnreadable:
		return "unreadable"
	case BinaryRejectionNotELF:
		return "not-elf"
	case BinaryRejectionMalformedELF:
		return "malformed-elf"
	case BinaryRejectionOtherOS:
		return "other-os"
	case BinaryRejectionNotExecutable:
		return "not-executable"
	case BinaryRejectionMachine:
		return "machine"
	default:
		return fmt.Sprintf("kind(%d)", int(k))
	}
}

// Validate returns an error when the kind is not one of the declared kinds.
func (k BinaryRejectionKind) Validate() error {
	if k < BinaryRejectionUnreadable || k > BinaryRejectionMachine {
		return fmt.Errorf("invalid binary rejection kind %d", int(k))
	}
	return nil
}

// Validate returns an error when the kind, or an architecture that is set,
// is invalid.
func (r BinaryRejection) Validate() error {
	errs := []error{r.Kind.Validate()}
	if r.BinaryArchitecture != "" {
		errs = append(errs, r.BinaryArchitecture.Validate())
	}
	if r.ContainerArchitecture != "" {
		errs = append(errs, r.ContainerArchitecture.Validate())
	}
	return errors.Join(errs...)
}

// String returns the reason a user reads in provisioning messages.
func (r BinaryRejection) String() string {
	switch r.Kind {
	case BinaryRejectionUnreadable:
		return "cannot read the file: " + r.Detail
	case BinaryRejectionNotELF:
		if r.Format != "" {
			return fmt.Sprintf("not a Linux executable (%s file, not ELF)", r.Format)
		}
		return "not a Linux executable (not an ELF file)"
	case BinaryRejectionMalformedELF:
		return "not a Linux executable (truncated or malformed ELF file: " + r.Detail + ")"
	case BinaryRejectionOtherOS:
		return "not a Linux executable (ELF for another operating system, OS ABI " + r.Detail + ")"
	case BinaryRejectionNotExecutable:
		return "not an executable (ELF type " + r.Detail + ")"
	case BinaryRejectionMachine:
		return fmt.Sprintf("built for %s, container is %s", r.BinaryArchitecture, r.ContainerArchitecture)
	default:
		return r.Kind.String()
	}
}

// checkLinuxBinary reads the header of the file at path and returns nil when
// it is an ELF executable for Linux whose class, byte order and machine match
// arch. An unknown arch, or one outside linuxELFMachines, skips only the
// machine comparison. It never runs the file.
func checkLinuxBinary(path types.FilesystemPath, arch container.ImageArchitecture) *BinaryRejection {
	file, err := os.Open(string(path))
	if err != nil {
		return &BinaryRejection{Kind: BinaryRejectionUnreadable, Detail: err.Error()}
	}
	defer func() { _ = file.Close() }() // Read-only file; close error is irrelevant.

	var magic [machOMagicByteCount]byte
	n, err := io.ReadFull(file, magic[:])
	if err != nil && n == 0 && !errors.Is(err, io.EOF) {
		return &BinaryRejection{Kind: BinaryRejectionUnreadable, Detail: err.Error()}
	}
	if string(magic[:n]) != elf.ELFMAG {
		return &BinaryRejection{Kind: BinaryRejectionNotELF, Format: nonELFFormat(magic[:n])}
	}

	f, err := elf.NewFile(file)
	if err != nil {
		return &BinaryRejection{Kind: BinaryRejectionMalformedELF, Detail: err.Error()}
	}
	if f.OSABI != elf.ELFOSABI_NONE && f.OSABI != elf.ELFOSABI_LINUX {
		return &BinaryRejection{Kind: BinaryRejectionOtherOS, Detail: f.OSABI.String()}
	}
	if f.Type != elf.ET_EXEC && f.Type != elf.ET_DYN {
		return &BinaryRejection{Kind: BinaryRejectionNotExecutable, Detail: f.Type.String()}
	}
	want, known := linuxELFMachines[arch]
	if !known {
		return nil
	}
	got := elfMachine{class: f.Class, data: f.Data, machine: f.Machine}
	if got != want {
		return &BinaryRejection{
			Kind:                  BinaryRejectionMachine,
			BinaryArchitecture:    elfArchitecture(got),
			ContainerArchitecture: arch,
		}
	}
	return nil
}

// elfArchitecture names the architecture of an ELF identity: its Go name when
// the table has it, otherwise the ELF machine and class.
func elfArchitecture(m elfMachine) container.ImageArchitecture {
	for arch, known := range linuxELFMachines {
		if known == m {
			return arch
		}
	}
	//goplint:ignore -- display name built from ELF constants, never empty.
	return container.ImageArchitecture(fmt.Sprintf("%s/%s", m.machine, m.class))
}

// nonELFFormat names the executable format a file's first bytes show, or ""
// when they show none invowk knows.
//
//goplint:ignore -- raw file magic bytes, classified into a display name only.
func nonELFFormat(magic []byte) string {
	if len(magic) >= 2 && magic[0] == 'M' && magic[1] == 'Z' {
		return "PE"
	}
	if len(magic) < machOMagicByteCount {
		return ""
	}
	switch binary.BigEndian.Uint32(magic) {
	case machOMagic32, machOMagic64, machOCigam32, machOCigam64, machOFatMagic, machOFatCigam:
		return "Mach-O"
	default:
		return ""
	}
}
