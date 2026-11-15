// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package fstree

import (
	"errors"
	"fmt"
	"os"
)

// errNoJunctions is returned by createJunction and probeJunction off Windows;
// junction instances are skipped there before creation.
var errNoJunctions = errors.New("directory junctions are supported only on Windows")

func createSymlink(target, link string) error {
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}
	return nil
}

// createJunction is never reached off Windows: junction instances are skipped.
func createJunction(_, _ string) error { return errNoJunctions }

func probeJunction(string) error { return errNoJunctions }
