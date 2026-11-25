# Spec Delta

## ADDED Requirements

### Requirement: The installed binary is the one that was checked
When a provisioned image is built with the invowk binary, the bytes installed in the image SHALL be bytes that passed the Linux-binary check and whose hash is the one in the image's cache key. If the file at `binary_path`, or at the default binary's path, changes or is replaced while invowk provisions, so that the bytes the build would install fail the check or differ from the hashed ones, the build SHALL fail with an error that names the path, and no image SHALL be tagged for it.

#### Scenario: Binary replaced after the cache key was computed
- **GIVEN** `binary_path` names a usable Linux build for the container architecture
- **WHEN** the file at `binary_path` is replaced by another file (a macOS build, a script, or a different Linux build) after invowk computed the image's cache key and before the image build copies the binary
- **THEN** the run SHALL fail with an error naming `binary_path` and saying that the binary changed during provisioning, and no provisioned image SHALL be created

#### Scenario: Binary replaced after the check by a file that is not usable
- **GIVEN** `binary_path` names a usable Linux build for the container architecture
- **WHEN** the file at `binary_path` is replaced by a macOS build after invowk checked it and before it computed the cache key
- **THEN** the run SHALL fail with an error naming `binary_path`, and no provisioned image SHALL contain the macOS build

#### Scenario: Binary unchanged
- **WHEN** the file at `binary_path` does not change during the run
- **THEN** the image SHALL be built and tagged as before this change, with the same cache key for the same bytes, base image and modules

#### Scenario: Cached image with an unchanged binary
- **GIVEN** a provisioned image cached for the current binary
- **WHEN** a later run finds the binary unchanged
- **THEN** the run SHALL reuse the cached image without copying the binary into a build context
