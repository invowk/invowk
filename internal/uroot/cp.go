// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/u-root/u-root/pkg/core/cp"
	"github.com/u-root/u-root/pkg/uroot/unixflag"
)

var errCpCannotCopyIntoSelf = errors.New("cannot copy a directory into itself")

// cpCommand wraps the u-root cp implementation.
type cpCommand struct {
	baseWrapper
}

// newCpCommand creates a new cp command wrapper.
func newCpCommand() *cpCommand {
	return &cpCommand{
		name: "cp",
		flags: []FlagInfo{
			{Name: "recursive", ShortName: "r", Description: "copy directories recursively"},
			{Name: "RECURSIVE", ShortName: "R", Description: "same as -r"},
			{Name: "interactive", ShortName: "i", Description: "prompt before overwriting a file"},
			{Name: "force", ShortName: "f", Description: "force copy by removing destination file if needed"},
			{Name: "verbose", ShortName: "v", Description: "print each copied file"},
			{Name: "no-dereference", ShortName: "P", Description: "never follow symbolic links"},
		},
	}
}

// Run executes the cp command.
// Note: This uses u-root's streaming implementation which uses io.Copy internally,
// ensuring constant memory usage regardless of file size.
//
//goplint:ignore -- Command interface uses raw argv strings.
func (c *cpCommand) Run(ctx context.Context, args []string) error {
	if len(args) > 1 {
		if err := checkCpIntoSelf(GetHandlerContext(ctx), args[1:]); err != nil {
			return wrapError(c.name, err)
		}
	}
	return c.runUpstream(ctx, cp.New(), args)
}

//goplint:ignore -- raw CLI/OS path strings.
func checkCpIntoSelf(hc *HandlerContext, raw []string) error {
	recursive, noFollow, operands, ok := parseCpFlagsAndOperands(raw)
	if !ok || !recursive || len(operands) < 2 {
		return nil
	}
	sources, rawTo := operands[:len(operands)-1], operands[len(operands)-1]
	toPath := hc.joinDir(rawTo)
	toInfo, err := os.Stat(toPath)
	toDir := err == nil && toInfo.IsDir()

	for _, rawSrc := range sources {
		srcPath := hc.joinDir(rawSrc)
		srcInfo, ok := cpStat(srcPath, noFollow)
		if !ok || !srcInfo.IsDir() {
			continue
		}
		dst := toPath
		if toDir {
			dst = filepath.Join(toPath, filepath.Base(srcPath))
		}
		if cpDstIsSelfEntry(srcInfo, dst, noFollow) {
			continue
		}
		if cpDstInsideSourceDir(srcInfo, dst) {
			return fmt.Errorf("%w: %s", errCpCannotCopyIntoSelf, rawTo)
		}
	}
	return nil
}

//goplint:ignore -- raw OS path strings.
func cpDstIsSelfEntry(srcInfo os.FileInfo, dst string, noFollow bool) bool {
	if noFollow {
		return false
	}
	dstInfo, ok := cpStat(dst, false)
	return ok && os.SameFile(srcInfo, dstInfo)
}

//goplint:ignore -- raw OS path strings.
func cpDstInsideSourceDir(srcInfo os.FileInfo, dst string) bool {
	anc := cpDeepestExistingAncestor(dst)
	for anc != "" {
		if ancInfo, err := os.Stat(anc); err == nil && os.SameFile(srcInfo, ancInfo) {
			return true
		}
		parent := filepath.Dir(anc)
		if parent == anc {
			return false
		}
		anc = parent
	}
	return false
}

//goplint:ignore -- raw OS path strings.
func cpDeepestExistingAncestor(path string) string {
	curr := filepath.Clean(path)
	for {
		if resolved, err := filepath.EvalSymlinks(curr); err == nil {
			return resolved
		}
		if _, err := os.Lstat(curr); err == nil {
			return curr
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			return ""
		}
		curr = parent
	}
}

//goplint:ignore -- raw CLI flag and operand strings.
func parseCpFlagsAndOperands(raw []string) (recursive, noFollow bool, operands []string, ok bool) {
	var tail []string
	if idx := leadingDashDash(raw); idx >= 0 {
		raw, tail = raw[:idx], raw[idx+1:]
	}
	fs := flag.NewFlagSet("cp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	r1 := fs.Bool("r", false, "")
	r2 := fs.Bool("recursive", false, "")
	r3 := fs.Bool("R", false, "")
	r4 := fs.Bool("RECURSIVE", false, "")
	p1 := fs.Bool("P", false, "")
	p2 := fs.Bool("no-dereference", false, "")
	for _, f := range []string{"i", "interactive", "f", "force", "v", "verbose"} {
		fs.Bool(f, false, "")
	}
	if err := fs.Parse(unixflag.ArgsToGoArgs(raw)); err != nil {
		return false, false, nil, false
	}
	ops := make([]string, 0, len(fs.Args())+len(tail))
	ops = append(ops, fs.Args()...)
	ops = append(ops, tail...)
	return *r1 || *r2 || *r3 || *r4, *p1 || *p2, ops, true
}
