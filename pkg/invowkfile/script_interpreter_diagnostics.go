// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"slices"
	"strings"
)

const (
	// ScriptInterpreterSourceInline identifies inline script.content sources.
	ScriptInterpreterSourceInline ScriptInterpreterSourceKind = "inline"
	// ScriptInterpreterSourceFile identifies script.file sources.
	ScriptInterpreterSourceFile ScriptInterpreterSourceKind = "file"

	// ScriptInterpreterProvenanceDefaultShell means runtime default shell behavior is used.
	ScriptInterpreterProvenanceDefaultShell ScriptInterpreterProvenance = "default_shell"
	// ScriptInterpreterProvenanceExplicit means script.interpreter selected the interpreter.
	ScriptInterpreterProvenanceExplicit ScriptInterpreterProvenance = "explicit"
	// ScriptInterpreterProvenanceShebang means a shebang selected the interpreter.
	ScriptInterpreterProvenanceShebang ScriptInterpreterProvenance = "shebang"

	// ScriptInterpreterDiagnosticShebangOverride reports an explicit interpreter overriding a shebang.
	ScriptInterpreterDiagnosticShebangOverride ScriptInterpreterDiagnosticCode = "script_interpreter_shebang_override"
)

type (
	//goplint:constant-only
	//
	// ScriptInterpreterSourceKind identifies the script source variant being analyzed.
	ScriptInterpreterSourceKind string

	//goplint:ignore -- diagnostic labels are display text synthesized from validated command metadata.
	//
	// ScriptInterpreterSourceLabel is human-readable script source context.
	ScriptInterpreterSourceLabel string

	//goplint:constant-only
	//
	// ScriptInterpreterProvenance identifies why an effective interpreter was selected.
	ScriptInterpreterProvenance string

	//goplint:constant-only
	//
	// ScriptInterpreterDiagnosticCode is a stable interpreter diagnostic identifier.
	ScriptInterpreterDiagnosticCode string

	//goplint:ignore -- diagnostic messages are human-readable rendering text.
	//
	// ScriptInterpreterDiagnosticMessage is a human-readable interpreter diagnostic.
	ScriptInterpreterDiagnosticMessage string

	//goplint:ignore -- diagnostic source DTO is synthesized internally from already validated script sources.
	//
	// ScriptInterpreterSource identifies the script source used for interpreter diagnostics.
	ScriptInterpreterSource struct {
		kind  ScriptInterpreterSourceKind
		path  *FilesystemPath
		label ScriptInterpreterSourceLabel
	}

	// ScriptInterpreterDiagnostic describes a non-fatal interpreter authoring issue.
	ScriptInterpreterDiagnostic struct {
		code     ScriptInterpreterDiagnosticCode
		source   ScriptInterpreterSource
		explicit ShebangInfo
		shebang  ShebangInfo
	}

	// ScriptInterpreterAnalysis describes effective interpreter selection for resolved script bytes.
	ScriptInterpreterAnalysis struct {
		source      ScriptInterpreterSource
		effective   ShebangInfo
		shebang     ShebangInfo
		provenance  ScriptInterpreterProvenance
		diagnostics []ScriptInterpreterDiagnostic
	}
)

// newInlineScriptInterpreterSource creates source metadata for script.content.
func newInlineScriptInterpreterSource(label ScriptInterpreterSourceLabel) ScriptInterpreterSource {
	return ScriptInterpreterSource{kind: ScriptInterpreterSourceInline, label: label}
}

// newFileScriptInterpreterSource creates source metadata for script.file.
func newFileScriptInterpreterSource(path ScriptFilePath, label ScriptInterpreterSourceLabel) ScriptInterpreterSource {
	filePath := FilesystemPath(path.String()) //goplint:ignore -- diagnostic source preserves validated script file reference text.
	return ScriptInterpreterSource{kind: ScriptInterpreterSourceFile, path: &filePath, label: label}
}

