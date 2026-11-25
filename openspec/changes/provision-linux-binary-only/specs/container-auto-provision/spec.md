# Spec Delta

## Purpose

Defines what container auto-provision installs into a provisioned image, and how it decides whether the invowk binary it would install can run inside a Linux container.

## ADDED Requirements

### Requirement: Only a Linux binary for the container architecture is installed
Auto-provision SHALL install the invowk binary into a provisioned image only when the file is an ELF executable for Linux whose machine matches the container architecture. The decision SHALL be made from the file's header, never from the host operating system alone. Any other file (Mach-O, PE, an ELF for another machine or operating system, an ELF that is not an executable, a truncated or unreadable file) SHALL be treated as not usable.

#### Scenario: Linux binary for the image architecture
- **WHEN** the candidate binary is a Linux ELF executable for amd64 and the container architecture is amd64
- **THEN** the binary is installed in the provisioned image and its directory is on the container's `PATH`

#### Scenario: Mach-O binary
- **WHEN** the candidate binary is a Mach-O file
- **THEN** the binary is treated as not usable

#### Scenario: PE binary
- **WHEN** the candidate binary is a PE file
- **THEN** the binary is treated as not usable

#### Scenario: ELF for another architecture
- **WHEN** the candidate binary is a Linux ELF executable for arm64 and the container architecture is amd64
- **THEN** the binary is treated as not usable

#### Scenario: ELF for another operating system
- **WHEN** the candidate binary is an ELF executable whose OS ABI is FreeBSD
- **THEN** the binary is treated as not usable

#### Scenario: Truncated or non-executable file
- **WHEN** the candidate binary is shorter than an ELF header, or is an ELF relocatable object
- **THEN** the binary is treated as not usable

#### Scenario: Linux binary without section headers
- **WHEN** the candidate binary is a Linux ELF executable for the container architecture that has no section headers, as a UPX-compressed release binary has
- **THEN** the binary is installed

### Requirement: Container architecture comes from the engine or a Linux host
The container architecture SHALL be the architecture the engine reports for the base image. When the engine cannot report it, the host architecture SHALL be assumed only when the host is Linux; on any other host the architecture SHALL be unknown, and a Linux ELF executable SHALL then be accepted for any architecture. Determining the architecture SHALL NOT pull an image or start a container.

#### Scenario: Engine reports the image architecture
- **WHEN** the base image is present in the engine and records architecture arm64, and the host is amd64 Linux
- **THEN** the container architecture is arm64

#### Scenario: Image unknown to the engine on a Linux host
- **WHEN** the engine cannot report the base image's architecture and the host is amd64 Linux
- **THEN** the container architecture is amd64

#### Scenario: Image unknown to the engine on a non-Linux host
- **WHEN** the engine cannot report the base image's architecture, the host is macOS, and `binary_path` names a Linux ELF executable for arm64
- **THEN** the binary is installed without an architecture check

#### Scenario: No extra engine work
- **WHEN** the container architecture is determined for a base image that is not present in the engine
- **THEN** no image is pulled and no container is started for that purpose

### Requirement: An unusable default binary is skipped with a warning
When `container.auto_provision.binary_path` is not set and the running invowk executable is not usable, auto-provision SHALL build the provisioned image without the binary and the command SHALL run. A warning SHALL say that nested invowk commands will not work in the container, give the reason and the container architecture (or that it is unknown), and tell the user to set `container.auto_provision.binary_path` to a Linux build for that architecture. `strict` SHALL NOT turn this into an error.

#### Scenario: macOS host without binary_path
- **WHEN** a container command runs on a macOS host with auto-provision enabled and no `binary_path`
- **THEN** the command runs in a provisioned image that contains the configured modules but no invowk binary, and a warning names `container.auto_provision.binary_path`

#### Scenario: Container PATH without the binary
- **WHEN** the default binary is skipped
- **THEN** the provisioned image and the container environment do not put the invowk binary directory on `PATH`

#### Scenario: Strict mode
- **WHEN** the default binary is skipped and `container.auto_provision.strict` is true
- **THEN** the command still runs, with the same warning

#### Scenario: Cached image
- **WHEN** a later run reuses the cached provisioned image that was built without the binary
- **THEN** the same warning is shown again

### Requirement: An unusable explicit binary_path fails the command
When `container.auto_provision.binary_path` is set and the file it names does not exist, cannot be read, or is not usable, the container command SHALL fail before any provisioned image is built, with an error that names the configured path and the reason. This SHALL hold whether `strict` is true or false; the command SHALL NOT fall back to the base image.

#### Scenario: Explicit Mach-O binary
- **WHEN** `binary_path` names a Mach-O file and `strict` is false
- **THEN** the command fails with an error naming the path and saying it is not a Linux executable, and no image is built

#### Scenario: Explicit binary for another architecture
- **WHEN** `binary_path` names a Linux ELF executable for amd64 and the engine reports the base image as arm64
- **THEN** the command fails with an error naming the path, its architecture and the container architecture

#### Scenario: Explicit path missing
- **WHEN** `binary_path` names a file that does not exist and `strict` is false
- **THEN** the command fails with an error naming the path, instead of running the base image with a warning

#### Scenario: Persistent container identity
- **WHEN** a persistent container's provisioned image identity is computed and `binary_path` is not usable
- **THEN** the command fails with the same error

### Requirement: Cached provisioned images reflect whether the binary is installed
A provisioned image built without the invowk binary SHALL NOT be reused for a run that would install a usable binary, and the reverse SHALL hold too. A persistent container's provisioned image identity SHALL be computed from the same binary decision as the image the run builds.

#### Scenario: binary_path added after a skipped run
- **WHEN** a run on a macOS host cached an image without the binary, and the user then sets `binary_path` to a usable Linux build
- **THEN** the next run builds and uses a provisioned image that contains that binary

#### Scenario: Persistent identity matches the built image
- **WHEN** a persistent container command runs and the default binary is skipped
- **THEN** the provisioned image identity it uses is the tag of the image built without the binary
