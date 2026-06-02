// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

type scriptInterpreterDiagnosticCase struct {
	name           string
	source         ScriptInterpreterSource
	interpreter    InterpreterSpec
	script         ScriptContent
	runtime        RuntimeMode
	wantProvenance ScriptInterpreterProvenance
	wantEffective  string
	wantShebang    string
	wantDiag       bool
}

func TestAnalyzeScriptInterpreterDiagnostics(t *testing.T) {
	t.Parallel()

	fileSource := newFileScriptInterpreterSource("scripts/build", "")
	inlineSource := newInlineScriptInterpreterSource("inline script")

	tests := []scriptInterpreterDiagnosticCase{
		{
			name:           "explicit interpreter overrides file shebang",
			source:         fileSource,
			interpreter:    "python3",
			script:         "#!/bin/sh\nprint('ok')",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceExplicit,
			wantEffective:  "python3",
			wantShebang:    "/bin/sh",
			wantDiag:       true,
		},
		{
			name:           "omitted interpreter uses shebang without warning",
			source:         inlineSource,
			script:         "#!/usr/bin/env python3\nprint('ok')",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceShebang,
			wantEffective:  "python3",
			wantShebang:    "python3",
		},
		{
			name:           "auto interpreter uses shebang without warning",
			source:         inlineSource,
			interpreter:    "auto",
			script:         "#!/bin/bash\necho ok",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceShebang,
			wantEffective:  "/bin/bash",
			wantShebang:    "/bin/bash",
		},
		{
			name:           "equivalent env shebang does not warn",
			source:         inlineSource,
			interpreter:    "python3",
			script:         "#!/usr/bin/env python3\nprint('ok')",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceExplicit,
			wantEffective:  "python3",
			wantShebang:    "python3",
		},
		{
			name:           "argument difference warns",
			source:         inlineSource,
			interpreter:    "python3",
			script:         "#!/usr/bin/env -S python3 -u\nprint('ok')",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceExplicit,
			wantEffective:  "python3",
			wantShebang:    "python3 -u",
			wantDiag:       true,
		},
		{
			name:           "virtual shell-compatible selections do not warn",
			source:         inlineSource,
			interpreter:    "bash",
			script:         "#!/bin/sh\necho ok",
			runtime:        RuntimeVirtualSh,
			wantProvenance: ScriptInterpreterProvenanceExplicit,
			wantEffective:  "bash",
			wantShebang:    "/bin/sh",
		},
		{
			name:           "default shell without shebang",
			source:         inlineSource,
			script:         "echo ok",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceDefaultShell,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertScriptInterpreterDiagnostic(t, tt)
		})
	}
}

func TestScriptInterpreterSourceMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("inline source preserves kind label and fallback display", func(t *testing.T) {
		t.Parallel()

		source := newInlineScriptInterpreterSource("custom check 'lint'")
		assertScriptInterpreterSource(t, source, ScriptInterpreterSourceInline, "", "custom check 'lint'")
		if got, want := source.DisplayName(), "custom check 'lint'"; got != want {
			t.Fatalf("DisplayName() = %q, want %q", got, want)
		}

		emptyLabel := newInlineScriptInterpreterSource("")
		assertScriptInterpreterSource(t, emptyLabel, ScriptInterpreterSourceInline, "", "")
		if got, want := emptyLabel.DisplayName(), "inline script"; got != want {
			t.Fatalf("DisplayName() = %q, want %q", got, want)
		}
	})

	t.Run("file source preserves kind path label and display precedence", func(t *testing.T) {
		t.Parallel()

		source := newFileScriptInterpreterSource("scripts/build.py", "custom check 'build'")
		assertScriptInterpreterSource(t, source, ScriptInterpreterSourceFile, "scripts/build.py", "custom check 'build'")
		if got, want := source.DisplayName(), "scripts/build.py"; got != want {
			t.Fatalf("DisplayName() = %q, want %q", got, want)
		}

		labelFallback := newFileScriptInterpreterSource(" \t", "custom check 'lint'")
		if got, want := labelFallback.DisplayName(), "custom check 'lint'"; got != want {
			t.Fatalf("DisplayName() = %q, want %q", got, want)
		}

		emptyFallback := newFileScriptInterpreterSource("", "")
		if got, want := emptyFallback.DisplayName(), "script file"; got != want {
			t.Fatalf("DisplayName() = %q, want %q", got, want)
		}
	})
}