// AnalyzeScriptInterpreter returns interpreter provenance and advisory diagnostics.
func AnalyzeScriptInterpreter(source ScriptInterpreterSource, interpreter InterpreterSpec, scriptContent ScriptContent, runtime RuntimeMode) ScriptInterpreterAnalysis {
	scriptText := scriptContent.String()
	shebang := ParseShebang(scriptText)
	effective := ResolveInterpreter(interpreter, scriptText)
	provenance := scriptInterpreterProvenance(interpreter, shebang)
	analysis := ScriptInterpreterAnalysis{
		source:     source,
		effective:  effective,
		shebang:    shebang,
		provenance: provenance,
	}
	if !shebang.Found || !effective.Found {
		return analysis
	}
	if interpreterSelectionsEquivalent(effective, shebang, runtime) {
		return analysis
	}
	analysis.diagnostics = append(analysis.diagnostics, ScriptInterpreterDiagnostic{
		code:     ScriptInterpreterDiagnosticShebangOverride,
		source:   source,
		explicit: effective,
		shebang:  shebang,
	})
	return analysis
}

// AnalyzeInterpreter returns interpreter analysis for an implementation script.
func (s ImplementationScript) AnalyzeInterpreter(scriptContent ScriptContent, runtime RuntimeMode) ScriptInterpreterAnalysis {
	return AnalyzeScriptInterpreter(s.InterpreterSource(), s.Interpreter, scriptContent, runtime)
}

// AnalyzeInterpreter returns interpreter analysis for a custom-check script.
func (s CustomCheckScript) AnalyzeInterpreter(scriptContent ScriptContent, runtime RuntimeMode, label ScriptInterpreterSourceLabel) ScriptInterpreterAnalysis {
	return AnalyzeScriptInterpreter(s.InterpreterSource(label), s.Interpreter, scriptContent, runtime)
}

// InterpreterSource returns diagnostic source metadata for an implementation script.
func (s ImplementationScript) InterpreterSource() ScriptInterpreterSource {
	if s.File != nil {
		return newFileScriptInterpreterSource(*s.File, "")
	}
	return newInlineScriptInterpreterSource("inline script")
}

// InterpreterSource returns diagnostic source metadata for a custom-check script.
func (s CustomCheckScript) InterpreterSource(label ScriptInterpreterSourceLabel) ScriptInterpreterSource {
	if s.File != nil {
		return newFileScriptInterpreterSource(*s.File, label)
	}
	return newInlineScriptInterpreterSource(label)
}

// Kind returns the source variant kind.
func (s ScriptInterpreterSource) Kind() ScriptInterpreterSourceKind { return s.kind }

// Path returns the authored script.file path when the source is file-backed.
func (s ScriptInterpreterSource) Path() FilesystemPath {
	if s.path == nil {
		return ""
	}
	return *s.path
}

// Label returns human-readable source context.
func (s ScriptInterpreterSource) Label() ScriptInterpreterSourceLabel { return s.label }

// Validate returns nil when the source metadata is structurally valid.
func (s ScriptInterpreterSource) Validate() error {
	if s.kind != "" {
		if err := s.kind.Validate(); err != nil {
			return err
		}
	}
	if s.path != nil {
		if err := s.path.Validate(); err != nil {
			return fmt.Errorf("script interpreter source path: %w", err)
		}
	}
	return s.label.Validate()
}

// DisplayName returns user-facing source text for diagnostics.
//
//goplint:ignore -- display helper returns UI text assembled from typed source metadata.
func (s ScriptInterpreterSource) DisplayName() string {
	if s.path != nil && strings.TrimSpace(s.path.String()) != "" {
		return s.path.String()
	}
	if strings.TrimSpace(string(s.label)) != "" {
		return string(s.label)
	}
	if s.kind == ScriptInterpreterSourceFile {
		return "script file"
	}
	return "inline script"
}

// String returns the source kind string.
func (k ScriptInterpreterSourceKind) String() string { return string(k) }

// Validate returns nil when the source kind is recognized.
func (k ScriptInterpreterSourceKind) Validate() error {
	switch k {
	case ScriptInterpreterSourceInline, ScriptInterpreterSourceFile:
		return nil
	default:
		return fmt.Errorf("invalid script interpreter source kind %q", k)
	}
}

// String returns the source label string.
func (l ScriptInterpreterSourceLabel) String() string { return string(l) }

// Validate returns nil because source labels are optional display context.
func (l ScriptInterpreterSourceLabel) Validate() error { return nil }

// String returns the provenance string.
func (p ScriptInterpreterProvenance) String() string { return string(p) }

// Validate returns nil when the provenance is recognized.
func (p ScriptInterpreterProvenance) Validate() error {
	switch p {
	case ScriptInterpreterProvenanceDefaultShell, ScriptInterpreterProvenanceExplicit, ScriptInterpreterProvenanceShebang:
		return nil
	default:
		return fmt.Errorf("invalid script interpreter provenance %q", p)
	}
}

