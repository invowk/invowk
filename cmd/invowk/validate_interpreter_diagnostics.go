// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	customCheckInterpreterTargetHost customCheckInterpreterTarget = iota
	customCheckInterpreterTargetRuntime
)

type (
	//goplint:constant-only
	//
	// customCheckInterpreterTarget identifies host vs runtime custom-check analysis.
	customCheckInterpreterTarget int
)

func (t customCheckInterpreterTarget) String() string {
	switch t {
	case customCheckInterpreterTargetHost:
		return "host"
	case customCheckInterpreterTargetRuntime:
		return "runtime"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

func (t customCheckInterpreterTarget) Validate() error {
	switch t {
	case customCheckInterpreterTargetHost, customCheckInterpreterTargetRuntime:
		return nil
	default:
		return fmt.Errorf("invalid custom check interpreter target %s", t)
	}
}

func collectCommandSetInterpreterDiagnostics(set *discovery.DiscoveredCommandSet) []invowkfile.ScriptInterpreterDiagnostic {
	if set == nil {
		return nil
	}
	seen := make(map[*invowkfile.Invowkfile]bool)
	var diagnostics []invowkfile.ScriptInterpreterDiagnostic
	for _, cmdInfo := range set.Commands {
		if cmdInfo == nil || cmdInfo.Invowkfile == nil || seen[cmdInfo.Invowkfile] {
			continue
		}
		seen[cmdInfo.Invowkfile] = true
		diagnostics = append(diagnostics, collectInvowkfileInterpreterDiagnostics(cmdInfo.Invowkfile, os.ReadFile)...)
	}
	return diagnostics
}

func collectInvowkfileInterpreterDiagnostics(inv *invowkfile.Invowkfile, readFile func(path string) ([]byte, error)) []invowkfile.ScriptInterpreterDiagnostic {
	if inv == nil {
		return nil
	}
	var diagnostics []invowkfile.ScriptInterpreterDiagnostic
	diagnostics = appendDependsOnInterpreterDiagnostics(diagnostics, inv.DependsOn, inv, readFile, customCheckInterpreterTargetHost, "")
	for _, command := range inv.FlattenCommands() {
		diagnostics = appendDependsOnInterpreterDiagnostics(diagnostics, command.DependsOn, inv, readFile, customCheckInterpreterTargetHost, "")
		for i := range command.Implementations {
			impl := &command.Implementations[i]
			scriptText, err := impl.ResolveScriptWithFSAndModule(inv.FilePath, inv.ModulePath, readFile)
			if err == nil {
				content := invowkfile.ScriptContent(scriptText) //goplint:ignore -- ResolveScriptWithFSAndModule already validated the resolved script content.
				analysis := impl.Script.AnalyzeInterpreter(content, firstImplementationRuntime(impl))
				diagnostics = append(diagnostics, analysis.Diagnostics()...)
			}
			diagnostics = appendDependsOnInterpreterDiagnostics(diagnostics, impl.DependsOn, inv, readFile, customCheckInterpreterTargetHost, "")
			for runtimeIndex := range impl.Runtimes {
				runtimeConfig := &impl.Runtimes[runtimeIndex]
				diagnostics = appendDependsOnInterpreterDiagnostics(
					diagnostics,
					runtimeConfig.DependsOn,
					inv,
					readFile,
					customCheckInterpreterTargetRuntime,
					runtimeConfig.Name,
				)
			}
		}
	}
	return diagnostics
}

func appendDependsOnInterpreterDiagnostics(
	diagnostics []invowkfile.ScriptInterpreterDiagnostic,
	dependsOn *invowkfile.DependsOn,
	inv *invowkfile.Invowkfile,
	readFile func(path string) ([]byte, error),
	target customCheckInterpreterTarget,
	runtime invowkfile.RuntimeMode,
) []invowkfile.ScriptInterpreterDiagnostic {
	if dependsOn == nil {
		return diagnostics
	}
	for _, customCheckDep := range dependsOn.CustomChecks {
		for _, check := range customCheckDep.GetChecks() {
			resolved, err := check.Script.ResolveWithFSAndModule(inv.ModulePath, readFile)
			if err != nil {
				continue
			}
			analysisRuntime := runtime
			if target == customCheckInterpreterTargetHost {
				analysisRuntime = hostCustomCheckInterpreterRuntime(check.Script, resolved)
			}
			label := invowkfile.ScriptInterpreterSourceLabel(fmt.Sprintf("custom check %q", check.Name))
			analysis := check.Script.AnalyzeInterpreter(resolved, analysisRuntime, label)
			diagnostics = append(diagnostics, analysis.Diagnostics()...)
		}
	}
	return diagnostics
}

func firstImplementationRuntime(impl *invowkfile.Implementation) invowkfile.RuntimeMode {
	if impl == nil || len(impl.Runtimes) == 0 {
		return ""
	}
	return impl.Runtimes[0].Name
}

func hostCustomCheckInterpreterRuntime(script invowkfile.CustomCheckScript, scriptText invowkfile.ScriptContent) invowkfile.RuntimeMode {
	interpInfo := script.ResolveInterpreterFromScript(scriptText.String())
	if interpInfo.Found && !invowkfile.IsShellInterpreter(interpInfo.Interpreter) {
		return invowkfile.RuntimeNative
	}
	return invowkfile.RuntimeVirtual
}

func renderValidationInterpreterDiagnostics(stderr io.Writer, diagnostics []invowkfile.ScriptInterpreterDiagnostic) {
	if len(diagnostics) == 0 {
		return
	}
	fmt.Fprintln(stderr)
	fmt.Fprintf(stderr, "%s %d advisory warning(s) found:\n", WarningStyle.Render("!"), len(diagnostics))
	fmt.Fprintln(stderr)
	for i := range diagnostics {
		diagnostic := diagnostics[i]
		issueNum := fmt.Sprintf("  %d.", i+1)
		codeTag := moduleIssueTypeStyle.Render(fmt.Sprintf("[%s]", diagnostic.Code()))
		if diagnostic.Path() != "" {
			fmt.Fprintf(stderr, validateIssueLineFmt, issueNum, codeTag, modulePathStyle.Render(diagnostic.Path().String()))
			fmt.Fprintf(stderr, "     %s\n", diagnostic.Message())
			continue
		}
		fmt.Fprintf(stderr, validateIssueLineFmt, issueNum, codeTag, diagnostic.Message())
	}
}
