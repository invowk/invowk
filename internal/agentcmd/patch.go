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
			return fmt.Errorf("command %q already exists; use `invowk agent cmd change %s` to modify it", name, name)
		}
		list.Elts[i] = command
		return nil
	}

	list.Elts = append(list.Elts, command)
	return nil
}

// FindCommandCUE returns the formatted command object for name when it exists.
func FindCommandCUE(existing, targetPath string, name invowkfile.CommandName) (commandCUE string, found bool, err error) {
	if strings.TrimSpace(existing) == "" {
		return "", false, nil
	}

	file, err := parser.ParseFile(targetPath, existing, parser.ParseComments)
	if err != nil {
		return "", false, fmt.Errorf("parse %s: %w", targetPath, err)
	}

	list, found, err := commandList(file)
	if err != nil || !found {
		return "", false, err
	}
	for i := range list.Elts {
		existingName, ok := commandNameFromExpr(list.Elts[i])
		if !ok || existingName != name {
			continue
		}
		data, formatErr := format.Node(list.Elts[i])
		if formatErr != nil {
			return "", false, fmt.Errorf("format command %q: %w", name, formatErr)
		}
		return strings.TrimSpace(string(data)), true, nil
	}
	return "", false, nil
}

// RemoveCommandFromInvowkfile removes name from existing invowkfile content.
func RemoveCommandFromInvowkfile(existing string, name invowkfile.CommandName, targetPath string) (removedCommand, content string, err error) {
	if strings.TrimSpace(existing) == "" {
		return "", "", fmt.Errorf("command %q does not exist in %s", name, targetPath)
	}

	file, err := parser.ParseFile(targetPath, existing, parser.ParseComments)
	if err != nil {
		return "", "", fmt.Errorf("parse %s: %w", targetPath, err)
	}

	list, found, err := commandList(file)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", fmt.Errorf("command %q does not exist in %s", name, targetPath)
	}

	for i := range list.Elts {
		existingName, ok := commandNameFromExpr(list.Elts[i])
		if !ok || existingName != name {
			continue
		}
		return removeCommandAt(file, list, i, name, targetPath)
	}

	return "", "", fmt.Errorf("command %q does not exist in %s", name, targetPath)
}

func removeCommandAt(
	file *ast.File,
	list *ast.ListLit,
	index int,
	name invowkfile.CommandName,
	targetPath string,
) (removedCommand, content string, err error) {
	data, formatErr := format.Node(list.Elts[index])
	if formatErr != nil {
		return "", "", fmt.Errorf("format command %q: %w", name, formatErr)
	}
	removed := strings.TrimSpace(string(data))
	list.Elts = append(list.Elts[:index], list.Elts[index+1:]...)
	if len(list.Elts) == 0 {
		if hasNonCommandFields(file) {
			return "", "", fmt.Errorf("cannot remove command %q because %s would have no commands but still contains other settings", name, targetPath)
		}
		return removed, "", nil
	}

	formatted, formatErr := format.Node(file)
	if formatErr != nil {
		return "", "", fmt.Errorf("format patched invowkfile: %w", formatErr)
	}
	content = string(formatted)
	if validateErr := validatePatchedContent(content, targetPath); validateErr != nil {
		return "", "", validateErr
	}
	return removed, content, nil
}

func hasNonCommandFields(file *ast.File) bool {
	for i := range file.Decls {
		field, ok := file.Decls[i].(*ast.Field)
		if !ok {
			continue
		}
		label, _, err := ast.LabelName(field.Label)
		if err != nil || label == cmdsFieldName {
			continue
		}
		return true
	}
	return false
}

func commandList(file *ast.File) (*ast.ListLit, bool, error) {
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
			return nil, true, fmt.Errorf("%s must be a list", cmdsFieldName)
		}
		return list, true, nil
	}
	return nil, false, nil
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
