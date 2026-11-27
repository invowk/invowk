// SPDX-License-Identifier: MPL-2.0

// Package cueutil provides shared CUE parsing utilities.
//
// The package consolidates the 3-step CUE parsing pattern used across invowkfile,
// invowkmod, and config packages:
//
//  1. Compile the embedded schema
//  2. Compile user data and unify with schema
//  3. Validate and decode to Go struct
//
// # Usage
//
// For embedded schemas parsed repeatedly, precompile once with [CompileSchema]
// (for example via [sync.OnceValues]) and pass the result to
// [ParseAndDecodeCompiled]:
//
//	//go:embed invowkfile_schema.cue
//	var schemaBytes []byte
//
//	var compiledSchema = sync.OnceValues(func() (*cueutil.CompiledSchema, error) {
//	    return cueutil.CompileSchema(schemaBytes, "#Invowkfile")
//	})
//
//	schema, err := compiledSchema()
//	if err != nil {
//	    return nil, err
//	}
//	result, err := cueutil.ParseAndDecodeCompiled[Invowkfile](
//	    schema,
//	    userFileBytes,
//	    cueutil.WithFilename("invowkfile.cue"),
//	)
//	if err != nil {
//	    return nil, err  // Error includes CUE path for debugging
//	}
//	return result.Value, nil
//
// For one-off parses, [ParseAndDecode] and [ParseAndDecodeString] compile the
// schema and user data together in a single call.
package cueutil
