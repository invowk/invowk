// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

const invowkmodCueFileName = "invowkmod.cue"

type (
	// ModuleMetadataParseError wraps an invowkmod.cue parse, schema, or
	// invowk_version validation failure encountered while loading a module.
	// It preserves both the formatted module summary for invowk validate/Load
	// callers and the underlying parse error for fail-closed CLI reporting.
	ModuleMetadataParseError struct {
		ModulePath *types.FilesystemPath
		Summary    string //goplint:ignore -- formatted ValidationResult summary text.
		Err        error
	}

	metadataValidation struct {
		meta     *Invowkmod
		parseErr error
	}
)

// Error implements the error interface.
func (e *ModuleMetadataParseError) Error() string {
	return "invalid module: " + e.Summary
}

// Unwrap returns the underlying invowkmod.cue parse/version error.
func (e *ModuleMetadataParseError) Unwrap() error {
	return e.Err
}

// Validate validates the parsed module metadata when present.
func (m metadataValidation) Validate() error {
	if m.meta != nil {
		return m.meta.Validate()
	}
	return nil
}

// Validate performs comprehensive validation of a module at the given path.
// Returns a ValidationResult with all issues found, or an error if the path
// cannot be accessed.
func Validate(modulePath types.FilesystemPath) (*ValidationResult, error) {
	result, _, err := validateWithMetadata(modulePath)
	return result, err
}

// validateWithMetadata performs module validation and returns parsed metadata
// from invowkmod.cue when available. Load() uses this to avoid parsing
// invowkmod.cue twice on the hot discovery path.
func validateWithMetadata(modulePath types.FilesystemPath) (*ValidationResult, metadataValidation, error) {
	validatedAbsPath, err := resolveValidatedModulePath(modulePath)
	if err != nil {
		return nil, metadataValidation{}, err
	}

	result := newValidationResult(validatedAbsPath)
	absPath := string(validatedAbsPath)

	canContinue, err := ensureModuleDirectory(result, absPath)
	if err != nil {
		return nil, metadataValidation{}, err
	}
	if !canContinue {
		return result, metadataValidation{}, nil
	}

	populateModuleName(result, absPath)
	metaResult := validateInvowkmodFile(result, absPath)
	validateOptionalInvowkfile(result, absPath)

	scanErr := scanModuleTree(result, absPath)
	if scanErr != nil {
		return nil, metadataValidation{}, scanErr
	}

	return result, metaResult, nil
}

func resolveValidatedModulePath(modulePath types.FilesystemPath) (types.FilesystemPath, error) {
	absPath, err := filepath.Abs(string(modulePath))
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return types.FilesystemPath(absPath), nil //goplint:ignore -- filepath.Abs returns a non-empty path on success.
}

func newValidationResult(modulePath types.FilesystemPath) *ValidationResult {
	return &ValidationResult{
		Valid:      true,
		ModulePath: modulePath,
		Issues:     []ValidationIssue{},
	}
}

//goplint:ignore -- validation helpers operate on OS-native paths derived from validated module roots.
func ensureModuleDirectory(result *ValidationResult, absPath string) (bool, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.AddIssue(IssueTypeStructure, "path does not exist", "")
			return false, nil
		}
		return false, fmt.Errorf("failed to stat path: %w", err)
	}
	if !info.IsDir() {
		result.AddIssue(IssueTypeStructure, "path is not a directory", "")
		return false, nil
	}
	return true, nil
}

//goplint:ignore -- validation helpers operate on OS-native paths derived from validated module roots.
func populateModuleName(result *ValidationResult, absPath string) {
	moduleName, err := ParseModuleName(filepath.Base(absPath))
	if err != nil {
		result.AddIssue(IssueTypeNaming, err.Error(), "")
		return
	}

	result.ModuleName = moduleName
	if string(moduleName) == "invowkfile" {
		result.AddIssue(IssueTypeNaming, "module name 'invowkfile' is reserved for the root invowkfile source", "")
	}
}

//goplint:ignore -- validation helpers operate on OS-native paths derived from validated module roots.
func validateInvowkmodFile(result *ValidationResult, absPath string) metadataValidation {
	invowkmodPath := filepath.Join(absPath, invowkmodCueFileName)
	invowkmodInfo, err := os.Stat(invowkmodPath)
	switch {
	case os.IsNotExist(err):
		result.AddIssue(IssueTypeStructure, "missing required invowkmod.cue", "")
		return metadataValidation{}
	case err != nil:
		result.AddIssue(IssueTypeStructure, fmt.Sprintf("cannot access invowkmod.cue: %v", err), "")
		return metadataValidation{}
	case invowkmodInfo.IsDir():
		result.AddIssue(IssueTypeStructure, "invowkmod.cue must be a file, not a directory", "")
		return metadataValidation{}
	}

	invowkmodFSPath := types.FilesystemPath(invowkmodPath)
	if err := invowkmodFSPath.Validate(); err != nil {
		result.AddIssue(IssueTypeStructure, fmt.Sprintf("invalid invowkmod.cue path: %v", err), "")
		return metadataValidation{}
	}
	result.InvowkmodPath = invowkmodFSPath
	if result.ModuleName == "" {
		return metadataValidation{}
	}

	meta, parseErr := ParseInvowkmod(invowkmodFSPath)
	switch {
	case parseErr != nil:
		result.AddIssue(IssueTypeInvowkmod, fmt.Sprintf("failed to parse invowkmod.cue: %v", parseErr), invowkmodCueFileName)
		return metadataValidation{parseErr: parseErr}
	case string(meta.Module) != string(result.ModuleName):
		result.AddIssue(
			IssueTypeNaming,
			fmt.Sprintf("module field '%s' in invowkmod.cue must match folder name '%s'", meta.Module, result.ModuleName),
			invowkmodCueFileName,
		)
		return metadataValidation{}
	default:
		return metadataValidation{meta: meta}
	}
}

