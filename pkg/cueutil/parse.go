// SPDX-License-Identifier: MPL-2.0

package cueutil

import (
	"errors"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const invalidCompiledSchemaErrMsg = "internal error: compiled schema is nil or uninitialized"

// ErrInvalidCompiledSchema is returned when ParseAndDecodeCompiled receives a
// nil or zero-value CompiledSchema.
var ErrInvalidCompiledSchema = errors.New(invalidCompiledSchemaErrMsg)

type (
	// ParseResult contains the result of a successful CUE parse operation.
	ParseResult[T any] struct {
		// Value is the decoded Go struct.
		Value *T

		// Unified is the unified CUE value, available for advanced use cases
		// such as extracting additional metadata or performing custom validation.
		Unified cue.Value
	}

	// CompiledSchema holds a precompiled CUE schema root definition so repeated
	// parses do not recompile the embedded schema or reload its standard-library
	// imports (such as "strings") on every call.
	//
	// A CompiledSchema is immutable after construction and safe for concurrent use:
	// each ParseAndDecodeCompiled call compiles user input into a fresh,
	// short-lived cue.Context and unifies it against the precompiled schema root.
	CompiledSchema struct {
		root     cue.Value
		compiled bool
	}
)

// CompileSchema compiles CUE schema bytes once and resolves schemaPath so
// repeated calls to ParseAndDecodeCompiled reuse the compiled schema and its
// loaded standard-library imports.
//
//goplint:ignore -- raw embedded CUE schema bytes at the cueutil parser boundary.
func CompileSchema(schema []byte, schemaPath CUEPath) (*CompiledSchema, error) {
	if err := schemaPath.Validate(); err != nil {
		return nil, fmt.Errorf("internal error: invalid schema path: %w", err)
	}

	ctx := cuecontext.New()
	schemaValue := ctx.CompileBytes(schema)
	if schemaValue.Err() != nil {
		return nil, fmt.Errorf("internal error: failed to compile schema: %w", schemaValue.Err())
	}

	schemaRoot := schemaValue.LookupPath(cue.ParsePath(schemaPath.String()))
	if schemaRoot.Err() != nil {
		return nil, fmt.Errorf("internal error: schema definition %s not found: %w", schemaPath, schemaRoot.Err())
	}

	return &CompiledSchema{
		root:     schemaRoot,
		compiled: true,
	}, nil
}

// ParseAndDecodeCompiled parses user CUE bytes against a precompiled schema:
//
//  1. Compile user data in a fresh, short-lived cue.Context
//  2. Unify user data with the precompiled schema root
//  3. Validate and decode to Go struct
//
//goplint:ignore -- raw user CUE bytes at the cueutil parser boundary.
func ParseAndDecodeCompiled[T any](schema *CompiledSchema, data []byte, opts ...Option) (*ParseResult[T], error) {
	options := resolveOptions(opts)

	if err := CheckFileSize(data, options.maxFileSize, options.filename); err != nil {
		return nil, err
	}
	if schema == nil || !schema.compiled {
		return nil, ErrInvalidCompiledSchema
	}

	userCtx := cuecontext.New()
	userValue := userCtx.CompileBytes(data, cue.Filename(options.filename))
	if userValue.Err() != nil {
		return nil, FormatError(userValue.Err(), options.filename)
	}

	// Keep schema.root as the receiver so CUE evaluates with the schema runtime
	// (where stdlib builtins like "strings" are already compiled) while userCtx
	// is discarded after the call.
	return unifyValidateDecode[T](schema.root, userValue, options)
}

// ParseAndDecode performs the 3-step CUE parsing flow:
//
//  1. Compile the embedded schema
//  2. Compile user data and unify with schema
//  3. Validate and decode to Go struct
//
// Parameters:
//   - schema: The embedded CUE schema bytes (from //go:embed)
//   - data: The user-provided CUE file bytes
//   - schemaPath: The path to the root definition (e.g., "#Invowkfile", "#Config")
//   - opts: Optional configuration
//
// Returns:
//   - *ParseResult[T] containing the decoded struct and unified CUE value
//   - error with formatted path information if parsing fails
func ParseAndDecode[T any](schema, data []byte, schemaPath string, opts ...Option) (*ParseResult[T], error) {
	options := resolveOptions(opts)

	// Early file size check to prevent OOM attacks from large files
	if err := CheckFileSize(data, options.maxFileSize, options.filename); err != nil {
		return nil, err
	}

	ctx := cuecontext.New()

	// Step 1: Compile the schema
	schemaValue := ctx.CompileBytes(schema)
	if schemaValue.Err() != nil {
		return nil, fmt.Errorf("internal error: failed to compile schema: %w", schemaValue.Err())
	}

	// Step 2: Compile the user data before looking up schemaPath so user syntax
	// errors surface directly even when schemaPath is missing.
	userValue := ctx.CompileBytes(data, cue.Filename(options.filename))
	if userValue.Err() != nil {
		return nil, FormatError(userValue.Err(), options.filename)
	}

	// Look up the root definition in the schema
	schemaRoot := schemaValue.LookupPath(cue.ParsePath(schemaPath))
	if schemaRoot.Err() != nil {
		return nil, fmt.Errorf("internal error: schema definition %s not found: %w", schemaPath, schemaRoot.Err())
	}

	return unifyValidateDecode[T](schemaRoot, userValue, options)
}

// ParseAndDecodeString is a convenience wrapper that accepts schema as string.
// Useful when the schema is embedded as a string constant rather than bytes.
func ParseAndDecodeString[T any](schema string, data []byte, schemaPath string, opts ...Option) (*ParseResult[T], error) {
	return ParseAndDecode[T]([]byte(schema), data, schemaPath, opts...)
}

func resolveOptions(opts []Option) parseOptions {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}
	if options.filename == "" {
		options.filename = "<input>"
	}
	return options
}

func unifyValidateDecode[T any](schemaRoot, userValue cue.Value, options parseOptions) (*ParseResult[T], error) {
	unified := schemaRoot.Unify(userValue)

	var validateOptions []cue.Option
	if options.concrete {
		validateOptions = append(validateOptions, cue.Concrete(true))
	}
	if err := unified.Validate(validateOptions...); err != nil {
		return nil, FormatError(err, options.filename)
	}

	var result T
	if err := unified.Decode(&result); err != nil {
		return nil, FormatError(err, options.filename)
	}

	return &ParseResult[T]{
		Value:   &result,
		Unified: unified,
	}, nil
}
