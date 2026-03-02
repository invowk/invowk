// SPDX-License-Identifier: MPL-2.0

// Package types defines cross-cutting DDD Value Types used by multiple domain
// packages (invowkfile, invowkmod, etc.). These are foundation types that carry
// semantic meaning and validation but have no domain-specific dependencies.
//
// This package is a leaf dependency: it imports only the standard library.
// Domain packages import it; it never imports domain packages.
package types