func TestScriptInterpreterStringMutationContracts(t *testing.T) {
	t.Parallel()

	if got, want := ScriptInterpreterSourceInline.String(), "inline"; got != want {
		t.Fatalf("ScriptInterpreterSourceKind.String() = %q, want %q", got, want)
	}
	if got, want := ScriptInterpreterProvenanceExplicit.String(), "explicit"; got != want {
		t.Fatalf("ScriptInterpreterProvenance.String() = %q, want %q", got, want)
	}
	if got, want := ScriptInterpreterDiagnosticShebangOverride.String(), "script_interpreter_shebang_override"; got != want {
		t.Fatalf("ScriptInterpreterDiagnosticCode.String() = %q, want %q", got, want)
	}
}

func TestScriptInterpreterAnalysisMutationContracts(t *testing.T) {
	t.Parallel()

	source := newFileScriptInterpreterSource("scripts/build.py", "custom check 'build'")
	analysis := AnalyzeScriptInterpreter(
		source,
		"python3 -u",
		"#!/usr/bin/env -S python3 -B\nprint('ok')",
		RuntimeNative,
	)

	assertScriptInterpreterSource(t, analysis.Source(), ScriptInterpreterSourceFile, "scripts/build.py", "custom check 'build'")
	if got, want := analysis.Effective().CommandString(), "python3 -u"; got != want {
		t.Fatalf("Effective().CommandString() = %q, want %q", got, want)
	}
	if got, want := analysis.Shebang().CommandString(), "python3 -B"; got != want {
		t.Fatalf("Shebang().CommandString() = %q, want %q", got, want)
	}
	if got, want := analysis.Provenance(), ScriptInterpreterProvenanceExplicit; got != want {
		t.Fatalf("Provenance() = %q, want %q", got, want)
	}
	if err := analysis.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	diagnostics := analysis.Diagnostics()
	if len(diagnostics) != 1 {
		t.Fatalf("Diagnostics() len = %d, want 1", len(diagnostics))
	}
	diagnostic := diagnostics[0]
	if got, want := diagnostic.Code(), ScriptInterpreterDiagnosticShebangOverride; got != want {
		t.Fatalf("diagnostic Code() = %q, want %q", got, want)
	}
	assertScriptInterpreterSource(t, diagnostic.Source(), ScriptInterpreterSourceFile, "scripts/build.py", "custom check 'build'")
	if got, want := diagnostic.Path(), FilesystemPath("scripts/build.py"); got != want {
		t.Fatalf("diagnostic Path() = %q, want %q", got, want)
	}
	if got, want := diagnostic.Explicit().CommandString(), "python3 -u"; got != want {
		t.Fatalf("diagnostic Explicit() = %q, want %q", got, want)
	}
	if got, want := diagnostic.Shebang().CommandString(), "python3 -B"; got != want {
		t.Fatalf("diagnostic Shebang() = %q, want %q", got, want)
	}
	if err := diagnostic.Validate(); err != nil {
		t.Fatalf("diagnostic Validate() error = %v, want nil", err)
	}
	if got := diagnostic.Message().String(); !strings.Contains(got, "scripts/build.py declares script.interpreter \"python3 -u\"") {
		t.Fatalf("diagnostic Message() = %q, want source and explicit interpreter", got)
	}

	diagnostics[0] = ScriptInterpreterDiagnostic{}
	if got, want := analysis.Diagnostics()[0].Code(), ScriptInterpreterDiagnosticShebangOverride; got != want {
		t.Fatalf("Diagnostics() returned mutable slice, code = %q, want %q", got, want)
	}
}

func TestAnalyzeScriptInterpreterGuardMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		interpreter   InterpreterSpec
		script        ScriptContent
		wantEffective string
		wantShebang   string
	}{
		{
			name:          "explicit interpreter without shebang produces no override diagnostic",
			interpreter:   "python3",
			script:        "print('ok')",
			wantEffective: "python3",
		},
		{
			name:        "concrete env without interpreter argument produces no override diagnostic",
			interpreter: "/usr/bin/env",
			script:      "#!/usr/bin/env python3\nprint('ok')",
			wantShebang: "python3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			analysis := AnalyzeScriptInterpreter(newInlineScriptInterpreterSource("inline script"), tt.interpreter, tt.script, RuntimeNative)
			if got := analysis.Effective().CommandString(); got != tt.wantEffective {
				t.Fatalf("Effective().CommandString() = %q, want %q", got, tt.wantEffective)
			}
			if got := analysis.Shebang().CommandString(); got != tt.wantShebang {
				t.Fatalf("Shebang().CommandString() = %q, want %q", got, tt.wantShebang)
			}
			if diagnostics := analysis.Diagnostics(); len(diagnostics) != 0 {
				t.Fatalf("Diagnostics() = %#v, want none", diagnostics)
			}
		})
	}
}

func TestScriptInterpreterValidationMutationContracts(t *testing.T) {
	t.Parallel()

	validSource := newInlineScriptInterpreterSource("inline script")
	invalidKindSource := ScriptInterpreterSource{kind: "unk" + "nown", label: "inline script"}
	blankPath := FilesystemPath(" \t")
	invalidPathSource := ScriptInterpreterSource{
		kind:  ScriptInterpreterSourceFile,
		path:  &blankPath,
		label: "script file",
	}
	if err := invalidPathSource.Validate(); !errors.Is(err, ErrInvalidFilesystemPath) {
		t.Fatalf("invalid path source Validate() error = %v, want ErrInvalidFilesystemPath", err)
	}

	tests := []struct {
		name     string
		validate func() error
		wantText string
	}{
		{
			name:     "source rejects invalid kind",
			validate: invalidKindSource.Validate,
			wantText: "invalid script interpreter source kind",
		},
		{
			name:     "source rejects invalid path",
			validate: invalidPathSource.Validate,
			wantText: "script interpreter source path",
		},
		{
			name: "diagnostic rejects invalid code before source",
			validate: ScriptInterpreterDiagnostic{
				code:   "unk" + "nown",
				source: validSource,
			}.Validate,
			wantText: "invalid script interpreter diagnostic code",
		},
		{
			name: "diagnostic rejects invalid source",
			validate: ScriptInterpreterDiagnostic{
				code:   ScriptInterpreterDiagnosticShebangOverride,
				source: invalidKindSource,
			}.Validate,
			wantText: "invalid script interpreter source kind",
		},
		{
			name: "analysis rejects invalid source",
			validate: ScriptInterpreterAnalysis{
				source:     invalidKindSource,
				provenance: ScriptInterpreterProvenanceExplicit,
			}.Validate,
			wantText: "invalid script interpreter source kind",
		},
		{
			name: "analysis rejects invalid provenance",
			validate: ScriptInterpreterAnalysis{
				source:     validSource,
				provenance: "unk" + "nown",
			}.Validate,
			wantText: "invalid script interpreter provenance",
		},
		{
			name: "analysis scans every diagnostic",
			validate: ScriptInterpreterAnalysis{
				source:     validSource,
				provenance: ScriptInterpreterProvenanceExplicit,
				diagnostics: []ScriptInterpreterDiagnostic{
					{
						code:   ScriptInterpreterDiagnosticShebangOverride,
						source: validSource,
					},
					{
						code:   "unk" + "nown",
						source: validSource,
					},
				},
			}.Validate,
			wantText: "invalid script interpreter diagnostic code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("Validate() error = %v, want text %q", err, tt.wantText)
			}
		})
	}
}

func TestInterpreterSelectionsEquivalentMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		explicit ShebangInfo
		shebang  ShebangInfo
		runtime  RuntimeMode
		want     bool
	}{
		{
			name:     "native shell interpreter mismatch is not equivalent",
			explicit: ShebangInfo{Found: true, Interpreter: "bash"},
			shebang:  ShebangInfo{Found: true, Interpreter: "/bin/sh"},
			runtime:  RuntimeNative,
		},
		{
			name:     "virtual shell pair is equivalent regardless of arguments",
			explicit: ShebangInfo{Found: true, Interpreter: "bash", Args: []string{"-x"}},
			shebang:  ShebangInfo{Found: true, Interpreter: "/bin/sh", Args: []string{"-eu"}},
			runtime:  RuntimeVirtualSh,
			want:     true,
		},
		{
			name:     "virtual explicit non-shell with shell shebang is not equivalent",
			explicit: ShebangInfo{Found: true, Interpreter: "python3"},
			shebang:  ShebangInfo{Found: true, Interpreter: "/bin/sh"},
			runtime:  RuntimeVirtualSh,
		},
		{
			name:     "virtual shell with non-shell shebang is not equivalent",
			explicit: ShebangInfo{Found: true, Interpreter: "bash"},
			shebang:  ShebangInfo{Found: true, Interpreter: "python3"},
			runtime:  RuntimeVirtualSh,
		},
		{
			name:     "same interpreter and args are equivalent",
			explicit: ShebangInfo{Found: true, Interpreter: "python3", Args: []string{"-u"}},
			shebang:  ShebangInfo{Found: true, Interpreter: "python3", Args: []string{"-u"}},
			runtime:  RuntimeNative,
			want:     true,
		},
		{
			name:     "same interpreter with different args is not equivalent outside virtual shell",
			explicit: ShebangInfo{Found: true, Interpreter: "python3", Args: []string{"-u"}},
			shebang:  ShebangInfo{Found: true, Interpreter: "python3", Args: []string{"-B"}},
			runtime:  RuntimeNative,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := interpreterSelectionsEquivalent(tt.explicit, tt.shebang, tt.runtime); got != tt.want {
				t.Fatalf("interpreterSelectionsEquivalent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func assertScriptInterpreterDiagnostic(t *testing.T, tt scriptInterpreterDiagnosticCase) {
	t.Helper()

	got := AnalyzeScriptInterpreter(tt.source, tt.interpreter, tt.script, tt.runtime)
	assertScriptInterpreterSource(t, got.Source(), tt.source.Kind(), tt.source.Path(), tt.source.Label())
	if got.Provenance() != tt.wantProvenance {
		t.Fatalf("Provenance() = %q, want %q", got.Provenance(), tt.wantProvenance)
	}
	if got.Effective().CommandString() != tt.wantEffective {
		t.Fatalf("Effective() = %q, want %q", got.Effective().CommandString(), tt.wantEffective)
	}
	if got.Shebang().CommandString() != tt.wantShebang {
		t.Fatalf("Shebang() = %q, want %q", got.Shebang().CommandString(), tt.wantShebang)
	}
	diagnostics := got.Diagnostics()
	if (len(diagnostics) > 0) != tt.wantDiag {
		t.Fatalf("Diagnostics() len = %d, want diagnostic=%v", len(diagnostics), tt.wantDiag)
	}
	if tt.wantDiag {
		assertInterpreterDiagnosticMessage(t, diagnostics[0])
		assertScriptInterpreterSource(t, diagnostics[0].Source(), tt.source.Kind(), tt.source.Path(), tt.source.Label())
	}
}

func assertScriptInterpreterSource(
	t *testing.T,
	source ScriptInterpreterSource,
	wantKind ScriptInterpreterSourceKind,
	wantPath FilesystemPath,
	wantLabel ScriptInterpreterSourceLabel,
) {
	t.Helper()

	if got := source.Kind(); got != wantKind {
		t.Fatalf("Kind() = %q, want %q", got, wantKind)
	}
	if got := source.Path(); got != wantPath {
		t.Fatalf("Path() = %q, want %q", got, wantPath)
	}
	if got := source.Label(); got != wantLabel {
		t.Fatalf("Label() = %q, want %q", got, wantLabel)
	}
}

func assertInterpreterDiagnosticMessage(t *testing.T, diagnostic ScriptInterpreterDiagnostic) {
	t.Helper()

	message := diagnostic.Message().String()
	for _, token := range []string{"script.interpreter", "overrides shebang", "takes precedence"} {
		if !strings.Contains(message, token) {
			t.Fatalf("diagnostic message %q missing %q", message, token)
		}
	}
}
