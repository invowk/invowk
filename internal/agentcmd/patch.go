// SPDX-License-Identifier: MPL-2.0

package agentcmd

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"

	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	cmdsFieldName = "cmds"
	diffOldPrefix = "--- "
	diffNewPrefix = "+++ "
)

// PatchInvowkfile returns the target invowkfile content with commandCUE inserted.
func PatchInvowkfile(existing string, exists bool, commandCUE string, name invowkfile.CommandName, replace bool, targetPath string) (string, error) {
	if !exists || strings.TrimSpace(existing) == "" {
		content := wrapCommandObject(commandCUE)
		if err := validatePatchedContent(content, targetPath); err != nil {
			return "", err
		}
		return content, nil
	}

	file, err := parser.ParseFile(targetPath, existing, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", targetPath, err)
	}

	expr, err := parser.ParseExpr(generatedCommandPath, commandCUE)
	if err != nil {
		return "", fmt.Errorf("parse generated command: %w", err)
	}

	if insertErr := insertCommand(file, expr, name, replace); insertErr != nil {
		return "", insertErr
	}

	formatted, err := format.Node(file)
	if err != nil {
		return "", fmt.Errorf("format patched invowkfile: %w", err)
	}
	content := string(formatted)
	if err := validatePatchedContent(content, targetPath); err != nil {
		return "", err
	}
	return content, nil
}

// BuildUnifiedDiff creates a compact file-level diff for dry-run output.
func BuildUnifiedDiff(path, oldContent, newContent string, existed bool) string {
	oldPath := path
	if !existed {
		oldPath = "/dev/null"
	}

	var b strings.Builder
	b.WriteString(diffOldPrefix)
	b.WriteString(oldPath)
	b.WriteByte('\n')
	b.WriteString(diffNewPrefix)
	b.WriteString(path)
	b.WriteByte('\n')

	for _, line := range splitDiffLines(oldContent) {
		b.WriteByte('-')
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, line := range splitDiffLines(newContent) {
		b.WriteByte('+')
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func insertCommand(file *ast.File, command ast.Expr, name invowkfile.CommandName, replace bool) error {
	for i := range file.Decls {
		field, ok := file.Decls[i].(*ast.Field)
		if !ok {
			continue
		}
		label, _, err := ast.LabelName(field.Label)
		if err != nil || label != cmdsFieldName {
			continue
		}
		list, ok := field.Value.(*ast.ListLit)
		if !ok {
			return fmt.Errorf("%s must be a list", cmdsFieldName)
		}
		return insertIntoCommandList(list, command, name, replace)
	}

	file.Decls = append(file.Decls, &ast.Field{
		Label: ast.NewIdent(cmdsFieldName),
		Value: ast.NewList(command),
	})
	return nil
}

func insertIntoCommandList(list *ast.ListLit, command ast.Expr, name invowkfile.CommandName, replace bool) error {
	for i := range list.Elts {
		existingName, ok := commandNameFromExpr(list.Elts[i])
		if !ok || existingName != name {
			continue
		}
		if !replace {
			return fmt.Errorf("command %q already exists; use --replace to overwrite it", name)
		}
		list.Elts[i] = command
		return nil
	}

	list.Elts = append(list.Elts, command)
	return nil
}

func commandNameFromExpr(expr ast.Expr) (invowkfile.CommandName, bool) {
	data, err := format.Node(expr)
	if err != nil {
		return "", false
	}
	inv, err := invowkfile.ParseBytes([]byte(wrapCommandObject(string(data))), generatedCommandPath)
	if err != nil || len(inv.Commands) != 1 {
		return "", false
	}
	return inv.Commands[0].Name, true
}

func formatCommandObject(commandCUE string) (string, error) {
	expr, err := parser.ParseExpr(generatedCommandPath, commandCUE)
	if err != nil {
		return "", fmt.Errorf("parse generated command: %w", err)
	}
	formatted, err := format.Node(expr)
	if err != nil {
		return "", fmt.Errorf("format generated command: %w", err)
	}
	return strings.TrimSpace(string(formatted)), nil
}

func wrapCommandObject(commandCUE string) string {
	commandCUE = strings.TrimSpace(commandCUE)
	return "cmds: [\n" + commandCUE + "\n]\n"
}

func validatePatchedContent(content, targetPath string) error {
	if _, err := invowkfile.ParseBytes([]byte(content), targetPath); err != nil {
		return fmt.Errorf("validate patched invowkfile: %w", err)
	}
	return nil
}

func splitDiffLines(content string) []string {
	if content == "" {
		return nil
	}
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return []string{""}
	}
	return strings.Split(content, "\n")
}
