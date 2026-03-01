// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// isStdinPiped reports whether stdin has piped content (not a terminal).
func isStdinPiped() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// readStdinAll reads all piped stdin content into a string.
// Returns an error wrapping noInputMsg when stdin is a terminal.
func readStdinAll(noInputMsg string) (string, error) {
	if !isStdinPiped() {
		return "", errors.New(noInputMsg)
	}

	var sb strings.Builder
	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadString('\n')
		sb.WriteString(line)

		if err != nil {
			if err == io.EOF {
				break
			}

			return "", fmt.Errorf("error reading stdin: %w", err)
		}
	}

	return sb.String(), nil
}
