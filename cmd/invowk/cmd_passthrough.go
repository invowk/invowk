// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type (
	passthroughContextKey struct{}

	passthroughPayload struct {
		args        []string //goplint:ignore -- raw passthrough CLI tokens forwarded verbatim
		set         bool
		preParseErr error
	}

	boundaryScan struct {
		preFlags    []string //goplint:ignore -- raw pre-command CLI flag tokens
		fromSource  string   //goplint:ignore -- raw --ivk-from flag text validated downstream
		configPath  string   //goplint:ignore -- raw --ivk-config flag text validated downstream
		noCwd       bool
		noCwdSet    bool
		noCwdErr    error
		positionals []string //goplint:ignore -- raw post-flag positional tokens
	}
)

func withPassthroughPayload(ctx context.Context, payload passthroughPayload) context.Context {
	return context.WithValue(ctx, passthroughContextKey{}, payload)
}

//goplint:ignore -- passthrough arguments are raw CLI tokens forwarded verbatim to scripts.
func passthroughFromContext(ctx context.Context) (args []string, preParseErr error, ok bool) {
	if ctx == nil {
		return nil, nil, false
	}
	payload, exists := ctx.Value(passthroughContextKey{}).(passthroughPayload)
	if !exists {
		return nil, nil, false
	}
	if payload.preParseErr != nil {
		return nil, payload.preParseErr, true
	}
	if !payload.set {
		return nil, nil, false
	}
	return payload.args, nil, true
}

// preparePassthroughInvocation inspects args before Cobra parses flags and, when
// the invocation targets a passthrough command under `invowk cmd`, splits argv
// at the command path boundary without mutating os.Args. Pre-command Invowk
// flags are parsed onto cmdCmd, while post-command tokens are carried in ctx.
//
//goplint:ignore -- parses raw CLI argv tokens at the Cobra adapter boundary.
func preparePassthroughInvocation(ctx context.Context, app *App, rootCmd *cobra.Command, args []string) (effectiveArgs, colorScanArgs []string, nextCtx context.Context) {
	if app == nil || app.Discovery == nil || rootCmd == nil {
		return args, args, ctx
	}
	cmdCmd := findCmdSubcommand(rootCmd)
	if cmdCmd == nil {
		return args, args, ctx
	}

	scan, ok := scanCmdBoundary(rootCmd.PersistentFlags(), cmdCmd.PersistentFlags(), args)
	if !ok || len(scan.positionals) == 0 || scan.noCwdErr != nil {
		return args, args, ctx
	}

	noCwd := scan.noCwd
	if !scan.noCwdSet {
		var envErr error
		noCwd, _, envErr = noCwdFromEnv(app.Getenv)
		if envErr != nil {
			return args, args, ctx
		}
	}

	lookupCtx := contextWithDiscoveryScope(ctx, scan.configPath, noCwd)
	result, err := app.Discovery.DiscoverAndValidateCommandSet(lookupCtx)
	if err != nil || result.Set == nil || discovery.CurrentDirLoadError(result.Diagnostics) != nil {
		return args, args, ctx
	}

	cmdPath, postArgs, matched := resolvePassthroughTarget(result.Set, scan.fromSource, scan.positionals)
	if !matched {
		return args, args, ctx
	}

	// Clear any config path pre-populated from a full-argv scan (for example,
	// when a post-command --ivk-config belongs to a wrapped invowk invocation).
	if cfgFlag := rootCmd.PersistentFlags().Lookup("ivk-config"); cfgFlag != nil {
		if setErr := cfgFlag.Value.Set(""); setErr != nil {
			return args, args, ctx
		}
	}

	if parseErr := cmdCmd.ParseFlags(scan.preFlags); parseErr != nil {
		wrappedErr := cmdCmd.FlagErrorFunc()(cmdCmd, parseErr)
		nextCtx = withPassthroughPayload(ctx, passthroughPayload{preParseErr: wrappedErr})
		cmdOnly := []string{"cmd"}
		rootCmd.SetArgs(cmdOnly)
		return cmdOnly, scan.preFlags, nextCtx
	}

	nextCtx = withPassthroughPayload(ctx, passthroughPayload{args: postArgs, set: true})
	preArgs := make([]string, 0, 1+len(cmdPath))
	preArgs = append(preArgs, "cmd")
	preArgs = append(preArgs, cmdPath...)
	rootCmd.SetArgs(preArgs)
	return preArgs, scan.preFlags, nextCtx
}

func findCmdSubcommand(rootCmd *cobra.Command) *cobra.Command {
	for _, child := range rootCmd.Commands() {
		if child.Name() == "cmd" {
			return child
		}
	}
	return nil
}

