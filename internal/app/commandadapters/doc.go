// SPDX-License-Identifier: MPL-2.0

// Package commandadapters provides infrastructure adapters for the command
// execution application service.
//
// The commandsvc package owns orchestration ports. This package owns concrete
// implementations for host SSH access, runtime registry construction, and
// interactive terminal execution so UI and transport details stay outside the
// service core.
package commandadapters
