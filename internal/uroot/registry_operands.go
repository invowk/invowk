// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/u-root/u-root/pkg/uroot/unixflag"
)

const (
	urootArgDashDash = "--"
	urootArgDash     = "-"

	gzipDefaultSuffix = ".gz"

	// operandPreprocessArgsToGoArgs: the built-in calls
	// fs.Parse(unixflag.ArgsToGoArgs(args)).
	operandPreprocessArgsToGoArgs operandPreprocess = 0
	// operandPreprocessNone: the built-in calls fs.Parse(args) (gzip).
	operandPreprocessNone operandPreprocess = 1
	// operandPreprocessFindReorder: the built-in calls
	// fs.Parse(reorderArgs(args)) (find).
	operandPreprocessFindReorder operandPreprocess = 2

	// operandHookFollow resolves an operand with HandlerContext.ResolvePath:
	// full resolution, following the last component.
	operandHookFollow operandHook = 0
	// operandHookNoFollow resolves an operand without following its last
	// component (rm operands, mv sources).
	operandHookNoFollow operandHook = 1
	// operandHookAsWritten keeps an operand as written: cat's "-" (standard
	// input) and the operands of tar with only -t.
	operandHookAsWritten operandHook = 2
)

var (
	// errUrootOperandRebuild reports that the argument list rebuilt for a
	// built-in would not be read back as the classification it was built from.
	errUrootOperandRebuild = errors.New("internal error: rebuilt arguments differ from their classification")
	// errGzipSuffixSeparator reports a gzip -S suffix that holds a path
	// separator.
	errGzipSuffixSeparator = errors.New("suffix must not contain a path separator")
	// errUrootParseFailure reports a parse error whose failing argument could
	// not be located.
	errUrootParseFailure = errors.New("internal error: cannot locate the failing option")

	// urootOperandRules lists the validated upstream built-ins and how each one
	// preprocesses its arguments before Go's flag parser reads them.
	urootOperandRules = map[string]operandPreprocess{
		urootCommandBase64: operandPreprocessArgsToGoArgs,
		"cat":              operandPreprocessArgsToGoArgs,
		"cp":               operandPreprocessArgsToGoArgs,
		"find":             operandPreprocessFindReorder,
		"gzip":             operandPreprocessNone,
		"ls":               operandPreprocessArgsToGoArgs,
		"mkdir":            operandPreprocessArgsToGoArgs,
		"mv":               operandPreprocessArgsToGoArgs,
		"rm":               operandPreprocessArgsToGoArgs,
		urootCommandShasum: operandPreprocessArgsToGoArgs,
		"tar":              operandPreprocessArgsToGoArgs,
		"touch":            operandPreprocessArgsToGoArgs,
	}
)

type (
	operandPreprocess int
	operandHook       int

	// urootOption is one option as Go's flag parser set it: its name, the
	// value it was set to, and whether it is a boolean option.
	urootOption struct {
		name   string
		value  string
		isBool bool
	}

	// urootArgs is a built-in's argument list as its parser reads it: the
	// options in the order they were parsed, and the operands.
	urootArgs struct {
		options  []urootOption
		operands []string
	}

	// recordingFlag is a flag.Value that records every Set in parse order.
	recordingFlag struct {
		name   string
		isBool bool
		parsed *[]urootOption
	}
)

func (f recordingFlag) String() string { return "" }

func (f recordingFlag) Set(value string) error {
	if f.isBool {
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("parse boolean: %w", err)
		}
	}
	*f.parsed = append(*f.parsed, urootOption{name: f.name, value: value, isBool: f.isBool})
	return nil
}

func (f recordingFlag) IsBoolFlag() bool { return f.isBool }

