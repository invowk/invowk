// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/invowk/invowk/pkg/types"
)

const (
	invalidCustomCheckScriptErrMsg       = "invalid custom check script"
	missingCustomCheckScriptSourceErrMsg = "custom check script must set content or file"
	mixedCustomCheckScriptSourceErrMsg   = "custom check script must not set both content and file"
)

var (
	// ErrInvalidBinaryName is the sentinel error wrapped by InvalidBinaryNameError.
	ErrInvalidBinaryName = errors.New("invalid binary name")
	// ErrInvalidCheckName is the sentinel error wrapped by InvalidCheckNameError.
	ErrInvalidCheckName = errors.New("invalid check name")
	// ErrInvalidScriptContent is the sentinel error wrapped by InvalidScriptContentError.
	ErrInvalidScriptContent = errors.New("invalid script content")
	// ErrInvalidToolDependency is the sentinel error wrapped by InvalidToolDependencyError.
	ErrInvalidToolDependency = errors.New("invalid tool dependency")
	// ErrInvalidCommandDependency is the sentinel error wrapped by InvalidCommandDependencyError.
	ErrInvalidCommandDependency = errors.New("invalid command dependency")
	// ErrInvalidCommandDependencyRef is the sentinel error wrapped by InvalidCommandDependencyRefError.
	ErrInvalidCommandDependencyRef = errors.New("invalid command dependency reference")
	// ErrInvalidCommandDependencySourceID is the sentinel error wrapped by InvalidCommandDependencySourceIDError.
	ErrInvalidCommandDependencySourceID = errors.New("invalid command dependency source id")
	// ErrInvalidCapabilityDependency is the sentinel error wrapped by InvalidCapabilityDependencyError.
	ErrInvalidCapabilityDependency = errors.New("invalid capability dependency")
	// ErrInvalidEnvVarCheck is the sentinel error wrapped by InvalidEnvVarCheckError.
	ErrInvalidEnvVarCheck = errors.New("invalid env var check")
	// ErrInvalidEnvVarDependency is the sentinel error wrapped by InvalidEnvVarDependencyError.
	ErrInvalidEnvVarDependency = errors.New("invalid env var dependency")
	// ErrInvalidFilepathDependency is the sentinel error wrapped by InvalidFilepathDependencyError.
	ErrInvalidFilepathDependency = errors.New("invalid filepath dependency")
	// ErrInvalidCustomCheck is the sentinel error wrapped by InvalidCustomCheckError.
	ErrInvalidCustomCheck = errors.New("invalid custom check")
	// ErrInvalidCustomCheckScript is the sentinel error wrapped by InvalidCustomCheckScriptError.
	ErrInvalidCustomCheckScript = errors.New(invalidCustomCheckScriptErrMsg)
	// ErrMissingCustomCheckScriptSource is returned when a custom check script selects no source.
	ErrMissingCustomCheckScriptSource = errors.New(missingCustomCheckScriptSourceErrMsg)
	// ErrMixedCustomCheckScriptSource is returned when a custom check script selects both sources.
	ErrMixedCustomCheckScriptSource = errors.New(mixedCustomCheckScriptSourceErrMsg)
	// ErrInvalidCustomCheckDependency is the sentinel error wrapped by InvalidCustomCheckDependencyError.
	ErrInvalidCustomCheckDependency = errors.New("invalid custom check dependency")
	// ErrInvalidDependsOn is the sentinel error wrapped by InvalidDependsOnError.
	ErrInvalidDependsOn = errors.New("invalid depends_on")
	// ErrMissingDependencyAlternatives is returned when an OR dependency has no alternatives.
	ErrMissingDependencyAlternatives = errors.New("dependency alternatives must contain at least one item")
	// ErrMixedCustomCheckDependency is returned when a custom check dependency
	// combines direct check fields with an alternatives list.
	ErrMixedCustomCheckDependency = errors.New("custom check dependency must use either direct fields or alternatives")
)

