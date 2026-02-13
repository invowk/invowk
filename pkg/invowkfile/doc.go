// SPDX-License-Identifier: MPL-2.0

// Package invowkfile provides types and parsing for invowkfile.cue command definitions.
//
// An invowkfile defines commands with implementations for different runtimes (native,
// virtual, container) and platforms. This package handles CUE schema validation,
// parsing to Go structs, and command/implementation selection.
//
// This package uses pkg/cueutil for CUE parsing implementation details.
// External consumers should use the exported Parse() and ParseBytes() functions;
// the CUE parsing internals are not part of the public API.
package invowkfile