// urootCommandOperands validates the path operands of a validated upstream
// built-in (urootOperandRules) and returns the argument list the built-in
// runs with. args holds the command name in args[0].
//
// It reads the arguments as the built-in's own parser does: the built-in's
// preprocessing (unixflag.ArgsToGoArgs, find's reorderArgs, or none for gzip),
// then Go's flag package with a flag set built from the command's
// SupportedFlags. For the ArgsToGoArgs commands, a raw "--" before the first
// argument that does not start with "-" ends the options, as on a host shell,
// and is not handed on. Each operand is resolved with the hook its command and
// position select (operandHookFor), tar's -f/-file values are resolved in
// place, and every other option value is kept as written. For gzip the output
// path it derives from each operand is validated too (checkGzipOutputPaths).
//
// The built-in receives the options and their values, then "--", then the
// resolved operands, encoded so that its own preprocessing gives that list
// back. Before it runs, the rebuilt list is read again with the built-in's own
// preprocessing and must give the same options and operands; otherwise the
// command is refused (errUrootOperandRebuild). When Go's parser fails on the
// arguments, the built-in receives only the option that failed, so it reports
// its own error and no operand reaches it.
func urootCommandOperands(hc *HandlerContext, cmd Command, args []string) ([]string, error) {
	name := args[0]
	preprocess, ok := urootOperandRules[name]
	if !ok {
		return args, nil
	}
	flags := cmd.SupportedFlags()
	parsed, failing, err := readUrootArgs(preprocess, flags, args[1:], true)
	if err != nil {
		if failing == "" {
			return nil, fmt.Errorf("%w: %w", errUrootParseFailure, err)
		}
		return []string{name, encodeUrootFailingOption(preprocess, failing)}, nil
	}
	resolved, err := resolveUrootArgs(hc, name, parsed)
	if err != nil {
		return nil, err
	}
	if name == "gzip" {
		if err := checkGzipOutputPaths(hc, resolved); err != nil {
			return nil, err
		}
	}
	rebuilt := rebuildUrootArgs(preprocess, resolved)
	if err := checkUrootRebuild(preprocess, flags, rebuilt, resolved); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return append([]string{name}, rebuilt...), nil
}

// readUrootArgs reads raw as a built-in with the given preprocessing reads it.
// With dashDash, the host-shell "--" rule of urootCommandOperands applies.
// On a parse error it returns the argument the parser failed on, as the
// parser saw it.
//
//goplint:ignore -- raw command-line strings in and out.
func readUrootArgs(preprocess operandPreprocess, flags []FlagInfo, raw []string, dashDash bool) (urootArgs, string, error) {
	if preprocess == operandPreprocessArgsToGoArgs && dashDash {
		if index := leadingDashDash(raw); index >= 0 {
			parsed, failing, err := parseUrootArgs(flags, unixflag.ArgsToGoArgs(raw[:index]))
			if err != nil {
				return urootArgs{}, failing, err
			}
			parsed.operands = append(parsed.operands, raw[index+1:]...)
			return parsed, "", nil
		}
	}
	return parseUrootArgs(flags, preprocessUrootArgs(preprocess, raw))
}

// leadingDashDash returns the index of a "--" among the arguments before the
// first one that does not start with "-", or -1.
func leadingDashDash(raw []string) int {
	for i, arg := range raw {
		if arg == urootArgDashDash {
			return i
		}
		if !strings.HasPrefix(arg, urootArgDash) {
			return -1
		}
	}
	return -1
}

//goplint:ignore -- raw command-line strings in and out.
func preprocessUrootArgs(preprocess operandPreprocess, raw []string) []string {
	switch preprocess {
	case operandPreprocessArgsToGoArgs:
		return unixflag.ArgsToGoArgs(raw)
	case operandPreprocessFindReorder:
		return findReorderArgs(raw)
	case operandPreprocessNone:
		return slices.Clone(raw)
	default:
		return slices.Clone(raw)
	}
}

