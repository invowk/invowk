// SPDX-License-Identifier: MPL-2.0

package invowkfile

// ValidateScriptPathContainmentForTest exposes the unexported
// validateScriptPathContainment to external test packages inside pkg/invowkfile
// (formal-verification golden replays). It is a test-only hook: no production
// code depends on it, and it binds the containment layer directly so a defect
// masked by ScriptFilePath.Validate at every exported entry point is still
// calibrated (design D5.3).
func ValidateScriptPathContainmentForTest(scriptPath, modulePath FilesystemPath) error {
	return validateScriptPathContainment(scriptPath, modulePath)
}