type (
	// BinaryName represents the name of an executable binary expected in PATH.
	// Must be non-empty, start with an alphanumeric character, and contain only
	// alphanumeric characters plus '.', '_', '+', or '-'.
	BinaryName string

	// InvalidBinaryNameError is returned when a BinaryName value is invalid.
	// It wraps ErrInvalidBinaryName for errors.Is() compatibility.
	InvalidBinaryNameError struct {
		Value  BinaryName
		Reason string
	}

	// CheckName identifies a custom check (used for error reporting).
	// Must be non-empty and not whitespace-only.
	CheckName string

	// InvalidCheckNameError is returned when a CheckName value is invalid.
	// DDD Value Type error struct — wraps ErrInvalidCheckName for errors.Is().
	InvalidCheckNameError struct {
		Value CheckName
	}

	// ScriptContent holds script source code for inline command content and dependency checks.
	// The zero value ("") is valid. Non-zero values must not be whitespace-only.
	//
	//goplint:cue-fed-path
	ScriptContent string

	// InvalidScriptContentError is returned when a ScriptContent value is invalid.
	// DDD Value Type error struct — wraps ErrInvalidScriptContent for errors.Is().
	InvalidScriptContentError struct {
		Value ScriptContent
	}

	// InvalidToolDependencyError is returned when a ToolDependency has invalid fields.
	InvalidToolDependencyError struct{ FieldErrors []error }
	// InvalidCommandDependencyError is returned when a CommandDependency has invalid fields.
	InvalidCommandDependencyError struct{ FieldErrors []error }
	// InvalidCommandDependencyRefError is returned when a command dependency reference is invalid.
	InvalidCommandDependencyRefError struct {
		Value  CommandDependencyRef
		Reason string
	}
	// InvalidCommandDependencySourceIDError is returned when a command dependency source ID is invalid.
	InvalidCommandDependencySourceIDError struct {
		Value  CommandDependencySourceID
		Reason string
	}
	// InvalidCapabilityDependencyError is returned when a CapabilityDependency has invalid fields.
	InvalidCapabilityDependencyError struct{ FieldErrors []error }
	// InvalidEnvVarCheckError is returned when an EnvVarCheck has invalid fields.
	InvalidEnvVarCheckError struct{ FieldErrors []error }
	// InvalidEnvVarDependencyError is returned when an EnvVarDependency has invalid fields.
	InvalidEnvVarDependencyError struct{ FieldErrors []error }
	// InvalidFilepathDependencyError is returned when a FilepathDependency has invalid fields.
	InvalidFilepathDependencyError struct{ FieldErrors []error }
	// InvalidCustomCheckError is returned when a CustomCheck has invalid fields.
	InvalidCustomCheckError struct{ FieldErrors []error }
	// InvalidCustomCheckDependencyError is returned when a CustomCheckDependency has invalid fields.
	InvalidCustomCheckDependencyError struct{ FieldErrors []error }
	// InvalidDependsOnError is returned when a DependsOn has invalid fields.
	InvalidDependsOnError struct{ FieldErrors []error }

	//goplint:validate-all
	//
	// ToolDependency represents a tool/binary that must be available in PATH
	ToolDependency struct {
		// Alternatives is a list of binary names where any match satisfies the dependency
		// If any of the provided tools is found in PATH, the validation succeeds (early return).
		// This allows specifying multiple possible tools (e.g., ["podman", "docker"]).
		Alternatives []BinaryName `json:"alternatives"`
	}

	//goplint:validate-all
	//
	// CustomCheckScript selects either inline custom-check script content or a module-contained file reference.
	CustomCheckScript struct {
		// Content contains inline custom-check script text.
		Content ScriptContent `json:"content,omitempty"`
		// File references a custom-check script file resolved from the source module.
		File *ScriptFilePath `json:"file,omitempty"`
		// Interpreter specifies how to execute the resolved custom-check script content.
		Interpreter InterpreterSpec `json:"interpreter,omitempty"`
	}

	// InvalidCustomCheckScriptError is returned when a CustomCheckScript has invalid fields.
	// It wraps ErrInvalidCustomCheckScript for errors.Is() compatibility.
	InvalidCustomCheckScriptError struct {
		FieldErrors []error
	}

	//goplint:validate-all
	//
	// CustomCheck represents a custom validation script to verify system requirements
	CustomCheck struct {
		// Name is an identifier for this check (used for error reporting)
		Name CheckName `json:"name"`
		// Script selects inline shell content or a module-contained script file reference.
		Script CustomCheckScript `json:"script"`
		// ExpectedCode is the expected exit code from Script (optional, default: 0)
		ExpectedCode *types.ExitCode `json:"expected_code,omitempty"`
		// ExpectedOutput is a regex pattern to match against Script output (optional)
		ExpectedOutput RegexPattern `json:"expected_output,omitempty"`
	}

	//goplint:validate-all
	//
	// CustomCheckDependency represents a custom check dependency that can be either:
	// - A single CustomCheck (direct check with name, script, etc.)
	// - An alternatives list of CustomChecks (OR semantics with early return)
	//nolint:recvcheck // DDD Validate() (value) + existing methods (pointer)
	CustomCheckDependency struct {
		// Direct check fields (used when this is a single check)
		// Name is an identifier for this check (used for error reporting)
		Name CheckName `json:"name,omitempty"`
		// Script selects inline shell content or a module-contained script file reference.
		Script CustomCheckScript `json:"script,omitzero"`
		// ExpectedCode is the expected exit code from Script (optional, default: 0)
		ExpectedCode *types.ExitCode `json:"expected_code,omitempty"`
		// ExpectedOutput is a regex pattern to match against Script output (optional)
		ExpectedOutput RegexPattern `json:"expected_output,omitempty"`

		// Alternatives is a list of custom checks where any passing check satisfies the dependency
		// If any of the provided checks passes, the validation succeeds (early return).
		// When Alternatives is set, the direct check fields above are ignored.
		Alternatives []CustomCheck `json:"alternatives,omitempty"`
	}

	//goplint:validate-all
	//
	// CommandDependency represents another invowk command that must be discoverable.
	CommandDependency struct {
		// Alternatives is a list of command references where any match satisfies the dependency.
		// If any of the provided commands is discoverable, the dependency is satisfied (early return).
		// Bare refs resolve only in the declaring command's source. Source-qualified refs
		// use "@source command" syntax, e.g., "@tools lint".
		Alternatives []CommandDependencyRef `json:"alternatives"`
	}

	// CommandDependencyRef is a depends_on.cmds reference.
	// Bare refs identify commands in the declaring command's source. Qualified refs
	// use "@source command" syntax to identify a command in an explicit command source.
	CommandDependencyRef string

	// CommandDependencySourceID identifies a command source in a qualified
	// depends_on.cmds reference.
	CommandDependencySourceID string

	//goplint:ignore -- parsed DTO returned only after CommandDependencyRef.Parse validates source and command parts.
	// CommandDependencyRefParts contains the parsed form of a command dependency reference.
	CommandDependencyRefParts struct {
		SourceID  CommandDependencySourceID
		Command   CommandName //goplint:ignore -- every parsed ref has a command; zero never represents optionality.
		Qualified bool
	}

	//goplint:validate-all
	//
	// CapabilityDependency represents a system capability that must be available
	CapabilityDependency struct {
		// Alternatives is a list of capability identifiers where any match satisfies the dependency
		// If any of the provided capabilities is available, the validation succeeds (early return).
		// Available capabilities: "local-area-network", "internet", "containers", "tty"
		Alternatives []CapabilityName `json:"alternatives"`
	}

	//goplint:validate-all
	//
	// EnvVarCheck represents a single environment variable check
	EnvVarCheck struct {
		// Name is the environment variable name to check (required, non-empty)
		// The check verifies that this env var exists in the user's environment
		Name EnvVarName `json:"name"`
		// Validation is a regex pattern to validate the env var value (optional)
		// If specified, the env var must exist AND its value must match this pattern
		Validation RegexPattern `json:"validation,omitempty"`
	}

	//goplint:validate-all
	//
	// EnvVarDependency represents an environment variable dependency with alternatives
	EnvVarDependency struct {
		// Alternatives is a list of env var checks where any match satisfies the dependency
		// If any of the provided env vars exists (and passes validation if specified), the dependency is satisfied
		// This allows specifying multiple possible env vars (e.g., ["AWS_ACCESS_KEY_ID", "AWS_PROFILE"])
		Alternatives []EnvVarCheck `json:"alternatives"`
	}

	//goplint:validate-all
	//
	// FilepathDependency represents a file or directory that must exist
	FilepathDependency struct {
		// Alternatives is a list of file or directory paths where any match satisfies the dependency
		// If any of the provided paths exists and satisfies the permission requirements,
		// the validation succeeds (early return). This allows specifying multiple
		// possible locations for a file (e.g., different paths on different systems).
		Alternatives []FilesystemPath `json:"alternatives"`
		// Readable checks if the path is readable
		Readable bool `json:"readable,omitempty"`
		// Writable checks if the path is writable
		Writable bool `json:"writable,omitempty"`
		// Executable checks if the path is executable
		Executable bool `json:"executable,omitempty"`
	}

	//goplint:validate-all
	//
	// DependsOn defines the dependencies for a command
	//nolint:recvcheck // DDD Validate() (value) + existing methods (pointer)
	DependsOn struct {
		// Tools lists binaries that must be available in PATH before running
		// Uses OR semantics: if any alternative in the list is found, the dependency is satisfied
		Tools []ToolDependency `json:"tools,omitempty"`
		// Commands lists invowk commands that must be discoverable for this command to run (invowkfile field: 'cmds')
		// Uses OR semantics: if any alternative in the list is discoverable, the dependency is satisfied
		Commands []CommandDependency `json:"cmds,omitempty"`
		// Filepaths lists files or directories that must exist before running
		// Uses OR semantics: if any alternative path exists, the dependency is satisfied
		Filepaths []FilepathDependency `json:"filepaths,omitempty"`
		// Capabilities lists system capabilities that must be available before running
		// Uses OR semantics: if any alternative capability is available, the dependency is satisfied
		Capabilities []CapabilityDependency `json:"capabilities,omitempty"`
		// CustomChecks lists custom validation scripts to verify system requirements
		// Each entry can be a single check or an alternatives list (OR semantics)
		CustomChecks []CustomCheckDependency `json:"custom_checks,omitempty"`
		// EnvVars lists environment variables that must exist before running
		// Uses OR semantics: if any alternative env var exists (and passes validation), the dependency is satisfied
		// IMPORTANT: Validated against the user's environment BEFORE invowk sets command-level env vars
		EnvVars []EnvVarDependency `json:"env_vars,omitempty"`
	}
)

