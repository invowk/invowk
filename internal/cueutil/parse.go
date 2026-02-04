// SPDX-License-Identifier: MPL-2.0

package cueutil

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// ParseResult contains the result of a successful CUE parse operation.
type ParseResult[T any] struct {
	// Value is the decoded Go struct.
	Value *T

	// Unified is the unified CUE value, available for advanced use cases
	// such as extracting additional metadata or performing custom validation.
	Unified cue.Value
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
//   - schemaPath: The path to the root definition (e.g., "#Invkfile", "#Config")
//   - opts: Optional configuration
//
// Returns:
//   - *ParseResult[T] containing the decoded struct and unified CUE value
//   - error with formatted path information if parsing fails
func ParseAndDecode[T any](schema, data []byte, schemaPath string, opts ...Option) (*ParseResult[T], error) {
	// Apply options
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	// Determine filename for error messages
	filename := options.filename
	if filename == "" {
		filename = "<input>"
	}

	// Early file size check to prevent OOM attacks from large files
	if err := CheckFileSize(data, options.maxFileSize, filename); err != nil {
		return nil, err
	}

	ctx := cuecontext.New()

	// Step 1: Compile the schema
	schemaValue := ctx.CompileBytes(schema)
	if schemaValue.Err() != nil {
		return nil, fmt.Errorf("internal error: failed to compile schema: %w", schemaValue.Err())
	}

	// Step 2: Compile the user data
	userValue := ctx.CompileBytes(data, cue.Filename(filename))
	if userValue.Err() != nil {
		return nil, FormatError(userValue.Err(), filename)
	}

	// Look up the root definition in the schema
	schemaRoot := schemaValue.LookupPath(cue.ParsePath(schemaPath))
	if schemaRoot.Err() != nil {
		return nil, fmt.Errorf("internal error: schema definition %s not found: %w", schemaPath, schemaRoot.Err())
	}

	// Unify user data with schema
	unified := schemaRoot.Unify(userValue)

	// Step 3: Validate
	if options.concrete {
		if err := unified.Validate(cue.Concrete(true)); err != nil {
			return nil, FormatError(err, filename)
		}
	} else {
		if err := unified.Validate(); err != nil {
			return nil, FormatError(err, filename)
		}
	}

	// Decode into struct
	var result T
	if err := unified.Decode(&result); err != nil {
		return nil, FormatError(err, filename)
	}

	return &ParseResult[T]{
		Value:   &result,
		Unified: unified,
	}, nil
}

// ParseAndDecodeString is a convenience wrapper that accepts schema as string.
// Useful when the schema is embedded as a string constant rather than bytes.
func ParseAndDecodeString[T any](schema string, data []byte, schemaPath string, opts ...Option) (*ParseResult[T], error) {
	return ParseAndDecode[T]([]byte(schema), data, schemaPath, opts...)
}