// scanCmdBoundary walks raw argv up to the first positional token after `cmd`,
// separating root/cmd persistent flags from command-resolution positionals.
//
//goplint:ignore -- operates on raw CLI argv tokens at the Cobra boundary.
func scanCmdBoundary(rootFlags, cmdFlags *pflag.FlagSet, args []string) (boundaryScan, bool) {
	var scan boundaryScan

	cmdIdx := -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || arg == "--help" || arg == "-h" || arg == "--version" {
			return boundaryScan{}, false
		}
		if strings.HasPrefix(arg, "--") {
			nextIdx, ok := consumeLongPreFlag(rootFlags, nil, args, i, &scan)
			if !ok {
				return boundaryScan{}, false
			}
			i = nextIdx
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			nextIdx, ok := consumeShortPreFlag(rootFlags, nil, args, i, &scan)
			if !ok {
				return boundaryScan{}, false
			}
			i = nextIdx
			continue
		}
		if arg != "cmd" {
			return boundaryScan{}, false
		}
		cmdIdx = i
		break
	}
	if cmdIdx < 0 {
		return boundaryScan{}, false
	}

	for i := cmdIdx + 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			scan.positionals = args[i+1:]
			return scan, true
		}
		if arg == "--help" || arg == "-h" {
			return boundaryScan{}, false
		}
		if strings.HasPrefix(arg, "--") {
			nextIdx, ok := consumeLongPreFlag(cmdFlags, rootFlags, args, i, &scan)
			if !ok {
				return boundaryScan{}, false
			}
			i = nextIdx
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			nextIdx, ok := consumeShortPreFlag(cmdFlags, rootFlags, args, i, &scan)
			if !ok {
				return boundaryScan{}, false
			}
			i = nextIdx
			continue
		}
		scan.positionals = args[i:]
		return scan, true
	}

	return scan, true
}

//goplint:ignore -- pre-parse CLI boundary helper operates on raw argv slices and indices.
func consumeLongPreFlag(primary, secondary *pflag.FlagSet, args []string, idx int, scan *boundaryScan) (int, bool) {
	arg := args[idx]
	name, inlineVal, hasEq := strings.Cut(arg[2:], "=")
	flag := lookupPreFlag(name, primary, secondary)
	if flag == nil {
		// An unknown `--flag=val` does not consume the following token, so we can
		// keep scanning and let cmdCmd.ParseFlags reject it if the target is passthrough.
		// Without '=', we cannot tell whether an unknown flag takes a value argument.
		if !hasEq {
			return idx, false
		}
		scan.preFlags = append(scan.preFlags, arg)
		return idx, true
	}

	scan.preFlags = append(scan.preFlags, arg)
	if hasEq {
		recordPreFlagValue(flag.Name, inlineVal, scan)
		return idx, true
	}
	if flag.NoOptDefVal != "" {
		if flag.Name == noCwdFlagName {
			scan.noCwd = true
			scan.noCwdSet = true
		}
		return idx, true
	}
	if idx+1 < len(args) {
		idx++
		scan.preFlags = append(scan.preFlags, args[idx])
		recordPreFlagValue(flag.Name, args[idx], scan)
	}
	return idx, true
}

//goplint:ignore -- pre-parse CLI boundary helper operates on raw argv slices and indices.
func consumeShortPreFlag(primary, secondary *pflag.FlagSet, args []string, idx int, scan *boundaryScan) (int, bool) {
	arg := args[idx]
	shortChars := arg[1:]

	// Single short flag with optional inline `=val` or attached value (e.g. `-w/tmp`).
	first := shortChars[:1]
	flag := lookupShortPreFlag(first, primary, secondary)
	if flag == nil {
		if len(shortChars) > 1 && shortChars[1] == '=' {
			scan.preFlags = append(scan.preFlags, arg)
			return idx, true
		}
		return idx, false
	}

	scan.preFlags = append(scan.preFlags, arg)
	if flag.NoOptDefVal == "" {
		if len(shortChars) == 1 {
			if idx+1 < len(args) {
				idx++
				scan.preFlags = append(scan.preFlags, args[idx])
				recordPreFlagValue(flag.Name, args[idx], scan)
			}
			return idx, true
		}
		val := strings.TrimPrefix(shortChars[1:], "=")
		recordPreFlagValue(flag.Name, val, scan)
		return idx, true
	}

	// Bool short flag: validate any remaining bundled short flags (e.g. `-vW` or `-vw /tmp`).
	rest := shortChars[1:]
	if val, isEq := strings.CutPrefix(rest, "="); isEq {
		recordPreFlagValue(flag.Name, val, scan)
		return idx, true
	}
	for j := range len(rest) {
		nextChar := rest[j : j+1]
		nextFlag := lookupShortPreFlag(nextChar, primary, secondary)
		if nextFlag == nil {
			return idx, false
		}
		if nextFlag.NoOptDefVal == "" {
			if j+1 < len(rest) {
				recordPreFlagValue(nextFlag.Name, strings.TrimPrefix(rest[j+1:], "="), scan)
				return idx, true
			}
			if idx+1 < len(args) {
				idx++
				scan.preFlags = append(scan.preFlags, args[idx])
				recordPreFlagValue(nextFlag.Name, args[idx], scan)
			}
			return idx, true
		}
	}
	return idx, true
}