// IsEmpty returns true if this DependsOn has no dependencies of any type.
func (d *DependsOn) IsEmpty() bool {
	return len(d.Tools) == 0 && len(d.Commands) == 0 && len(d.Filepaths) == 0 &&
		len(d.Capabilities) == 0 && len(d.CustomChecks) == 0 && len(d.EnvVars) == 0
}

// IsAlternatives returns true if this dependency uses the alternatives format
func (c *CustomCheckDependency) IsAlternatives() bool {
	return len(c.Alternatives) > 0
}

// IsContent returns true when this custom-check script contains inline script text.
func (s CustomCheckScript) IsContent() bool {
	return s.Content != ""
}

// IsFile returns true when this custom-check script references a module-contained file.
func (s CustomCheckScript) IsFile() bool {
	return s.File != nil
}

// GetScriptFilePathWithModule returns the custom-check script file path resolved from the module root.
// It returns an empty path for inline content or non-module contexts.
func (s CustomCheckScript) GetScriptFilePathWithModule(modulePath FilesystemPath) FilesystemPath {
	if !s.IsFile() || modulePath == "" {
		return ""
	}

	return s.File.ResolveFromModule(modulePath)
}

// ResolveWithFSAndModule resolves custom-check script content using the source module boundary.
func (s CustomCheckScript) ResolveWithFSAndModule(modulePath FilesystemPath, readFile func(path string) ([]byte, error)) (ScriptContent, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	if s.IsFile() {
		if modulePath == "" {
			return "", ErrScriptFileRequiresModule
		}
		scriptPath := s.GetScriptFilePathWithModule(modulePath)
		if err := validateScriptPathContainment(scriptPath, modulePath); err != nil {
			return "", err
		}
		if readFile == nil {
			return "", ErrScriptReaderRequired
		}
		content, err := readFile(string(scriptPath))
		if err != nil {
			return "", scriptFileReadError(*s.File, scriptPath, err)
		}
		return validateResolvedScriptContent("custom check script file content", ScriptContent(content)) //goplint:ignore -- validated before use.
	}
	return validateResolvedScriptContent("custom check inline script content", s.Content)
}

