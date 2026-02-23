// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validateRuntimeConfig checks that a runtime configuration is valid.
// Note: Format validation (non-empty interpreter, valid env var names) is handled by the CUE schema.
// This function focuses on Go-only validations: cross-field logic, filesystem access, and security checks.
func validateRuntimeConfig(rt *RuntimeConfig, cmdName string, implIndex int) error {
	// [CUE-VALIDATED] Interpreter format validation is in CUE schema:
	// interpreter?: string & =~"^\\s*\\S.*$" (requires at least one non-whitespace char)

	// Validate env inherit mode and env var names
	if rt.EnvInheritMode != "" {
		if isValid, _ := rt.EnvInheritMode.IsValid(); !isValid {
			return fmt.Errorf("command '%s' implementation #%d: env_inherit_mode must be one of: none, allow, all", cmdName, implIndex)
		}
	}
	for _, name := range rt.EnvInheritAllow {
		if isValid, errs := name.IsValid(); !isValid {
			return fmt.Errorf("command '%s' implementation #%d: env_inherit_allow: %w", cmdName, implIndex, errs[0])
		}
	}
	for _, name := range rt.EnvInheritDeny {
		if isValid, errs := name.IsValid(); !isValid {
			return fmt.Errorf("command '%s' implementation #%d: env_inherit_deny: %w", cmdName, implIndex, errs[0])
		}
	}

	// [GO-ONLY] Cross-field validation: Container-specific fields are only valid for container runtime.
	// CUE uses discriminated unions (#RuntimeConfigNative | #RuntimeConfigVirtual | #RuntimeConfigContainer)
	// which handle field presence at the type level. This Go validation provides clearer error messages
	// and catches any edge cases where the CUE type system might be bypassed.
	if rt.Name != RuntimeContainer {
		if rt.EnableHostSSH {
			return fmt.Errorf("command '%s' implementation #%d: enable_host_ssh is only valid for container runtime", cmdName, implIndex)
		}
		if rt.Containerfile != "" {
			return fmt.Errorf("command '%s' implementation #%d: containerfile is only valid for container runtime", cmdName, implIndex)
		}
		if rt.Image != "" {
			return fmt.Errorf("command '%s' implementation #%d: image is only valid for container runtime", cmdName, implIndex)
		}
		if len(rt.Volumes) > 0 {
			return fmt.Errorf("command '%s' implementation #%d: volumes is only valid for container runtime", cmdName, implIndex)
		}
		if len(rt.Ports) > 0 {
			return fmt.Errorf("command '%s' implementation #%d: ports is only valid for container runtime", cmdName, implIndex)
		}
	} else {
		// For container runtime, validate mutual exclusivity of containerfile and image
		if rt.Containerfile != "" && rt.Image != "" {
			return fmt.Errorf("command '%s' implementation #%d: containerfile and image are mutually exclusive - specify only one", cmdName, implIndex)
		}
		// At least one of containerfile or image must be specified for container runtime
		if rt.Containerfile == "" && rt.Image == "" {
			return fmt.Errorf("command '%s' implementation #%d: container runtime requires either containerfile or image to be specified", cmdName, implIndex)
		}
		// Validate container image name format
		if rt.Image != "" {
			if err := ValidateContainerImage(string(rt.Image)); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: invalid image: %w", cmdName, implIndex, err)
			}
		}
		// Validate containerfile path for security (path traversal prevention)
		// Note: baseDir validation is done at parse time when FilePath is available
		if rt.Containerfile != "" {
			cfStr := string(rt.Containerfile)
			if len(cfStr) > MaxPathLength {
				return fmt.Errorf("command '%s' implementation #%d: containerfile path too long (%d chars, max %d)", cmdName, implIndex, len(cfStr), MaxPathLength)
			}
			if filepath.IsAbs(cfStr) {
				return fmt.Errorf("command '%s' implementation #%d: containerfile path must be relative, not absolute", cmdName, implIndex)
			}
			if strings.ContainsRune(cfStr, '\x00') {
				return fmt.Errorf("command '%s' implementation #%d: containerfile path contains null byte", cmdName, implIndex)
			}
		}
		// Validate volume mounts
		for i, vol := range rt.Volumes {
			if err := ValidateVolumeMount(string(vol)); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: volume #%d: %w", cmdName, implIndex, i+1, err)
			}
		}
		// Validate port mappings
		for i, port := range rt.Ports {
			if err := ValidatePortMapping(string(port)); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: port #%d: %w", cmdName, implIndex, i+1, err)
			}
		}
	}
	return nil
}
