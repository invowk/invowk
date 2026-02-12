// SPDX-License-Identifier: MPL-2.0

// Package cueutil provides shared CUE parsing utilities.
//
// The package consolidates the 3-step CUE parsing pattern used across invkfile,
// invkmod, and config packages:
//
//  1. Compile the embedded schema
//  2. Compile user data and unify with schema
//  3. Validate and decode to Go struct
//
// # Usage
//
//	//go:embed invkfile_schema.cue
//	var schemaBytes []byte
//
//	result, err := cueutil.ParseAndDecode[Invkfile](
//	    schemaBytes,
//	    userFileBytes,
//	    "#Invkfile",
//	    cueutil.WithFilename("invkfile.cue"),
//	)
//	if err != nil {
//	    return nil, err  // Error includes CUE path for debugging
//	}
//	return result.Value, nil
package cueutil
