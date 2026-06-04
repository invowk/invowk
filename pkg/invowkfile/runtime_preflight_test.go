// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
)

func TestParseBytesRuntimePreflightDiagnostics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runtime     string
		wantField   string
		wantMessage string
	}{
		{
			name:        "native rejects container-only field",
			runtime:     `{name: "native", persistent: {create_if_missing: true}}`,
			wantField:   "cmds[0].implementations[0].runtimes[0].persistent",
			wantMessage: "persistent is only valid for container runtime",
		},
		{
			name:        "container requires source",
			runtime:     `{name: "container"}`,
			wantField:   "cmds[0].implementations[0].runtimes[0]",
			wantMessage: "container runtime requires either image or containerfile",
		},
		{
			name:        "container rejects duplicate source",
			runtime:     `{name: "container", image: "debian:stable-slim", containerfile: "Containerfile"}`,
			wantField:   "cmds[0].implementations[0].runtimes[0].image",
			wantMessage: "image and containerfile are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseBytes([]byte(invowkfileWithRuntime(tt.runtime)), "runtime-preflight.cue")
			if err == nil {
				t.Fatal("ParseBytes() error = nil, want runtime preflight diagnostic")
			}
			got := err.Error()
			if !strings.Contains(got, tt.wantField) {
				t.Fatalf("ParseBytes() error missing field %q:\n%s", tt.wantField, got)
			}
			if !strings.Contains(got, tt.wantMessage) {
				t.Fatalf("ParseBytes() error missing message %q:\n%s", tt.wantMessage, got)
			}
		})
	}
}

func invowkfileWithRuntime(runtime string) string {
	return `cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo test"}
		runtimes: [` + runtime + `]
		platforms: [{name: "linux"}]
	}]
}]`
}

func TestRuntimePreflightMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("invalid cue falls back without preflight diagnostics", func(t *testing.T) {
		t.Parallel()

		errs := runtimeSchemaPreflightValidationErrors([]byte("cmds: ["), "bad.cue")
		if len(errs) != 0 {
			t.Fatalf("runtimeSchemaPreflightValidationErrors() = %v, want no fallback diagnostics", errs)
		}
	})

	t.Run("missing or unknown runtime name is ignored", func(t *testing.T) {
		t.Parallel()

		for _, runtime := range []*ast.StructLit{
			nil,
			parseRuntimePreflightStruct(t, `{image: "debian:stable-slim"}`),
			parseRuntimePreflightStruct(t, `{name: localRuntime, image: "debian:stable-slim"}`),
			parseRuntimePreflightStruct(t, `{name: 42, image: "debian:stable-slim"}`),
			parseRuntimePreflightStruct(t, `{name: "custom", image: "debian:stable-slim"}`),
		} {
			errs := validateRuntimePreflight(runtime, "cmds[0].implementations[0].runtimes[0]")
			if len(errs) != 0 {
				t.Fatalf("validateRuntimePreflight(%#v) = %v, want no diagnostics", runtime, errs)
			}
		}
	})

	t.Run("native rejects every non native field", func(t *testing.T) {
		t.Parallel()

		errs := validateRuntimePreflight(parseRuntimePreflightStruct(t, `{
			name: "native"
			allowed_binaries: ["git"]
			binary_lookup_mode: "host"
			cpu_limit: 1
			memory_limit: "1M"
			depends_on: {binaries: ["git"]}
			enable_host_ssh: true
			containerfile: "Containerfile"
			image: "debian:stable-slim"
			persistent: {create_if_missing: true}
			ports: ["8080:80"]
			volumes: ["./data:/data"]
		}`), "runtime")

		requireRuntimePreflightErrorCount(t, errs, 11)
		requireRuntimePreflightDiagnostic(t, errs, "runtime.allowed_binaries", "allowed_binaries is only valid for virtual runtimes")
		requireRuntimePreflightDiagnostic(t, errs, "runtime.binary_lookup_mode", "binary_lookup_mode is only valid for virtual runtimes")
		requireRuntimePreflightDiagnostic(t, errs, "runtime.cpu_limit", "cpu_limit is only valid for virtual-lua runtime")
		requireRuntimePreflightDiagnostic(t, errs, "runtime.memory_limit", "memory_limit is only valid for virtual-lua runtime")
		requireRuntimePreflightDiagnostic(t, errs, "runtime.depends_on", "depends_on is only valid for container runtime")
		requireRuntimePreflightDiagnostic(t, errs, "runtime.enable_host_ssh", "enable_host_ssh is only valid for container runtime")
		requireRuntimePreflightDiagnostic(t, errs, "runtime.containerfile", "containerfile is only valid for container runtime")
		requireRuntimePreflightDiagnostic(t, errs, "runtime.image", "image is only valid for container runtime")
		requireRuntimePreflightDiagnostic(t, errs, "runtime.persistent", "persistent is only valid for container runtime")
		requireRuntimePreflightDiagnostic(t, errs, "runtime.ports", "ports is only valid for container runtime")
		requireRuntimePreflightDiagnostic(t, errs, "runtime.volumes", "volumes is only valid for container runtime")
	})

	t.Run("virtual runtime applies lua field split", func(t *testing.T) {
		t.Parallel()

		virtualShErrs := validateRuntimePreflight(parseRuntimePreflightStruct(t, `{
			name: "virtual-sh"
			cpu_limit: 1
			memory_limit: "1M"
		}`), "runtime")
		requireRuntimePreflightErrorCount(t, virtualShErrs, 2)
		requireRuntimePreflightDiagnostic(t, virtualShErrs, "runtime.cpu_limit", "cpu_limit is only valid for virtual-lua runtime")
		requireRuntimePreflightDiagnostic(t, virtualShErrs, "runtime.memory_limit", "memory_limit is only valid for virtual-lua runtime")

		virtualLuaErrs := validateRuntimePreflight(parseRuntimePreflightStruct(t, `{
			name: "virtual-lua"
			allowed_binaries: ["lua"]
			binary_lookup_mode: "strict"
			cpu_limit: 1
			memory_limit: "1M"
		}`), "runtime")
		if len(virtualLuaErrs) != 0 {
			t.Fatalf("validateRuntimePreflight(virtual-lua) = %v, want no diagnostics", virtualLuaErrs)
		}

		virtualLuaContainerErrs := validateRuntimePreflight(parseRuntimePreflightStruct(t, `{
			name: "virtual-lua"
			image: "debian:stable-slim"
		}`), "runtime")
		requireRuntimePreflightErrorCount(t, virtualLuaContainerErrs, 1)
		requireRuntimePreflightDiagnostic(t, virtualLuaContainerErrs, "runtime.image", "image is only valid for container runtime")
	})

	t.Run("container rejects virtual fields and enforces source selection", func(t *testing.T) {
		t.Parallel()

		imageOnlyErrs := validateRuntimePreflight(parseRuntimePreflightStruct(t, `{
			name: "container"
			image: "debian:stable-slim"
		}`), "runtime")
		if len(imageOnlyErrs) != 0 {
			t.Fatalf("validateRuntimePreflight(container image-only) = %v, want no diagnostics", imageOnlyErrs)
		}

		containerfileOnlyErrs := validateRuntimePreflight(parseRuntimePreflightStruct(t, `{
			name: "container"
			containerfile: "Containerfile"
		}`), "runtime")
		if len(containerfileOnlyErrs) != 0 {
			t.Fatalf("validateRuntimePreflight(containerfile-only) = %v, want no diagnostics", containerfileOnlyErrs)
		}

		fieldErrs := validateRuntimePreflight(parseRuntimePreflightStruct(t, `{
			name: "container"
			image: "debian:stable-slim"
			allowed_binaries: ["git"]
			binary_lookup_mode: "host"
			cpu_limit: 1
			memory_limit: "1M"
		}`), "runtime")
		requireRuntimePreflightErrorCount(t, fieldErrs, 4)
		requireRuntimePreflightDiagnostic(t, fieldErrs, "runtime.allowed_binaries", "allowed_binaries is only valid for virtual runtimes")
		requireRuntimePreflightDiagnostic(t, fieldErrs, "runtime.binary_lookup_mode", "binary_lookup_mode is only valid for virtual runtimes")
		requireRuntimePreflightDiagnostic(t, fieldErrs, "runtime.cpu_limit", "cpu_limit is only valid for virtual-lua runtime")
		requireRuntimePreflightDiagnostic(t, fieldErrs, "runtime.memory_limit", "memory_limit is only valid for virtual-lua runtime")

		duplicateSourceErrs := validateRuntimePreflight(parseRuntimePreflightStruct(t, `{
			name: "container"
			image: "debian:stable-slim"
			containerfile: "Containerfile"
		}`), "runtime")
		requireRuntimePreflightErrorCount(t, duplicateSourceErrs, 1)
		requireRuntimePreflightDiagnostic(
			t,
			duplicateSourceErrs,
			"runtime.image",
			"image and containerfile are mutually exclusive; choose exactly one container source",
		)

		missingSourceErrs := validateRuntimePreflight(parseRuntimePreflightStruct(t, `{
			name: "container"
		}`), "runtime")
		requireRuntimePreflightErrorCount(t, missingSourceErrs, 1)
		requireRuntimePreflightDiagnostic(
			t,
			missingSourceErrs,
			"runtime",
			"container runtime requires either image or containerfile",
		)
	})

	t.Run("schema preflight traverses nested runtime indexes", func(t *testing.T) {
		t.Parallel()

		errs := runtimeSchemaPreflightValidationErrors([]byte(`cmds: [
			{implementations: [{runtimes: [{name: "native"}]}]},
			{implementations: [
				{runtimes: [{name: "virtual-sh"}]},
				{runtimes: [{name: "virtual-lua"}, {name: "container"}]},
			]},
		]`), "nested.cue")
		requireRuntimePreflightErrorCount(t, errs, 1)
		requireRuntimePreflightDiagnostic(
			t,
			errs,
			"cmds[1].implementations[1].runtimes[1]",
			"container runtime requires either image or containerfile",
		)
	})

	t.Run("ast helpers ignore missing and non list fields", func(t *testing.T) {
		t.Parallel()

		if got := fieldList(nil, "cmds"); len(got) != 0 {
			t.Fatalf("fieldList(nil) length = %d, want 0", len(got))
		}
		if got := fieldList(parseRuntimePreflightStruct(t, `{cmds: "not-list"}`), "cmds"); len(got) != 0 {
			t.Fatalf("fieldList(non-list) length = %d, want 0", len(got))
		}
		got := fieldList(parseRuntimePreflightStruct(t, `{cmds: [{name: "native"}, "skip", 42]}`), "cmds")
		if len(got) != 1 || !hasField(got[0], "name") {
			t.Fatalf("fieldList(mixed list) = %#v, want only struct item", got)
		}
		if hasField(nil, "name") {
			t.Fatal("hasField(nil) = true, want false")
		}
	})
}

