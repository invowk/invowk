// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

const (
	completionShellBash       completionShell = "bash"
	completionShellZsh        completionShell = "zsh"
	completionShellFish       completionShell = "fish"
	completionShellPowerShell completionShell = "powershell"

	errMsgInvalidCompletionShell        = "invalid completion shell"
	errMsgInvalidCompletionAlias        = "invalid --alias"
	errMsgInvalidCompletionPrefix       = "invalid --prefix"
	errMsgCompletionPrefixWithoutAlias  = "cannot use --prefix without --alias"
	errMsgCompletionTemplateRewriteFail = "failed to rewrite completion template"
)

var (
	// errInvalidCompletionShell is returned when the completion shell argument is unrecognized.
	errInvalidCompletionShell = errors.New(errMsgInvalidCompletionShell)
	// errInvalidCompletionAlias is returned when --alias is empty or contains unsafe characters.
	errInvalidCompletionAlias = errors.New(errMsgInvalidCompletionAlias)
	// errInvalidCompletionPrefix is returned when --prefix contains control characters or unclosed quotes.
	errInvalidCompletionPrefix = errors.New(errMsgInvalidCompletionPrefix)
	// errCompletionPrefixWithoutAlias is returned when --prefix is specified without --alias.
	errCompletionPrefixWithoutAlias = errors.New(errMsgCompletionPrefixWithoutAlias)
	// errCompletionTemplateRewriteFail is returned when Cobra's generated script lacks an expected anchor.
	errCompletionTemplateRewriteFail = errors.New(errMsgCompletionTemplateRewriteFail)

	// completionAliasRE restricts --alias to safe command identifier tokens so
	// generated shell completion scripts cannot inject arbitrary shell code.
	completionAliasRE = regexp.MustCompile(`\A[a-zA-Z0-9][a-zA-Z0-9._-]*\z`)
)

type (
	// completionFlagValues holds the flag bindings for `invowk completion`.
	completionFlagValues struct {
		// alias binds completions to a standalone wrapper/alias command name.
		alias string //goplint:ignore -- Cobra flag binding validated into completionAlias at adapter boundary.
		// prefix specifies shell-quoted tokens prepended to `invowk __complete`.
		prefix string //goplint:ignore -- Cobra flag binding parsed into completionPrefixTokens at adapter boundary.
	}

	// completionShell identifies a supported shell target for `invowk completion`.
	completionShell string

	// completionAlias is a validated command identifier for wrapper completions.
	completionAlias string

	// completionPrefixTokens is a validated sequence of literal tokens prepended
	// to `invowk __complete` when completing an alias/wrapper command.
	completionPrefixTokens []string

	// invalidCompletionShellError reports an unrecognized shell target.
	invalidCompletionShellError struct {
		value completionShell
	}

	// invalidCompletionAliasError reports an unsafe or empty --alias value.
	invalidCompletionAliasError struct {
		value completionAlias
	}
)

// Error implements the error interface for invalidCompletionShellError.
func (e *invalidCompletionShellError) Error() string {
	return fmt.Sprintf("%s %q: must be one of bash, zsh, fish, powershell", errMsgInvalidCompletionShell, e.value)
}

// Unwrap returns errInvalidCompletionShell for errors.Is() compatibility.
func (e *invalidCompletionShellError) Unwrap() error { return errInvalidCompletionShell }

// Error implements the error interface for invalidCompletionAliasError.
func (e *invalidCompletionAliasError) Error() string {
	return fmt.Sprintf("%s %q: must start with an alphanumeric character and contain only letters, digits, '.', '_', or '-'", errMsgInvalidCompletionAlias, e.value)
}

// Unwrap returns errInvalidCompletionAlias for errors.Is() compatibility.
func (e *invalidCompletionAliasError) Unwrap() error { return errInvalidCompletionAlias }

// String returns the string representation of completionShell.
func (s completionShell) String() string { return string(s) }

// Validate ensures completionShell is one of bash, zsh, fish, or powershell.
func (s completionShell) Validate() error {
	switch s {
	case completionShellBash, completionShellZsh, completionShellFish, completionShellPowerShell:
		return nil
	default:
		return &invalidCompletionShellError{value: s}
	}
}

