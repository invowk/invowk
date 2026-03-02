// SPDX-License-Identifier: MPL-2.0

// This file previously contained the commandService struct and its methods.
// The implementation has been extracted to internal/app/commandsvc/.
// The CLI adapter (cliCommandAdapter) in app.go bridges the service to the
// CommandService interface used by Cobra handlers.
package cmd