//goplint:ignore -- pre-parse helper looks up pflag names by string identifier.
func lookupPreFlag(name string, primary, secondary *pflag.FlagSet) *pflag.Flag {
	if primary != nil {
		if f := primary.Lookup(name); f != nil {
			return f
		}
	}
	if secondary != nil {
		return secondary.Lookup(name)
	}
	return nil
}

//goplint:ignore -- pre-parse helper looks up pflag short flags by character.
func lookupShortPreFlag(short string, primary, secondary *pflag.FlagSet) *pflag.Flag {
	if primary != nil {
		if f := primary.ShorthandLookup(short); f != nil {
			return f
		}
	}
	if secondary != nil {
		return secondary.ShorthandLookup(short)
	}
	return nil
}

//goplint:ignore -- pre-parse flag value extraction reads raw argv strings.
func recordPreFlagValue(name, value string, scan *boundaryScan) {
	switch name {
	case "ivk-from":
		scan.fromSource = value
	case "ivk-config":
		scan.configPath = value
	case noCwdFlagName:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			scan.noCwdErr = err
			scan.noCwdSet = true
			return
		}
		scan.noCwd = parsed
		scan.noCwdSet = true
	}
}

// resolvePassthroughTarget mirrors CommandService command resolution on
// commandSet (supporting @source, --ivk-from, unambiguous SimpleName, and
// source-qualified `<source> <simpleName>`). It returns the command-path prefix
// and remaining post-command tokens only when the matched command has
// Passthrough == true.
//
//goplint:ignore -- resolves raw positional tokens against discovered commands.
func resolvePassthroughTarget(
	commandSet *discovery.DiscoveredCommandSet,
	fromFlag string,
	positionals []string,
) (cmdPath, postArgs []string, matched bool) {
	if commandSet == nil || len(positionals) == 0 {
		return nil, nil, false
	}

	if fromFlag != "" || strings.HasPrefix(positionals[0], "@") {
		rawSource := fromFlag
		rest := positionals
		consumedPrefix := 0
		if rawSource == "" {
			rawSource = positionals[0]
			rest = positionals[1:]
			consumedPrefix = 1
		}
		if len(rest) == 0 {
			return nil, nil, false
		}
		sourceID, err := parseSourceName(rawSource)
		if err != nil || sourceID == "" {
			return nil, nil, false
		}
		sourceCmds := commandSet.BySource[sourceID]
		for i := len(rest); i > 0; i-- {
			candidate := strings.Join(rest[:i], " ")
			for _, cmdInfo := range sourceCmds {
				if string(cmdInfo.SimpleName) == candidate || string(cmdInfo.Name) == candidate {
					if cmdInfo.Command == nil || !cmdInfo.Command.Passthrough {
						return nil, nil, false
					}
					splitIdx := consumedPrefix + i
					return slices.Clone(positionals[:splitIdx]), slices.Clone(positionals[splitIdx:]), true
				}
			}
		}
		return nil, nil, false
	}

	for i := len(positionals); i > 0; i-- {
		candidate := invowkfile.CommandName(strings.Join(positionals[:i], " "))
		if err := candidate.Validate(); err != nil {
			continue
		}
		if commandSet.AmbiguousNames[candidate] {
			return nil, nil, false
		}
		if target := commandSet.ByName[candidate]; target != nil {
			if target.Command == nil || !target.Command.Passthrough {
				return nil, nil, false
			}
			return slices.Clone(positionals[:i]), slices.Clone(positionals[i:]), true
		}
		if simpleMatches := commandSet.BySimpleName[candidate]; len(simpleMatches) == 1 {
			target := simpleMatches[0]
			if target.Command == nil || !target.Command.Passthrough {
				return nil, nil, false
			}
			return slices.Clone(positionals[:i]), slices.Clone(positionals[i:]), true
		}
	}

	return nil, nil, false
}