// String returns the string representation of completionAlias.
func (a completionAlias) String() string { return string(a) }

// Validate ensures completionAlias is a safe non-empty command identifier.
func (a completionAlias) Validate() error {
	if !completionAliasRE.MatchString(string(a)) {
		return &invalidCompletionAliasError{value: a}
	}
	return nil
}

// ident returns a shell-identifier-safe name derived from the alias by replacing
// '-' and '.' with '_', preventing invalid variable names (e.g. in Fish) and
// avoiding collisions with invowk's own completion helpers.
//
//goplint:ignore -- derives shell-identifier-safe string from validated completionAlias.
func (a completionAlias) ident() string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(string(a))
}

// String returns the space-joined representation of completionPrefixTokens.
func (p completionPrefixTokens) String() string {
	return strings.Join(p, " ")
}

// Validate checks that no prefix token contains NUL or newline characters.
func (p completionPrefixTokens) Validate() error {
	for _, tok := range p {
		if strings.ContainsAny(tok, "\x00\n\r") {
			return fmt.Errorf("%w: token must not contain NUL or newline characters", errInvalidCompletionPrefix)
		}
	}
	return nil
}

// newCompletionCommand creates the `invowk completion` command.
func newCompletionCommand() *cobra.Command {
	flags := &completionFlagValues{}

	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for invowk.

To enable shell completions, run one of the following commands:

` + SubtitleStyle.Render("Bash:") + `
  # Add to ~/.bashrc:
  eval "$(invowk completion bash)"

  # Or install system-wide:
  invowk completion bash > /etc/bash_completion.d/invowk

` + SubtitleStyle.Render("Zsh:") + `
  # Add to ~/.zshrc:
  eval "$(invowk completion zsh)"

  # Or install to fpath:
  invowk completion zsh > "${fpath[1]}/_invowk"

` + SubtitleStyle.Render("Fish:") + `
  invowk completion fish > ~/.config/fish/completions/invowk.fish

` + SubtitleStyle.Render("PowerShell:") + `
  invowk completion powershell | Out-String | Invoke-Expression

  # Or add to $PROFILE:
  invowk completion powershell >> $PROFILE

` + SubtitleStyle.Render("Standalone Wrapper Aliases:") + `
  # Generate completions for a wrapper command (e.g. cst -> invowk -c /etc/cst/config.cue --ivk-no-cwd cmd cst "$@"):
  eval "$(invowk completion bash --alias cst --prefix "-c /etc/cst/config.cue --ivk-no-cwd cmd cst")"
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompletion(cmd, args[0], flags)
		},
	}

	completionCmd.Flags().StringVar(&flags.alias, "alias", "", "command name to bind generated completions to (e.g. for wrapper scripts)")
	completionCmd.Flags().StringVar(&flags.prefix, "prefix", "", "tokens prepended to invowk __complete (cannot be used without --alias)")

	return completionCmd
}

//goplint:ignore -- CLI boundary parses raw shell arg and --alias/--prefix flags before rewriting.
func runCompletion(cmd *cobra.Command, rawShell string, flags *completionFlagValues) error {
	shell := completionShell(rawShell)
	if err := shell.Validate(); err != nil {
		return newUsageError(err)
	}

	aliasSet := cmd.Flags().Changed("alias") || flags.alias != ""
	prefixSet := cmd.Flags().Changed("prefix") || flags.prefix != ""
	if prefixSet && !aliasSet {
		cmd.SilenceUsage = true
		return newUsageError(errCompletionPrefixWithoutAlias)
	}

	if !aliasSet {
		return generateBaseCompletion(cmd.Root(), shell, cmd.OutOrStdout())
	}

	alias := completionAlias(flags.alias)
	if err := alias.Validate(); err != nil {
		cmd.SilenceUsage = true
		return newUsageError(err)
	}

	prefixTokens, err := parseCompletionPrefix(flags.prefix)
	if err != nil {
		cmd.SilenceUsage = true
		return newUsageError(err)
	}

	var buf bytes.Buffer
	if genErr := generateBaseCompletion(cmd.Root(), shell, &buf); genErr != nil {
		return genErr
	}

	rewritten, err := rewriteCompletionScript(buf.String(), cmd.Root().Name(), shell, alias, prefixTokens)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(cmd.OutOrStdout(), rewritten); err != nil {
		return fmt.Errorf("write completion script: %w", err)
	}
	return nil
}

