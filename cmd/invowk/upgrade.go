// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/selfupdate"
	"github.com/invowk/invowk/internal/tui"
)

// upgradeParams bundles the dependencies and flags for the upgrade command,
// enabling the core logic in runUpgrade to be tested without a real Cobra
// command or live GitHub API calls.
type upgradeParams struct {
	stdout  io.Writer
	stderr  io.Writer
	updater *selfupdate.Updater
	target  string // target version (empty = latest)
	check   bool   // --check mode: report availability without installing
	yes     bool   // --yes flag: skip confirmation prompt
}

// newUpgradeCommand creates the `invowk upgrade` command, which updates the
// binary to the latest stable release or a specific version from GitHub Releases.
func newUpgradeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade [version]",
		Short: "Update invowk to the latest stable release or a specific version",
		Long: `Update invowk to the latest stable release or a specific version.

The upgrade command downloads the new binary from GitHub Releases,
verifies its SHA256 checksum, and atomically replaces the current binary.

If invowk was installed via Homebrew or go install, the command suggests
using the appropriate package manager instead.`,
		Example: `  # Upgrade to latest stable
  invowk upgrade

  # Check for updates without installing
  invowk upgrade --check

  # Upgrade to a specific version
  invowk upgrade v1.2.0

  # Skip confirmation prompt
  invowk upgrade --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			checkFlag, _ := cmd.Flags().GetBool("check")
			yesFlag, _ := cmd.Flags().GetBool("yes")

			var target string
			if len(args) > 0 {
				target = args[0]
			}

			// Build GitHub client options, adding a token if available
			// for higher rate limits (5000/hour vs 60/hour unauthenticated).
			var clientOpts []selfupdate.ClientOption
			if token := os.Getenv("GITHUB_TOKEN"); token != "" {
				clientOpts = append(clientOpts, selfupdate.WithToken(token))
			}
			clientOpts = append(clientOpts, selfupdate.WithUserAgent("invowk/"+Version))

			client := selfupdate.NewGitHubClient(clientOpts...)
			updater := selfupdate.NewUpdater(Version, selfupdate.WithGitHubClient(client))

			p := upgradeParams{
				stdout:  cmd.OutOrStdout(),
				stderr:  cmd.ErrOrStderr(),
				updater: updater,
				target:  target,
				check:   checkFlag,
				yes:     yesFlag,
			}

			if err := runUpgrade(cmd.Context(), p); err != nil {
				fmt.Fprintln(p.stderr, formatUpgradeError(err))
				return &ExitError{Code: classifyUpgradeExitCode(err), Err: err}
			}

			return nil
		},
	}

	cmd.Flags().Bool("check", false, "Check for available upgrade without installing")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}

// runUpgrade is the core upgrade logic, separated from Cobra for testability.
// All user-facing output goes through p.stdout and p.stderr.
//
// Flow:
//  1. Check for available upgrade via the GitHub API.
//  2. If the install is managed (Homebrew/go install), print guidance and return.
//  3. If already up-to-date, print status and return.
//  4. If --check, print availability and return.
//  5. Otherwise, confirm with the user (unless --yes), download, verify, and replace.
func runUpgrade(ctx context.Context, p upgradeParams) error {
	check, err := p.updater.Check(ctx, p.target)
	if err != nil {
		return fmt.Errorf("checking for upgrade: %w", err)
	}

	// Managed installs (Homebrew, go install) should use their respective
	// package managers. The Check method returns a pre-formatted message.
	if check.InstallMethod == selfupdate.InstallMethodHomebrew ||
		check.InstallMethod == selfupdate.InstallMethodGoInstall {
		fmt.Fprintln(p.stdout, check.Message)
		return nil
	}

	// Not upgrade available: already up-to-date or running a pre-release ahead
	// of the latest stable version.
	if !check.UpgradeAvailable {
		fmt.Fprintf(p.stdout, "Current version: %s\n", check.CurrentVersion)
		if check.LatestVersion != "" {
			fmt.Fprintf(p.stdout, "Latest version:  %s\n", check.LatestVersion)
		}
		fmt.Fprintf(p.stdout, "\n%s\n", check.Message)
		return nil
	}

	// Upgrade available, check-only mode: report and exit without installing.
	if p.check {
		fmt.Fprintf(p.stdout, "Current version: %s\n", check.CurrentVersion)
		fmt.Fprintf(p.stdout, "Latest version:  %s\n", check.LatestVersion)
		fmt.Fprintf(p.stdout, "\nAn upgrade is available: %s \u2192 %s\n", check.CurrentVersion, check.LatestVersion)
		fmt.Fprintln(p.stdout, "Run 'invowk upgrade' to install.")
		return nil
	}

	// Upgrade available, apply mode: confirm, download, verify, and replace.
	fmt.Fprintf(p.stdout, "Current version: %s\n", check.CurrentVersion)
	fmt.Fprintf(p.stdout, "Latest version:  %s\n", check.LatestVersion)

	if !p.yes {
		confirmed, confirmErr := tui.Confirm(tui.ConfirmOptions{
			Title:       fmt.Sprintf("Upgrade invowk from %s to %s?", check.CurrentVersion, check.LatestVersion),
			Affirmative: "Yes",
			Negative:    "No",
		})
		if confirmErr != nil {
			return fmt.Errorf("confirmation prompt: %w", confirmErr)
		}
		if !confirmed {
			return nil
		}
	}

	fmt.Fprintf(p.stdout, "\nDownloading invowk %s...\n", check.LatestVersion)

	if err := p.updater.Apply(ctx, check.TargetRelease); err != nil {
		return fmt.Errorf("applying upgrade: %w", err)
	}

	fmt.Fprintln(p.stdout, "Verifying checksum... OK")
	fmt.Fprintln(p.stdout, "Replacing binary...  OK")
	fmt.Fprintln(p.stdout, SuccessStyle.Render(fmt.Sprintf("Successfully upgraded to %s", check.LatestVersion)))

	return nil
}

// classifyUpgradeExitCode maps an upgrade error to the appropriate process exit code.
// Permission errors and missing releases use exit code 1 (user-correctable);
// all other failures use exit code 2 (unexpected/transient).
func classifyUpgradeExitCode(err error) int {
	switch {
	case errors.Is(err, os.ErrPermission):
		return 1
	case errors.Is(err, selfupdate.ErrReleaseNotFound):
		return 1
	default:
		return 2
	}
}

// formatUpgradeError produces a user-friendly error message with actionable
// remediation guidance tailored to the specific error type.
func formatUpgradeError(err error) string {
	var rateLimitErr *selfupdate.RateLimitError
	if errors.As(err, &rateLimitErr) {
		return fmt.Sprintf("%s\n\nTo increase your rate limit, set a GitHub token:\n  export GITHUB_TOKEN=ghp_...\nThen retry: invowk upgrade",
			rateLimitErr.Error())
	}

	var checksumErr *selfupdate.ChecksumError
	if errors.As(err, &checksumErr) {
		return fmt.Sprintf("checksum verification failed for %s\n\nExpected: %s\nGot:      %s\n\nThe download may be corrupted. Please try again.\nIf this persists, report at https://github.com/invowk/invowk/issues",
			checksumErr.Filename, checksumErr.Expected, checksumErr.Got)
	}

	if errors.Is(err, os.ErrPermission) {
		return "insufficient permissions to replace the binary\n\nTry running with elevated privileges:\n  sudo invowk upgrade"
	}

	return fmt.Sprintf("%s\n\nCheck your network connection and try again.\nIf behind a firewall, set GITHUB_TOKEN for authenticated access.", err.Error())
}