// ResolveInterpreterFromScript resolves the interpreter for this custom-check
// script using the provided resolved script content.
//
//goplint:ignore -- interpreter resolution consumes already-validated custom-check script bytes.
func (s CustomCheckScript) ResolveInterpreterFromScript(scriptContent string) ShebangInfo {
	return ResolveInterpreter(s.Interpreter, scriptContent)
}

// GetChecks returns the list of CustomCheck to validate.
// If Alternatives is set, returns those; otherwise returns a single-element list with the direct check.
func (c *CustomCheckDependency) GetChecks() []CustomCheck {
	if c.IsAlternatives() {
		return c.Alternatives
	}
	// Return as a single-element list
	return []CustomCheck{{
		Name:           c.Name,
		Script:         c.Script,
		ExpectedCode:   c.ExpectedCode,
		ExpectedOutput: c.ExpectedOutput,
	}}
}

// MergeDependsOnAll merges root-level, command-level, and implementation-level dependencies.
// Dependencies are combined in order: root -> command -> implementation.
// Returns a new DependsOn struct with combined dependencies.
func MergeDependsOnAll(rootDeps, cmdDeps, implDeps *DependsOn) *DependsOn {
	merged := &DependsOn{}

	// Append in declaration order: root → command → implementation.
	merged.appendFrom(rootDeps)
	merged.appendFrom(cmdDeps)
	merged.appendFrom(implDeps)

	// Return nil if no dependencies after merging
	if merged.IsEmpty() {
		return nil
	}

	return merged
}