// parseUrootArgs runs Go's flag parser over list with a flag set built from
// flags: a boolean option for each flag without a value, a string option for
// each flag with one, under its Name and, when it differs, its ShortName.
//
//goplint:ignore -- raw command-line strings in and out.
func parseUrootArgs(flags []FlagInfo, list []string) (urootArgs, string, error) {
	var options []urootOption
	fs := flag.NewFlagSet("invowk-operands", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() { /* the built-in prints its own usage */ }
	for _, info := range flags {
		for _, name := range []string{info.Name, info.ShortName} {
			if name == "" || fs.Lookup(name) != nil {
				continue
			}
			fs.Var(recordingFlag{name: name, isBool: !info.TakesValue, parsed: &options}, name, "")
		}
	}
	if err := fs.Parse(list); err != nil {
		// The parser stops on the failing argument. It has consumed it when
		// it is an undefined option, an option without its value or with a
		// malformed value; it has not when its syntax is bad ("---x", "-=").
		index := len(list) - len(fs.Args())
		if index >= len(list) || !isBadFlagSyntax(list[index]) {
			index--
		}
		parseErr := fmt.Errorf("parse arguments: %w", err)
		if index < 0 || index >= len(list) {
			return urootArgs{}, "", parseErr
		}
		return urootArgs{}, list[index], parseErr
	}
	return urootArgs{options: options, operands: slices.Clone(fs.Args())}, "", nil
}

// isBadFlagSyntax reports whether Go's flag parser rejects arg without
// consuming it: an argument that starts with "-", is not "--", and whose name
// after its one or two leading dashes is empty or starts with "-" or "=".
//
//goplint:ignore -- raw command-line string.
func isBadFlagSyntax(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' || arg == urootArgDashDash {
		return false
	}
	name := arg[1:]
	if name[0] == '-' {
		name = name[1:]
	}
	return name == "" || name[0] == '-' || name[0] == '='
}

// encodeUrootFailingOption writes an argument the parser failed on so that
// the built-in's preprocessing gives it back unchanged.
//
//goplint:ignore -- raw command-line strings in and out.
func encodeUrootFailingOption(preprocess operandPreprocess, arg string) string {
	if preprocess == operandPreprocessArgsToGoArgs && strings.HasPrefix(arg, urootArgDash) {
		return urootArgDash + arg
	}
	return arg
}

// resolveUrootArgs resolves the operands of parsed and tar's archive paths.
func resolveUrootArgs(hc *HandlerContext, name string, parsed urootArgs) (urootArgs, error) {
	resolved := urootArgs{
		options:  slices.Clone(parsed.options),
		operands: make([]string, len(parsed.operands)),
	}
	if name == "tar" {
		for i, option := range resolved.options {
			if option.name != "f" && option.name != "file" {
				continue
			}
			path, err := hc.ResolvePath(option.value)
			if err != nil {
				return urootArgs{}, err
			}
			resolved.options[i].value = path
		}
	}
	for i, operand := range parsed.operands {
		path, err := resolveUrootOperand(hc, operandHookFor(name, parsed, i, operand), operand)
		if err != nil {
			return urootArgs{}, err
		}
		resolved.operands[i] = path
	}
	return resolved, nil
}

// operandHookFor selects how operand i of a built-in is resolved.
//
//goplint:ignore -- raw command-line operand.
func operandHookFor(name string, parsed urootArgs, index int, operand string) operandHook {
	switch name {
	case "cat":
		if operand == urootArgDash {
			return operandHookAsWritten
		}
	case "rm":
		return operandHookNoFollow
	case "mv":
		if index < len(parsed.operands)-1 || len(parsed.operands) == 1 {
			return operandHookNoFollow
		}
	case "tar":
		if parsed.boolOption("t", "list") && !parsed.boolOption("c", "create") && !parsed.boolOption("x", "extract") {
			return operandHookAsWritten
		}
	}
	return operandHookFollow
}

//goplint:ignore -- raw command-line operand in, raw OS path out.
func resolveUrootOperand(hc *HandlerContext, hook operandHook, operand string) (string, error) {
	switch hook {
	case operandHookFollow:
		return hc.ResolvePath(operand)
	case operandHookAsWritten:
		return operand, nil
	case operandHookNoFollow:
		// Until the no-follow hook exists, rm and mv operands use full
		// resolution like every other operand.
		return hc.ResolvePath(operand)
	default:
		return hc.ResolvePath(operand)
	}
}

// boolOption reports the last value any of names was set to, false when none
// was set.
func (a urootArgs) boolOption(names ...string) bool {
	set := false
	for _, option := range a.options {
		if slices.Contains(names, option.name) {
			value, err := strconv.ParseBool(option.value)
			set = err == nil && value
		}
	}
	return set
}

// stringOption returns the last value any of names was set to, or fallback.
//
//goplint:ignore -- raw option value.
func (a urootArgs) stringOption(fallback string, names ...string) string {
	value := fallback
	for _, option := range a.options {
		if slices.Contains(names, option.name) {
			value = option.value
		}
	}
	return value
}