func generateBaseCompletion(root *cobra.Command, shell completionShell, w io.Writer) error {
	var err error
	switch shell {
	case completionShellBash:
		err = root.GenBashCompletionV2(w, true)
	case completionShellZsh:
		err = root.GenZshCompletion(w)
	case completionShellFish:
		err = root.GenFishCompletion(w, true)
	case completionShellPowerShell:
		err = root.GenPowerShellCompletionWithDesc(w)
	default:
		return newUsageError(&invalidCompletionShellError{value: shell})
	}
	if err != nil {
		return fmt.Errorf("generate %s completion: %w", shell, err)
	}
	return nil
}

// parseCompletionPrefix splits a raw --prefix string into literal tokens using
// POSIX-style quoting rules, rejecting NUL bytes, newlines, trailing escapes,
// and unclosed quotes.
//
//goplint:ignore -- parses raw --prefix flag text at the CLI boundary.
func parseCompletionPrefix(raw string) (completionPrefixTokens, error) {
	if strings.ContainsAny(raw, "\x00\n\r") {
		return nil, fmt.Errorf("%w: must not contain NUL or newline characters", errInvalidCompletionPrefix)
	}

	var (
		tokens       completionPrefixTokens
		cur          strings.Builder
		inToken      bool
		inSingle     bool
		inDouble     bool
		escaped      bool
		escapedInDbl bool
	)

	flushToken := func() {
		if inToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			inToken = false
		}
	}

	for i := range len(raw) {
		ch := raw[i]
		switch {
		case escaped:
			cur.WriteByte(ch)
			inToken = true
			escaped = false
		case escapedInDbl:
			// POSIX double-quote rules: backslash only escapes \, ", $, and `.
			// Preserving other backslashes keeps Windows paths like "C:\dir\cfg.cue" intact.
			if ch != '\\' && ch != '"' && ch != '$' && ch != '`' {
				cur.WriteByte('\\')
			}
			cur.WriteByte(ch)
			inToken = true
			escapedInDbl = false
		case inSingle:
			if ch == '\'' {
				inSingle = false
			} else {
				cur.WriteByte(ch)
			}
		case inDouble:
			switch ch {
			case '"':
				inDouble = false
			case '\\':
				escapedInDbl = true
			default:
				cur.WriteByte(ch)
			}
		default:
			switch ch {
			case ' ', '\t':
				flushToken()
			case '\'':
				inSingle = true
				inToken = true
			case '"':
				inDouble = true
				inToken = true
			case '\\':
				escaped = true
				inToken = true
			default:
				cur.WriteByte(ch)
				inToken = true
			}
		}
	}

	if escaped || escapedInDbl {
		return nil, fmt.Errorf("%w: trailing backslash escape", errInvalidCompletionPrefix)
	}
	if inSingle {
		return nil, fmt.Errorf("%w: unclosed single quote", errInvalidCompletionPrefix)
	}
	if inDouble {
		return nil, fmt.Errorf("%w: unclosed double quote", errInvalidCompletionPrefix)
	}

	flushToken()
	if err := tokens.Validate(); err != nil {
		return nil, err
	}
	return tokens, nil
}

//goplint:ignore -- rewrites Cobra template text at the CLI completion boundary.
func rewriteCompletionScript(script, rootName string, shell completionShell, alias completionAlias, prefix completionPrefixTokens) (string, error) {
	switch shell {
	case completionShellBash:
		return rewriteBashCompletion(script, rootName, alias, prefix)
	case completionShellZsh:
		return rewriteZshCompletion(script, rootName, alias, prefix)
	case completionShellFish:
		return rewriteFishCompletion(script, rootName, alias, prefix)
	case completionShellPowerShell:
		return rewritePowerShellCompletion(script, rootName, alias, prefix)
	default:
		return "", newUsageError(&invalidCompletionShellError{value: shell})
	}
}

