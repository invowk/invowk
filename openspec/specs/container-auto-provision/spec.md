# container-auto-provision Specification

## Purpose
Defines what container auto-provision installs into a provisioned image, and how it decides whether the invowk binary it would install can run inside a Linux container.

## Requirements

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

### Requirement: An architecture invowk does not know skips the machine comparison
When the engine reports an architecture for the base image that invowk does not know, the machine comparison SHALL be skipped, and the host architecture SHALL NOT be assumed in its place. The other checks of the binary SHALL still apply.

#### Scenario: Engine reports an architecture invowk does not know
- **WHEN** the engine reports the base image's architecture as mips64le, the host is amd64 Linux, and the candidate binary is a Linux ELF executable for amd64
- **THEN** the binary is installed without a machine comparison, and amd64 is not assumed

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

### Requirement: The skip warning is shown only by runs that prepare an image
The skip warning SHALL be shown by every run that prepares a provisioned image, whether it builds the image or reuses a cached one. A run that reuses an existing persistent container prepares no image and SHALL show no skip warning; the run that prepared that container's image showed it.

#### Scenario: Existing persistent container reused
- **WHEN** a run reuses an existing persistent container, so no image is prepared, and the default binary would be skipped
- **THEN** the command runs and no provisioning warning is shown

### Requirement: An unusable explicit binary_path fails the command
When `container.auto_provision.binary_path` is set and the file it names does not exist, cannot be read, or is not usable, the container command SHALL fail before any provisioned image is built, with an error that names the configured path and the reason. This SHALL hold whether `strict` is true or false; the command SHALL NOT fall back to the base image.

#### Scenario: Explicit Mach-O binary
- **WHEN** `binary_path` names a Mach-O file and `strict` is false
- **THEN** the command fails with an error naming the path and saying it is not a Linux executable, and no provisioned image is built

#### Scenario: Explicit binary for another architecture
- **WHEN** `binary_path` names a Linux ELF executable for amd64 and the engine reports the base image as arm64
- **THEN** the command fails with an error naming the path, its architecture and the container architecture

#### Scenario: Explicit path missing
- **WHEN** `binary_path` names a file that does not exist and `strict` is false
- **THEN** the command fails with an error naming the path, instead of running the base image with a warning

#### Scenario: Persistent container identity
- **WHEN** a persistent container's provisioned image identity is computed and `binary_path` names a file that is missing, unreadable or not a Linux ELF executable, or one whose architecture differs from the architecture the engine reports
- **THEN** the command fails with the same error

### Requirement: A rejection against an assumed architecture names the recovery
When an explicit `binary_path` is rejected only because its architecture differs from a host architecture that was assumed because the engine could not report the base image, the error SHALL say that the architecture was assumed, and SHALL tell the user to make the image present in the engine first (pull it, or build its Containerfile) and run again.

#### Scenario: Explicit binary against an assumed architecture
- **WHEN** the host is amd64 Linux, the engine cannot report the base image's architecture, and `binary_path` names a Linux ELF executable for arm64
- **THEN** the command fails with an error that names the path, says amd64 was assumed because the engine does not have the image, and tells the user to pull or build the image first and run again

#### Scenario: Explicit binary after the image is made present
- **WHEN** after that failure the user pulls the image, and the engine now reports the base image as arm64
- **THEN** the next run installs the arm64 binary

### Requirement: Cached provisioned images reflect whether the binary is installed
A provisioned image built without the invowk binary SHALL NOT be reused for a run that would install a usable binary, and the reverse SHALL hold too.

#### Scenario: binary_path added after a skipped run
- **WHEN** a non-persistent run on a macOS host cached an image without the binary, and the user then sets `binary_path` to a usable Linux build
- **THEN** the next run builds and uses a provisioned image that contains that binary

### Requirement: Persistent container identity follows the same rule without assuming the host architecture
A persistent container's provisioned image identity SHALL be computed by the same rule from the same configuration as the image the run builds, except that it SHALL NOT assume the host architecture: when the engine cannot report the base image, the identity SHALL skip the machine comparison and leave it to the provisioning of the image. Once the base image is present in the engine, the identity SHALL equal the tag of the image the run builds.

#### Scenario: Persistent identity matches the built image
- **WHEN** a persistent container command runs, the base image is present in the engine, and the default binary is skipped
- **THEN** the provisioned image identity it uses is the tag of the image built without the binary

#### Scenario: Persistent container on a cross-architecture Containerfile not yet built
- **WHEN** the host is amd64 Linux, a persistent container command uses a Containerfile whose base is for arm64 and has not been built, and `binary_path` names a Linux ELF executable for arm64
- **THEN** the identity step does not reject the binary, the base image is built, and the command runs in a provisioned image that contains the binary

#### Scenario: Persistent container after the binary decision changes
- **WHEN** a managed persistent container was created from a provisioned image and the binary decision for its configuration later changes (for example `binary_path` is set, or the engine now reports a different architecture)
- **THEN** the run fails with the existing managed-container drift error until the container is removed, and does not keep using the old identity