// checkGzipOutputPaths validates the file gzip writes for each operand. A -S
// suffix that holds a path separator is refused under both access modes.
// Unless -c or -t is set, the output path of each resolved operand (plus the
// suffix, or with -d without it, as upstream derives it) is validated with the
// follow hook, because gzip creates it with os.Create. The validated path is
// not handed on: gzip derives it again from the same resolved operand.
func checkGzipOutputPaths(hc *HandlerContext, resolved urootArgs) error {
	suffix := resolved.stringOption(gzipDefaultSuffix, "S")
	if strings.ContainsRune(suffix, '/') || (runtime.GOOS == "windows" && strings.ContainsRune(suffix, '\\')) {
		return fmt.Errorf("-S %q: %w", suffix, errGzipSuffixSeparator)
	}
	if resolved.boolOption("c") || resolved.boolOption("t") {
		return nil
	}
	decompress := resolved.boolOption("d")
	for _, operand := range resolved.operands {
		output := operand + suffix
		if decompress {
			output = strings.TrimSuffix(operand, suffix)
		}
		if _, err := hc.ResolvePath(output); err != nil {
			return err
		}
	}
	return nil
}

// rebuildUrootArgs writes resolved as an argument list for the built-in: the
// options and their values, then "--", then the operands, so no operand can
// be read as an option. For the ArgsToGoArgs commands each argument that
// starts with "-" and comes before the first one that does not is written
// with one more "-", which ArgsToGoArgs takes off again. find gets its options
// in reverse parse order and its operands, with no "--": its reorderArgs
// moves every argument that starts with "-" to the front in reverse order.
//
//goplint:ignore -- raw command-line strings out.
func rebuildUrootArgs(preprocess operandPreprocess, resolved urootArgs) []string {
	options := make([]string, 0, len(resolved.options))
	for _, option := range resolved.options {
		if option.isBool && option.value == "true" {
			options = append(options, urootArgDash+option.name)
			continue
		}
		options = append(options, urootArgDash+option.name+"="+option.value)
	}
	if preprocess == operandPreprocessFindReorder {
		slices.Reverse(options)
		return append(options, resolved.operands...)
	}
	list := append(append(options, urootArgDashDash), resolved.operands...)
	if preprocess != operandPreprocessArgsToGoArgs {
		return list
	}
	for i, arg := range list {
		if !strings.HasPrefix(arg, urootArgDash) {
			break
		}
		list[i] = urootArgDash + arg
	}
	return list
}

// checkUrootRebuild reads rebuilt with only the built-in's own preprocessing
// (no "--" rule: the list is no longer what a user wrote) and refuses unless
// it gives exactly want.
//
//goplint:ignore -- raw command-line strings in.
func checkUrootRebuild(preprocess operandPreprocess, flags []FlagInfo, rebuilt []string, want urootArgs) error {
	got, _, err := readUrootArgs(preprocess, flags, rebuilt, false)
	if err != nil {
		return fmt.Errorf("%w: %w", errUrootOperandRebuild, err)
	}
	if !slices.Equal(got.options, want.options) || !slices.Equal(got.operands, want.operands) {
		return errUrootOperandRebuild
	}
	return nil
}

// findReorderArgs is upstream find's reorderArgs (u-root v0.16.0,
// pkg/core/find/find.go), copied: every argument that starts with "-" moves
// to the front, with the next argument for -name, -type and -mode, and each
// moved argument is put before the ones moved earlier; the others keep their
// order.
//
//goplint:ignore -- raw command-line strings in and out.
func findReorderArgs(args []string) []string {
	var reordered []string
	for i := 0; i < len(args); {
		arg := args[i]
		expectsValue := arg == "-name" || arg == "-type" || arg == "-mode"
		switch {
		case expectsValue && i+1 < len(args):
			reordered = append([]string{arg, args[i+1]}, reordered...)
			i += 2
		case strings.HasPrefix(arg, urootArgDash):
			reordered = append([]string{arg}, reordered...)
			i++
		default:
			reordered = append(reordered, arg)
			i++
		}
	}
	return reordered
}