//goplint:ignore -- validation helpers operate on OS-native paths derived from validated module roots.
func validateOptionalInvowkfile(result *ValidationResult, absPath string) {
	invowkfilePath := filepath.Join(absPath, "invowkfile.cue")
	invowkfileInfo, err := os.Stat(invowkfilePath)
	switch {
	case os.IsNotExist(err):
		result.IsLibraryOnly = true
	case err != nil:
		result.AddIssue(IssueTypeStructure, fmt.Sprintf("cannot access invowkfile.cue: %v", err), "")
	case invowkfileInfo.IsDir():
		result.AddIssue(IssueTypeStructure, "invowkfile.cue must be a file, not a directory", "")
	default:
		resolvedPath := types.FilesystemPath(invowkfilePath)
		if err := resolvedPath.Validate(); err != nil {
			result.AddIssue(IssueTypeStructure, fmt.Sprintf("invalid invowkfile.cue path: %v", err), "")
			return
		}
		result.InvowkfilePath = resolvedPath
	}
}

//goplint:ignore -- validation helpers operate on OS-native paths derived from validated module roots.
func scanModuleTree(result *ValidationResult, absPath string) error {
	if err := filepath.WalkDir(absPath, func(path string, d os.DirEntry, entryErr error) error {
		if entryErr == nil {
			return inspectModuleEntry(result, absPath, path, d)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to walk module directory: %w", err)
	}
	return nil
}

//goplint:ignore -- validation helpers operate on OS-native paths derived from validated module roots.
func inspectModuleEntry(result *ValidationResult, absPath, entryPath string, entry os.DirEntry) error {
	if entryPath == absPath {
		return nil
	}
	if entry.IsDir() && entry.Name() == VendoredModulesDir {
		return filepath.SkipDir
	}
	if entry.Type()&os.ModeSymlink != 0 {
		recordSymlinkIssue(result, absPath, entryPath)
	}
	if entry.IsDir() && strings.HasSuffix(entry.Name(), ModuleSuffix) {
		recordModuleTreeIssue(result, absPath, entryPath, IssueTypeStructure, "nested modules are not allowed (except in invowk_modules/)")
	}
	if platform.IsWindowsReservedName(entry.Name()) {
		recordModuleTreeIssue(result, absPath, entryPath, IssueTypeCompatibility, fmt.Sprintf("filename '%s' is reserved on Windows", entry.Name()))
	}
	return nil
}

//goplint:ignore -- validation helpers operate on OS-native paths derived from validated module roots.
func recordSymlinkIssue(result *ValidationResult, absPath, entryPath string) {
	relPath, _ := filepath.Rel(absPath, entryPath)
	linkTarget, readErr := os.Readlink(entryPath)
	if readErr != nil {
		result.AddIssue(IssueTypeSecurity, "cannot read symlink target", relPath)
		return
	}

	resolvedTarget := resolveModuleSymlinkTarget(entryPath, linkTarget)
	relToRoot, relErr := filepath.Rel(absPath, resolvedTarget)
	if relErr != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		result.AddIssue(IssueTypeSecurity, fmt.Sprintf("symlink points outside module directory (target: %s)", linkTarget), relPath)
		return
	}

	result.AddIssue(IssueTypeSecurity, "symlinks are not allowed in modules (security risk during extraction)", relPath)
}

//goplint:ignore -- validation helpers operate on OS-native symlink targets from the filesystem.
func resolveModuleSymlinkTarget(entryPath, linkTarget string) string {
	if filepath.IsAbs(linkTarget) {
		return linkTarget
	}
	return filepath.Clean(filepath.Join(filepath.Dir(entryPath), linkTarget))
}

//goplint:ignore -- validation helpers operate on OS-native paths derived from validated module roots.
func recordModuleTreeIssue(result *ValidationResult, absPath, entryPath string, issueType ValidationIssueType, message string) {
	relPath, _ := filepath.Rel(absPath, entryPath)
	result.AddIssue(issueType, message, relPath)
}

// Load loads and validates a module at the given path.
// Returns a Module (operational wrapper) if valid, or an error with validation details.
// Note: This loads only metadata (invowkmod.cue), not commands (invowkfile.cue).
// To load commands as well, use pkg/invowkfile.ParseModule().
func Load(modulePath types.FilesystemPath) (*Module, error) {
	result, metaResult, err := validateWithMetadata(modulePath)
	if err != nil {
		return nil, err
	}

	if !result.Valid {
		// Collect all issues into error message
		var msgs []string
		for _, issue := range result.Issues {
			msgs = append(msgs, issue.Error())
		}
		summary := strings.Join(msgs, "; ")
		if metaResult.parseErr != nil {
			modPath := result.ModulePath
			return nil, &ModuleMetadataParseError{
				ModulePath: &modPath,
				Summary:    summary,
				Err:        metaResult.parseErr,
			}
		}
		return nil, fmt.Errorf("invalid module: %s", summary)
	}

	// Metadata is parsed during validation when invowkmod.cue is valid.
	if metaResult.meta == nil {
		return nil, errors.New("failed to parse module metadata: invowkmod.cue validation did not produce metadata")
	}

	return &Module{
		Metadata:      metaResult.meta,
		Path:          result.ModulePath,
		IsLibraryOnly: result.IsLibraryOnly,
	}, nil
}
