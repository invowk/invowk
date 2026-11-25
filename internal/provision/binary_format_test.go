// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"debug/elf"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/types"
)

// hostFixtureArch is the host's Go architecture when the ELF table has it,
// otherwise amd64. On a host outside the table the assumed architecture skips
// the machine comparison, so an amd64 fixture is accepted there too.
func hostFixtureArch() string {
	if _, ok := linuxELFMachines[container.ImageArchitecture(goruntime.GOARCH)]; ok {
		return goruntime.GOARCH
	}
	return "amd64"
}

// writeHostLinuxBinary writes a Linux ELF executable fixture that the
// provisioner accepts on the test host when the engine reports nothing.
func writeHostLinuxBinary(t *testing.T, path string, trailer []byte) types.FilesystemPath {
	t.Helper()
	return types.FilesystemPath(testutil.WriteLinuxELFFixture(t, path, hostFixtureArch(), trailer))
}

func TestCheckLinuxBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	at := func(name string) string { return filepath.Join(dir, name) }

	elfWith := func(name string, edit func(*testutil.ELFFixture)) string {
		f := testutil.LinuxELFExecutable(t, "amd64")
		edit(&f)
		return testutil.WriteELFFixture(t, at(name), f)
	}

	amd64 := testutil.WriteLinuxELFFixture(t, at("amd64"), "amd64", nil)
	arm64 := testutil.WriteLinuxELFFixture(t, at("arm64"), "arm64", nil)
	s390x := testutil.WriteLinuxELFFixture(t, at("s390x"), "s390x", nil)
	i386 := testutil.WriteLinuxELFFixture(t, at("386"), "386", nil)
	machO := testutil.WriteMachOFixture(t, at("macho"))
	peFile := testutil.WritePEFixture(t, at("pe"))
	freeBSD := elfWith("freebsd", func(f *testutil.ELFFixture) { f.OSABI = elf.ELFOSABI_FREEBSD })
	linuxABI := elfWith("linux-abi", func(f *testutil.ELFFixture) { f.OSABI = elf.ELFOSABI_LINUX })
	pie := elfWith("pie", func(f *testutil.ELFFixture) { f.Type = elf.ET_DYN })
	relocatable := elfWith("relocatable", func(f *testutil.ELFFixture) { f.Type = elf.ET_REL })
	headerOnly := elfWith("header-only", func(f *testutil.ELFFixture) { f.HeaderOnly = true })
	truncated := elfWith("truncated", func(f *testutil.ELFFixture) { f.TruncateTo = 20 })
	mips := elfWith("mips", func(f *testutil.ELFFixture) { f.Machine = elf.EM_MIPS })
	empty := testutil.WriteEmptyFixture(t, at("empty"))
	script := testutil.WriteShellScriptFixture(t, at("script"))
	missing := at("missing")

	tests := []struct {
		name       string
		path       string
		arch       container.ImageArchitecture
		wantKind   BinaryRejectionKind
		wantFormat string
		wantBinary container.ImageArchitecture
		wantReason string
	}{
		{name: "ELF amd64 against amd64", path: amd64, arch: "amd64"},
		{name: "ELF arm64 against arm64", path: arm64, arch: "arm64"},
		{name: "ELF s390x big-endian against s390x", path: s390x, arch: "s390x"},
		{name: "Linux OS ABI", path: linuxABI, arch: "amd64"},
		{name: "PIE", path: pie, arch: "amd64"},
		{name: "header-only ELF without section headers", path: headerOnly, arch: "amd64"},
		{name: "unknown architecture accepts any Linux ELF", path: arm64, arch: container.ImageArchitectureUnknown},
		{name: "architecture outside the table accepts any Linux ELF", path: amd64, arch: "mips64le"},
		{
			name: "ELF arm64 against amd64", path: arm64, arch: "amd64",
			wantKind: BinaryRejectionMachine, wantBinary: "arm64", wantReason: "built for arm64, container is amd64",
		},
		{
			name: "ELF amd64 against arm64", path: amd64, arch: "arm64",
			wantKind: BinaryRejectionMachine, wantBinary: "amd64", wantReason: "built for amd64, container is arm64",
		},
		{
			name: "32-bit ELF against amd64", path: i386, arch: "amd64",
			wantKind: BinaryRejectionMachine, wantBinary: "386",
		},
		{
			name: "ELF machine outside the table", path: mips, arch: "amd64",
			wantKind: BinaryRejectionMachine, wantBinary: "EM_MIPS/ELFCLASS64",
		},
		{
			name: "Mach-O", path: machO, arch: "amd64",
			wantKind: BinaryRejectionNotELF, wantFormat: "Mach-O", wantReason: "not a Linux executable (Mach-O file, not ELF)",
		},
		{
			name: "Mach-O against an unknown architecture", path: machO, arch: container.ImageArchitectureUnknown,
			wantKind: BinaryRejectionNotELF, wantFormat: "Mach-O",
		},
		{
			name: "PE", path: peFile, arch: "amd64",
			wantKind: BinaryRejectionNotELF, wantFormat: "PE", wantReason: "not a Linux executable (PE file, not ELF)",
		},
		{
			name: "FreeBSD OS ABI", path: freeBSD, arch: "amd64",
			wantKind: BinaryRejectionOtherOS, wantReason: "ELF for another operating system, OS ABI ELFOSABI_FREEBSD",
		},
		{
			name: "relocatable object", path: relocatable, arch: "amd64",
			wantKind: BinaryRejectionNotExecutable, wantReason: "not an executable (ELF type ET_REL)",
		},
		{
			name: "truncated ELF", path: truncated, arch: "amd64",
			wantKind: BinaryRejectionMalformedELF, wantReason: "truncated or malformed ELF file",
		},
		{
			name: "empty file", path: empty, arch: "amd64",
			wantKind: BinaryRejectionNotELF, wantReason: "not a Linux executable (not an ELF file)",
		},
		{
			name: "shell script", path: script, arch: "amd64",
			wantKind: BinaryRejectionNotELF, wantReason: "not a Linux executable (not an ELF file)",
		},
		{
			name: "missing file", path: missing, arch: "amd64",
			wantKind: BinaryRejectionUnreadable, wantReason: "cannot read the file",
		},
		{
			name: "directory", path: dir, arch: "amd64",
			wantKind: BinaryRejectionUnreadable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := checkLinuxBinary(types.FilesystemPath(tt.path), tt.arch)
			if tt.wantKind == 0 {
				if got != nil {
					t.Fatalf("checkLinuxBinary() = %+v (%s), want usable", *got, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("checkLinuxBinary() = usable, want rejection %s", tt.wantKind)
			}
			if got.Kind != tt.wantKind {
				t.Fatalf("checkLinuxBinary() kind = %s (%s), want %s", got.Kind, got, tt.wantKind)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("rejection Validate() = %v", err)
			}
			if got.Format != tt.wantFormat {
				t.Errorf("Format = %q, want %q", got.Format, tt.wantFormat)
			}
			if tt.wantBinary != "" {
				if got.BinaryArchitecture != tt.wantBinary || got.ContainerArchitecture != tt.arch {
					t.Errorf("architectures = %q/%q, want %q/%q", got.BinaryArchitecture, got.ContainerArchitecture, tt.wantBinary, tt.arch)
				}
			}
			if !strings.Contains(got.String(), tt.wantReason) {
				t.Errorf("String() = %q, want it to contain %q", got.String(), tt.wantReason)
			}
		})
	}
}

func TestBinaryRejectionKindValidate(t *testing.T) {
	t.Parallel()

	for k := BinaryRejectionUnreadable; k <= BinaryRejectionMachine; k++ {
		if err := k.Validate(); err != nil {
			t.Errorf("BinaryRejectionKind(%d).Validate() = %v", k, err)
		}
		if strings.HasPrefix(k.String(), "kind(") {
			t.Errorf("BinaryRejectionKind(%d).String() = %q, want a name", k, k.String())
		}
	}
	for _, k := range []BinaryRejectionKind{0, BinaryRejectionMachine + 1} {
		if err := k.Validate(); err == nil {
			t.Errorf("BinaryRejectionKind(%d).Validate() = nil, want error", k)
		}
	}
}