func TestRuntimePreflightErrorFields(t *testing.T) {
	t.Parallel()

	err := runtimePreflightError("runtime.image", "message")
	if err.Validator != runtimePreflightValidatorName {
		t.Fatalf("Validator = %q, want %q", err.Validator, runtimePreflightValidatorName)
	}
	if err.Field != "runtime.image" {
		t.Fatalf("Field = %q, want runtime.image", err.Field)
	}
	if err.Message != "message" {
		t.Fatalf("Message = %q, want message", err.Message)
	}
	if err.Severity != SeverityError {
		t.Fatalf("Severity = %v, want SeverityError", err.Severity)
	}

	if got := runtimeFieldPath(3, 4, 5); got != "cmds[3].implementations[4].runtimes[5]" {
		t.Fatalf("runtimeFieldPath() = %q, want indexed runtime path", got)
	}
}

func parseRuntimePreflightStruct(t *testing.T, src string) *ast.StructLit {
	t.Helper()

	expr, err := parser.ParseExpr("runtime.cue", src)
	if err != nil {
		t.Fatalf("ParseExpr(%q) = %v", src, err)
	}
	st, ok := expr.(*ast.StructLit)
	if !ok {
		t.Fatalf("ParseExpr(%q) = %T, want *ast.StructLit", src, expr)
	}
	return st
}

func requireRuntimePreflightErrorCount(t *testing.T, errs ValidationErrors, want int) {
	t.Helper()

	if len(errs) != want {
		t.Fatalf("preflight error count = %d, want %d: %v", len(errs), want, errs)
	}
}

func requireRuntimePreflightDiagnostic(t *testing.T, errs ValidationErrors, field, message string) {
	t.Helper()

	for _, err := range errs {
		if err.Validator == runtimePreflightValidatorName &&
			err.Field == field &&
			err.Message == message &&
			err.Severity == SeverityError {
			return
		}
	}
	t.Fatalf("preflight errors = %v, want field %q message %q", errs, field, message)
}