//goplint:ignore -- rewrites Cobra Bash v2 template text at the CLI boundary.
func rewriteBashCompletion(script, rootName string, alias completionAlias, prefix completionPrefixTokens) (string, error) {
	const probeOld = `requestComp="${words[0]} __complete ${args[*]}"`
	if !strings.Contains(script, probeOld) {
		return "", fmt.Errorf("%w: bash probe anchor not found", errCompletionTemplateRewriteFail)
	}

	ident := alias.ident()
	aliasStr := alias.String()
	regOld := "complete -o default -F __start_" + rootName + " " + rootName
	if !strings.Contains(script, regOld) {
		return "", fmt.Errorf("%w: bash registration anchor not found", errCompletionTemplateRewriteFail)
	}

	var probeNew string
	if len(prefix) == 0 {
		probeNew = fmt.Sprintf(`requestComp="%s __complete ${args[*]}"`, rootName)
	} else {
		quoted := joinQuotedTokens(prefix, quotePOSIXSingle, " ")
		probeNew = fmt.Sprintf("local prefixComp\n    printf -v prefixComp '%%q ' %s\n    requestComp=\"%s __complete ${prefixComp}${args[*]}\"", quoted, rootName)
	}

	replacer := strings.NewReplacer(
		"# bash completion V2 for "+rootName, "# bash completion V2 for "+aliasStr,
		"__"+rootName+"_", "__"+ident+"_",
		"__start_"+rootName+" "+rootName, "__start_"+ident+" "+aliasStr,
		"__start_"+rootName, "__start_"+ident,
		probeOld, probeNew,
	)
	return replacer.Replace(script), nil
}

//goplint:ignore -- rewrites Cobra Zsh template text at the CLI boundary.
func rewriteZshCompletion(script, rootName string, alias completionAlias, prefix completionPrefixTokens) (string, error) {
	const probeOld = `requestComp="${words[1]} __complete ${words[2,-1]}"`
	if !strings.Contains(script, probeOld) {
		return "", fmt.Errorf("%w: zsh probe anchor not found", errCompletionTemplateRewriteFail)
	}

	ident := alias.ident()
	aliasStr := alias.String()
	compdefOld := "compdef _" + rootName + " " + rootName
	if !strings.Contains(script, compdefOld) {
		return "", fmt.Errorf("%w: zsh compdef anchor not found", errCompletionTemplateRewriteFail)
	}

	var probeNew string
	if len(prefix) == 0 {
		probeNew = fmt.Sprintf(`requestComp="%s __complete ${words[2,-1]}"`, rootName)
	} else {
		quoted := joinQuotedTokens(prefix, quotePOSIXSingle, " ")
		probeNew = fmt.Sprintf("local -a prefixArgs\n    prefixArgs=( %s )\n    requestComp=\"%s __complete ${(j: :)${(@q)prefixArgs}} ${words[2,-1]}\"", quoted, rootName)
	}

	replacer := strings.NewReplacer(
		"#compdef "+rootName+"\n", "#compdef "+aliasStr+"\n",
		compdefOld+"\n", "compdef _"+ident+" "+aliasStr+"\n",
		"# zsh completion for "+rootName, "# zsh completion for "+aliasStr,
		"__"+rootName+"_", "__"+ident+"_",
		"_"+rootName+"()", "_"+ident+"()",
		`if [ "$funcstack[1]" = "_`+rootName+`" ]; then`+"\n    _"+rootName+"\n",
		`if [ "$funcstack[1]" = "_`+ident+`" ]; then`+"\n    _"+ident+"\n",
		probeOld, probeNew,
	)
	return replacer.Replace(script), nil
}

