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

type statReader interface {
	Stat() (os.FileInfo, error)
}

// isInputPiped reports whether input has provided content. Injected readers are
// treated as provided input so Cobra SetIn tests do not need process-wide stdin.
func isInputPiped(input io.Reader) bool {
	file, ok := input.(statReader)
	if !ok {
		return true
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// readInputAll reads all provided input content into a string.
// Returns an error wrapping noInputMsg when stdin is a terminal.
//
//goplint:ignore -- CLI adapter helper carries transient terminal text and display error text.
func readInputAll(input io.Reader, noInputMsg string) (string, error) {
	if !isInputPiped(input) {
		return "", errors.New(noInputMsg)
	}

	var sb strings.Builder
	reader := bufio.NewReader(input)

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