// Code returns the diagnostic code.
func (d ScriptInterpreterDiagnostic) Code() ScriptInterpreterDiagnosticCode { return d.code }

// Source returns the diagnostic source metadata.
func (d ScriptInterpreterDiagnostic) Source() ScriptInterpreterSource { return d.source }

// Explicit returns the explicit interpreter selection that takes precedence.
func (d ScriptInterpreterDiagnostic) Explicit() ShebangInfo { return d.explicit }

// Shebang returns the shebang interpreter selection that is overridden.
func (d ScriptInterpreterDiagnostic) Shebang() ShebangInfo { return d.shebang }

// Path returns the authored script.file path when available.
func (d ScriptInterpreterDiagnostic) Path() FilesystemPath { return d.source.Path() }

// Validate returns nil when the diagnostic metadata is structurally valid.
func (d ScriptInterpreterDiagnostic) Validate() error {
	if err := d.code.Validate(); err != nil {
		return err
	}
	return d.source.Validate()
}

// Message returns a human-readable diagnostic message.
func (d ScriptInterpreterDiagnostic) Message() ScriptInterpreterDiagnosticMessage {
	return ScriptInterpreterDiagnosticMessage(fmt.Sprintf(
		"%s declares script.interpreter %q, which overrides shebang %q; script.interpreter takes precedence",
		d.source.DisplayName(),
		d.explicit.CommandString(),
		d.shebang.CommandString(),
	))
}

// String returns the diagnostic code string.
func (c ScriptInterpreterDiagnosticCode) String() string { return string(c) }

// Validate returns nil when the diagnostic code is recognized.
func (c ScriptInterpreterDiagnosticCode) Validate() error {
	switch c {
	case ScriptInterpreterDiagnosticShebangOverride:
		return nil
	default:
		return fmt.Errorf("invalid script interpreter diagnostic code %q", c)
	}
}

// String returns the diagnostic message string.
func (m ScriptInterpreterDiagnosticMessage) String() string { return string(m) }

// Validate returns nil because diagnostic messages are synthesized display text.
func (m ScriptInterpreterDiagnosticMessage) Validate() error { return nil }

// Source returns the analyzed script source.
func (a ScriptInterpreterAnalysis) Source() ScriptInterpreterSource { return a.source }

// Effective returns the interpreter selected for execution.
func (a ScriptInterpreterAnalysis) Effective() ShebangInfo { return a.effective }

// Shebang returns the shebang parsed from resolved script bytes.
func (a ScriptInterpreterAnalysis) Shebang() ShebangInfo { return a.shebang }

// Provenance returns why the effective interpreter was selected.
func (a ScriptInterpreterAnalysis) Provenance() ScriptInterpreterProvenance { return a.provenance }

// Validate returns nil when the analysis metadata is structurally valid.
func (a ScriptInterpreterAnalysis) Validate() error {
	if err := a.source.Validate(); err != nil {
		return err
	}
	if a.provenance != "" {
		if err := a.provenance.Validate(); err != nil {
			return err
		}
	}
	for i := range a.diagnostics {
		if err := a.diagnostics[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Diagnostics returns advisory interpreter diagnostics.
func (a ScriptInterpreterAnalysis) Diagnostics() []ScriptInterpreterDiagnostic {
	return slices.Clone(a.diagnostics)
}

func scriptInterpreterProvenance(interpreter InterpreterSpec, shebang ShebangInfo) ScriptInterpreterProvenance {
	if isConcreteInterpreter(interpreter) {
		return ScriptInterpreterProvenanceExplicit
	}
	if shebang.Found {
		return ScriptInterpreterProvenanceShebang
	}
	return ScriptInterpreterProvenanceDefaultShell
}

func isConcreteInterpreter(interpreter InterpreterSpec) bool {
	spec := strings.TrimSpace(interpreter.String())
	return spec != "" && spec != InterpreterAuto
}

func interpreterSelectionsEquivalent(explicit, shebang ShebangInfo, runtime RuntimeMode) bool {
	if runtime == RuntimeVirtualSh && IsShellInterpreter(explicit.Interpreter) && IsShellInterpreter(shebang.Interpreter) {
		return true
	}
	return explicit.Interpreter == shebang.Interpreter && slices.Equal(explicit.Args, shebang.Args)
}