//goplint:ignore -- rewrites Cobra Fish template text at the CLI boundary.
func rewriteFishCompletion(script, rootName string, alias completionAlias, prefix completionPrefixTokens) (string, error) {
	envVar := strings.ToUpper(strings.NewReplacer("-", "_", ":", "_").Replace(rootName)) + "_ACTIVE_HELP"
	probeOld := fmt.Sprintf(`set -l requestComp "%s=0 $args[1] __complete $args[2..-1] $lastArg"`, envVar)
	if !strings.Contains(script, probeOld) {
		return "", fmt.Errorf("%w: fish probe anchor not found", errCompletionTemplateRewriteFail)
	}

	rootVar := strings.NewReplacer("-", "_", ":", "_").Replace(rootName)
	ident := alias.ident()
	aliasStr := alias.String()
	regOld := "complete -c " + rootName + " -e"
	if !strings.Contains(script, regOld) {
		return "", fmt.Errorf("%w: fish registration anchor not found", errCompletionTemplateRewriteFail)
	}

	var probeNew string
	if len(prefix) == 0 {
		probeNew = fmt.Sprintf(`set -l requestComp "%s=0 %s __complete $args[2..-1] $lastArg"`, envVar, rootName)
	} else {
		quoted := joinQuotedTokens(prefix, quoteFishSingle, " ")
		probeNew = fmt.Sprintf("set -l prefixArgs (string escape -- %s)\n    set -l requestComp \"%s=0 %s __complete $prefixArgs $args[2..-1] $lastArg\"", quoted, envVar, rootName)
	}

	replacer := strings.NewReplacer(
		"# fish completion for "+rootName, "# fish completion for "+aliasStr,
		"__"+rootVar+"_", "__"+ident+"_",
		`if type -q "`+rootName+`"`, `if type -q "`+aliasStr+`"`,
		`complete --do-complete "`+rootName+` "`, `complete --do-complete "`+aliasStr+` "`,
		"complete -c "+rootName+" ", "complete -c "+aliasStr+" ",
		"complete -k -c "+rootName+" ", "complete -k -c "+aliasStr+" ",
		probeOld, probeNew,
	)
	return replacer.Replace(script), nil
}

//goplint:ignore -- rewrites Cobra PowerShell template text at the CLI boundary.
func rewritePowerShellCompletion(script, rootName string, alias completionAlias, prefix completionPrefixTokens) (string, error) {
	const probeOld = `$RequestComp="$Program __complete $Arguments"`
	if !strings.Contains(script, probeOld) {
		return "", fmt.Errorf("%w: powershell probe anchor not found", errCompletionTemplateRewriteFail)
	}

	rootVar := strings.NewReplacer("-", "_", ":", "_").Replace(rootName)
	ident := alias.ident()
	aliasStr := alias.String()
	regOld := "Register-ArgumentCompleter -CommandName '" + rootName + "'"
	if !strings.Contains(script, regOld) {
		return "", fmt.Errorf("%w: powershell registration anchor not found", errCompletionTemplateRewriteFail)
	}

	var probeNew string
	if len(prefix) == 0 {
		probeNew = fmt.Sprintf(`$RequestComp="%s __complete $Arguments"`, rootName)
	} else {
		quoted := joinQuotedTokens(prefix, quotePowerShellSingle, ", ")
		probeNew = fmt.Sprintf("$PrefixComp = @(%s) | ForEach-Object { \"'\" + ($_ -replace \"'\", \"''\") + \"'\" }\n    $RequestComp=\"%s __complete $($PrefixComp -join ' ') $Arguments\"", quoted, rootName)
	}

	replacer := strings.NewReplacer(
		"# powershell completion for "+rootName, "# powershell completion for "+aliasStr,
		"__"+rootName+"_", "__"+ident+"_",
		"${__"+rootVar+"CompleterBlock}", "${__"+ident+"CompleterBlock}",
		regOld, "Register-ArgumentCompleter -CommandName '"+aliasStr+"'",
		probeOld, probeNew,
	)
	return replacer.Replace(script), nil
}

//goplint:ignore -- pure shell-token quoting helper for Cobra completion rewriting.
func joinQuotedTokens(tokens completionPrefixTokens, quoteFn func(string) string, sep string) string {
	quoted := make([]string, len(tokens))
	for i, tok := range tokens {
		quoted[i] = quoteFn(tok)
	}
	return strings.Join(quoted, sep)
}

//goplint:ignore -- pure shell-token quoting helper for Cobra completion rewriting.
func quotePOSIXSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

//goplint:ignore -- pure shell-token quoting helper for Cobra completion rewriting.
func quoteFishSingle(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, "'", `\'`)
	return "'" + escaped + "'"
}

//goplint:ignore -- pure shell-token quoting helper for Cobra completion rewriting.
func quotePowerShellSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