// Error implements the error interface for InvalidBinaryNameError.
func (e *InvalidBinaryNameError) Error() string {
	return fmt.Sprintf("invalid binary name %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidBinaryName for errors.Is() compatibility.
func (e *InvalidBinaryNameError) Unwrap() error { return ErrInvalidBinaryName }

// Validate returns nil if the BinaryName is valid, or a validation error if not.
// A valid BinaryName must be non-empty, not whitespace-only, fit MaxNameLength,
// start with an alphanumeric character, and contain only alphanumeric
// characters plus '.', '_', '+', or '-'.
//
//goplint:nonzero
func (b BinaryName) Validate() error {
	s := string(b)
	if strings.TrimSpace(s) == "" {
		return &InvalidBinaryNameError{Value: b, Reason: "must not be empty or whitespace-only"}
	}
	if utf8.RuneCountInString(s) > MaxNameLength {
		return &InvalidBinaryNameError{Value: b, Reason: fmt.Sprintf("exceeds maximum length of %d runes", MaxNameLength)}
	}
	if strings.ContainsAny(s, "/\\") {
		return &InvalidBinaryNameError{Value: b, Reason: "must not contain path separators"}
	}
	if !toolNameRegex.MatchString(s) {
		return &InvalidBinaryNameError{Value: b, Reason: "must start with an alphanumeric character and contain only alphanumeric characters, '.', '_', '+', or '-'"}
	}
	return nil
}

// String returns the string representation of the BinaryName.
func (b BinaryName) String() string { return string(b) }

// String returns the string representation of the CheckName.
func (c CheckName) String() string { return string(c) }

// Validate returns nil if the CheckName is valid, or a validation error if not.
// A valid CheckName is non-empty and not whitespace-only.
//
//goplint:nonzero
func (c CheckName) Validate() error {
	if strings.TrimSpace(string(c)) == "" {
		return &InvalidCheckNameError{Value: c}
	}
	return nil
}

// Error implements the error interface for InvalidCheckNameError.
func (e *InvalidCheckNameError) Error() string {
	return fmt.Sprintf("invalid check name %q: must be non-empty and not whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidCheckName for errors.Is() compatibility.
func (e *InvalidCheckNameError) Unwrap() error { return ErrInvalidCheckName }

// String returns the string representation of the ScriptContent.
func (s ScriptContent) String() string { return string(s) }

// Validate returns nil if the ScriptContent is valid, or a validation error if not.
// The zero value ("") is valid. Non-zero values must not be whitespace-only.
func (s ScriptContent) Validate() error {
	if s == "" {
		return nil
	}
	if strings.TrimSpace(string(s)) == "" {
		return &InvalidScriptContentError{Value: s}
	}
	return nil
}

// Error implements the error interface for InvalidScriptContentError.
func (e *InvalidScriptContentError) Error() string {
	return fmt.Sprintf("invalid script content: non-empty value must not be whitespace-only (got %q)", e.Value)
}

// Unwrap returns ErrInvalidScriptContent for errors.Is() compatibility.
func (e *InvalidScriptContentError) Unwrap() error { return ErrInvalidScriptContent }

// Validate returns nil if the ToolDependency has valid fields.
func (t ToolDependency) Validate() error {
	var errs []error
	if len(t.Alternatives) == 0 {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	for _, b := range t.Alternatives {
		if err := b.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidToolDependencyError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the CommandDependency has valid fields.
func (c CommandDependency) Validate() error {
	var errs []error
	if len(c.Alternatives) == 0 {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	for _, ref := range c.Alternatives {
		if err := ref.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidCommandDependencyError{FieldErrors: errs}
	}
	return nil
}

// Parse returns the structured form of a command dependency reference.
func (r CommandDependencyRef) Parse() (CommandDependencyRefParts, error) {
	raw := string(r)
	if strings.TrimSpace(raw) == "" {
		return CommandDependencyRefParts{}, &InvalidCommandDependencyRefError{
			Value:  r,
			Reason: invalidReasonMustNotBeEmpty,
		}
	}
	if strings.HasPrefix(raw, "@") {
		return parseQualifiedCommandDependencyRef(r)
	}
	command := CommandName(raw)
	if err := command.Validate(); err != nil {
		return CommandDependencyRefParts{}, &InvalidCommandDependencyRefError{
			Value:  r,
			Reason: "expected bare command name or @source command reference",
		}
	}
	return CommandDependencyRefParts{Command: command}, nil
}

// Validate returns nil if the command dependency reference is valid.
func (r CommandDependencyRef) Validate() error {
	_, err := r.Parse()
	return err
}

// String returns the string representation of the command dependency reference.
func (r CommandDependencyRef) String() string { return string(r) }

// Validate returns nil if the parsed command dependency reference parts are internally consistent.
func (p CommandDependencyRefParts) Validate() error {
	if err := p.Command.Validate(); err != nil {
		return &InvalidCommandDependencyRefError{
			Value:  p.validationRef(),
			Reason: "invalid command name",
		}
	}
	if !p.Qualified {
		if p.SourceID != "" {
			return &InvalidCommandDependencyRefError{
				Value:  p.validationRef(),
				Reason: "bare references must not include a source",
			}
		}
		return nil
	}
	if err := p.SourceID.Validate(); err != nil {
		return &InvalidCommandDependencyRefError{
			Value:  p.validationRef(),
			Reason: err.Error(),
		}
	}
	return nil
}

// String renders the parsed command dependency reference in user-facing syntax.
func (p CommandDependencyRefParts) String() string {
	if p.Qualified {
		return "@" + p.SourceID.String() + " " + p.Command.String()
	}
	return p.Command.String()
}

func (p CommandDependencyRefParts) validationRef() CommandDependencyRef {
	return CommandDependencyRef(p.String()) //goplint:ignore -- rendered parsed parts were already validated field-by-field.
}

// Validate returns nil if the command dependency source ID is valid.
func (s CommandDependencySourceID) Validate() error {
	value := string(s)
	if strings.TrimSpace(value) == "" {
		return &InvalidCommandDependencySourceIDError{Reason: invalidReasonMustNotBeEmpty}
	}
	if len(value) > MaxNameLength {
		return &InvalidCommandDependencySourceIDError{Value: s, Reason: fmt.Sprintf("exceeds maximum length of %d chars", MaxNameLength)}
	}
	if !cmdDependencySourceIDRegex.MatchString(value) {
		return &InvalidCommandDependencySourceIDError{
			Value:  s,
			Reason: "must start with a letter and contain only letters, digits, dots, underscores, or hyphens",
		}
	}
	return nil
}

// String returns the string representation of the command dependency source ID.
func (s CommandDependencySourceID) String() string { return string(s) }

// Error implements the error interface for InvalidCommandDependencyRefError.
func (e *InvalidCommandDependencyRefError) Error() string {
	reason := e.Reason
	if reason == "" {
		reason = "expected bare command name or @source command reference"
	}
	return fmt.Sprintf("invalid command dependency reference %q: %s", e.Value, reason)
}

// Unwrap returns ErrInvalidCommandDependencyRef for errors.Is compatibility.
func (e *InvalidCommandDependencyRefError) Unwrap() error { return ErrInvalidCommandDependencyRef }

// Error implements the error interface for InvalidCommandDependencySourceIDError.
func (e *InvalidCommandDependencySourceIDError) Error() string {
	reason := e.Reason
	if reason == "" {
		reason = "must start with a letter and contain only letters, digits, dots, underscores, or hyphens"
	}
	return fmt.Sprintf("invalid command dependency source id %q: %s", e.Value, reason)
}

// Unwrap returns ErrInvalidCommandDependencySourceID for errors.Is compatibility.
func (e *InvalidCommandDependencySourceIDError) Unwrap() error {
	return ErrInvalidCommandDependencySourceID
}

func parseQualifiedCommandDependencyRef(ref CommandDependencyRef) (CommandDependencyRefParts, error) {
	sourceAndCommand := strings.TrimPrefix(ref.String(), "@")
	source, commandPart, ok := strings.Cut(sourceAndCommand, " ")
	if !ok {
		return CommandDependencyRefParts{}, &InvalidCommandDependencyRefError{
			Value:  ref,
			Reason: "qualified references must use @source command",
		}
	}
	sourceID := CommandDependencySourceID(source)
	if err := sourceID.Validate(); err != nil {
		return CommandDependencyRefParts{}, &InvalidCommandDependencyRefError{
			Value:  ref,
			Reason: err.Error(),
		}
	}
	command := CommandName(commandPart) //goplint:ignore -- command part is validated immediately below before returning parsed ref.
	if command == "" {
		return CommandDependencyRefParts{}, &InvalidCommandDependencyRefError{
			Value:  ref,
			Reason: "qualified references must include a command name after the source",
		}
	}
	if err := command.Validate(); err != nil {
		return CommandDependencyRefParts{}, &InvalidCommandDependencyRefError{
			Value:  ref,
			Reason: "invalid command name after source",
		}
	}
	parts := CommandDependencyRefParts{
		SourceID:  sourceID,
		Command:   command,
		Qualified: true,
	}
	return parts, nil
}

// Validate returns nil if the CapabilityDependency has valid fields.
func (c CapabilityDependency) Validate() error {
	var errs []error
	if len(c.Alternatives) == 0 {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	for _, n := range c.Alternatives {
		if err := n.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidCapabilityDependencyError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the EnvVarCheck has valid fields.
// Delegates to Name.Validate() (nonzero) and Validation.Validate() (zero-valid).
func (e EnvVarCheck) Validate() error {
	var errs []error
	if err := e.Name.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := e.Validation.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidEnvVarCheckError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the EnvVarDependency has valid fields.
func (e EnvVarDependency) Validate() error {
	var errs []error
	if len(e.Alternatives) == 0 {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	for _, c := range e.Alternatives {
		if err := c.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidEnvVarDependencyError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the FilepathDependency has valid fields.
func (f FilepathDependency) Validate() error {
	var errs []error
	if len(f.Alternatives) == 0 {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	for _, p := range f.Alternatives {
		if err := p.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidFilepathDependencyError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil when the custom-check script selects exactly one valid source.
func (s CustomCheckScript) Validate() error {
	hasContent := s.Content != ""
	hasFile := s.File != nil
	var errs []error
	switch {
	case hasContent && hasFile:
		errs = append(errs, ErrMixedCustomCheckScriptSource)
	case !hasContent && !hasFile:
		errs = append(errs, ErrMissingCustomCheckScriptSource)
	}
	if hasContent {
		if err := s.Content.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if hasFile {
		if err := s.File.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	appendOptionalValidation(&errs, s.Interpreter, s.Interpreter != "")
	if len(errs) > 0 {
		return &InvalidCustomCheckScriptError{FieldErrors: errs}
	}
	return nil
}

func (s CustomCheckScript) hasSource() bool {
	return s.IsContent() || s.IsFile()
}

// Validate returns nil if the CustomCheck has valid fields.
// Delegates to Name.Validate() (nonzero), Script.Validate() (required),
// ExpectedCode.Validate() (when non-nil), and ExpectedOutput.Validate() (zero-valid).
func (c CustomCheck) Validate() error {
	var errs []error
	if err := c.Name.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.Script.Validate(); err != nil {
		errs = append(errs, err)
	}
	if c.ExpectedCode != nil {
		if err := c.ExpectedCode.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := c.ExpectedOutput.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidCustomCheckError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the CustomCheckDependency has valid fields.
// When Alternatives is set, delegates to each CustomCheck.Validate().
// Otherwise, validates the direct check fields.
func (c CustomCheckDependency) Validate() error {
	var errs []error

	if len(c.Alternatives) > 0 {
		if c.hasDirectFields() {
			errs = append(errs, ErrMixedCustomCheckDependency)
		}
		for i := range c.Alternatives {
			appendFieldError(&errs, c.Alternatives[i].Validate())
		}
		if len(errs) > 0 {
			return &InvalidCustomCheckDependencyError{FieldErrors: errs}
		}
		return nil
	}

	if c.Name == "" && !c.Script.hasSource() {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	appendFieldError(&errs, c.Name.Validate())
	appendFieldError(&errs, c.Script.Validate())
	if c.ExpectedCode != nil {
		appendFieldError(&errs, c.ExpectedCode.Validate())
	}
	if c.ExpectedOutput != "" {
		appendFieldError(&errs, c.ExpectedOutput.Validate())
	}
	if len(errs) > 0 {
		return &InvalidCustomCheckDependencyError{FieldErrors: errs}
	}
	return nil
}

func (c CustomCheckDependency) hasDirectFields() bool {
	return c.Name != "" || c.Script.hasSource() || c.ExpectedCode != nil || c.ExpectedOutput != ""
}

// Validate returns nil if the DependsOn has valid fields.
// Delegates to Validate() on all dependency slices.
func (d DependsOn) Validate() error {
	var errs []error
	appendEachValidation(&errs, d.Tools)
	appendEachValidation(&errs, d.Commands)
	appendEachValidation(&errs, d.Filepaths)
	appendEachValidation(&errs, d.Capabilities)
	appendEachValidation(&errs, d.CustomChecks)
	appendEachValidation(&errs, d.EnvVars)
	if len(errs) > 0 {
		return &InvalidDependsOnError{FieldErrors: errs}
	}
	return nil
}

// Error/Unwrap implementations for dependency error types.

func (e *InvalidToolDependencyError) Error() string {
	return types.FormatFieldErrors("tool dependency", e.FieldErrors)
}

func (e *InvalidToolDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidToolDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidCommandDependencyError) Error() string {
	return types.FormatFieldErrors("command dependency", e.FieldErrors)
}

func (e *InvalidCommandDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidCommandDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidCapabilityDependencyError) Error() string {
	return types.FormatFieldErrors("capability dependency", e.FieldErrors)
}

func (e *InvalidCapabilityDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidCapabilityDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidEnvVarCheckError) Error() string {
	return types.FormatFieldErrors("env var check", e.FieldErrors)
}
func (e *InvalidEnvVarCheckError) Unwrap() error { return ErrInvalidEnvVarCheck }

func (e *InvalidEnvVarDependencyError) Error() string {
	return types.FormatFieldErrors("env var dependency", e.FieldErrors)
}

func (e *InvalidEnvVarDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidEnvVarDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidFilepathDependencyError) Error() string {
	return types.FormatFieldErrors("filepath dependency", e.FieldErrors)
}

func (e *InvalidFilepathDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidFilepathDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidCustomCheckError) Error() string {
	return types.FormatFieldErrors("custom check", e.FieldErrors)
}

func (e *InvalidCustomCheckError) Unwrap() error {
	return errors.Join(ErrInvalidCustomCheck, errors.Join(e.FieldErrors...))
}

func (e *InvalidCustomCheckScriptError) Error() string {
	return types.FormatFieldErrors("custom check script", e.FieldErrors)
}

func (e *InvalidCustomCheckScriptError) Unwrap() error {
	return errors.Join(ErrInvalidCustomCheckScript, errors.Join(e.FieldErrors...))
}

func (e *InvalidCustomCheckDependencyError) Error() string {
	return types.FormatFieldErrors("custom check dependency", e.FieldErrors)
}

func (e *InvalidCustomCheckDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidCustomCheckDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidDependsOnError) Error() string {
	return types.FormatFieldErrors("depends_on", e.FieldErrors)
}

func (e *InvalidDependsOnError) Unwrap() error {
	return errors.Join(ErrInvalidDependsOn, errors.Join(e.FieldErrors...))
}

// appendFrom appends all dependency slices from src into d. Nil src is a no-op.
func (d *DependsOn) appendFrom(src *DependsOn) {
	if src == nil {
		return
	}
	d.Tools = append(d.Tools, src.Tools...)
	d.Commands = append(d.Commands, src.Commands...)
	d.Filepaths = append(d.Filepaths, src.Filepaths...)
	d.Capabilities = append(d.Capabilities, src.Capabilities...)
	d.CustomChecks = append(d.CustomChecks, src.CustomChecks...)
	d.EnvVars = append(d.EnvVars, src.EnvVars...)
}
