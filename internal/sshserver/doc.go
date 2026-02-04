// SPDX-License-Identifier: MPL-2.0

// Package sshserver provides an SSH server using the Wish library for container callback.
//
// This allows container-executed commands to SSH back into the host system.
// The server only accepts connections from commands that invowk itself has spawned,
// using a token-based authentication mechanism.
package sshserver
