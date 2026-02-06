// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"strings"
)

// GenerateCUE generates CUE text from an Invkfile struct.
// This is useful for creating invkfile.cue files programmatically.
func GenerateCUE(inv *Invkfile) string {
	var sb strings.Builder

	sb.WriteString("// Invkfile - Command definitions for invowk\n")
	sb.WriteString("// See https://github.com/invowk/invowk for documentation\n\n")

	if inv.DefaultShell != "" {
		fmt.Fprintf(&sb, "default_shell: %q\n", inv.DefaultShell)
	}
	if inv.WorkDir != "" {
		fmt.Fprintf(&sb, "workdir: %q\n", inv.WorkDir)
	}

	// Root-level env
	if inv.Env != nil && (len(inv.Env.Files) > 0 || len(inv.Env.Vars) > 0) {
		sb.WriteString("env: {\n")
		if len(inv.Env.Files) > 0 {
			sb.WriteString("\tfiles: [")
			for i, ef := range inv.Env.Files {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(&sb, "%q", ef)
			}
			sb.WriteString("]\n")
		}
		if len(inv.Env.Vars) > 0 {
			sb.WriteString("\tvars: {\n")
			for k, v := range inv.Env.Vars {
				fmt.Fprintf(&sb, "\t\t%s: %q\n", k, v)
			}
			sb.WriteString("\t}\n")
		}
		sb.WriteString("}\n")
	}

	// Root-level depends_on
	generateDependsOn(&sb, inv.DependsOn, "\t")

	// Commands
	sb.WriteString("\ncmds: [\n")
	for i := range inv.Commands {
		generateCommand(&sb, &inv.Commands[i])
	}
	sb.WriteString("]\n")

	return sb.String()
}

// generateDependsOn generates CUE for a DependsOn block
func generateDependsOn(sb *strings.Builder, deps *DependsOn, indent string) {
	if deps == nil {
		return
	}
	if len(deps.Tools) == 0 && len(deps.Commands) == 0 && len(deps.Filepaths) == 0 &&
		len(deps.Capabilities) == 0 && len(deps.CustomChecks) == 0 && len(deps.EnvVars) == 0 {
		return
	}

	baseIndent := strings.TrimSuffix(indent, "\t")
	sb.WriteString(baseIndent + "depends_on: {\n")

	if len(deps.Tools) > 0 {
		sb.WriteString(indent + "tools: [\n")
		for _, tool := range deps.Tools {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range tool.Alternatives {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", alt)
			}
			sb.WriteString("]},\n")
		}
		sb.WriteString(indent + "]\n")
	}

	if len(deps.Commands) > 0 {
		sb.WriteString(indent + "cmds: [\n")
		for _, dep := range deps.Commands {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range dep.Alternatives {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", alt)
			}
			sb.WriteString("]},\n")
		}
		sb.WriteString(indent + "]\n")
	}

	if len(deps.Filepaths) > 0 {
		sb.WriteString(indent + "filepaths: [\n")
		for _, fp := range deps.Filepaths {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range fp.Alternatives {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", alt)
			}
			sb.WriteString("]")
			if fp.Readable {
				sb.WriteString(", readable: true")
			}
			if fp.Writable {
				sb.WriteString(", writable: true")
			}
			if fp.Executable {
				sb.WriteString(", executable: true")
			}
			sb.WriteString("},\n")
		}
		sb.WriteString(indent + "]\n")
	}

	if len(deps.Capabilities) > 0 {
		sb.WriteString(indent + "capabilities: [\n")
		for _, cap := range deps.Capabilities {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range cap.Alternatives {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", alt)
			}
			sb.WriteString("]},\n")
		}
		sb.WriteString(indent + "]\n")
	}

	if len(deps.CustomChecks) > 0 {
		sb.WriteString(indent + "custom_checks: [\n")
		for _, check := range deps.CustomChecks {
			if check.IsAlternatives() {
				sb.WriteString(indent + "\t{alternatives: [\n")
				for _, alt := range check.Alternatives {
					sb.WriteString(indent + "\t\t{")
					fmt.Fprintf(sb, "name: %q, check_script: %q", alt.Name, alt.CheckScript)
					if alt.ExpectedCode != nil {
						fmt.Fprintf(sb, ", expected_code: %d", *alt.ExpectedCode)
					}
					if alt.ExpectedOutput != "" {
						fmt.Fprintf(sb, ", expected_output: %q", alt.ExpectedOutput)
					}
					sb.WriteString("},\n")
				}
				sb.WriteString(indent + "\t]},\n")
			} else {
				sb.WriteString(indent + "\t{")
				fmt.Fprintf(sb, "name: %q, check_script: %q", check.Name, check.CheckScript)
				if check.ExpectedCode != nil {
					fmt.Fprintf(sb, ", expected_code: %d", *check.ExpectedCode)
				}
				if check.ExpectedOutput != "" {
					fmt.Fprintf(sb, ", expected_output: %q", check.ExpectedOutput)
				}
				sb.WriteString("},\n")
			}
		}
		sb.WriteString(indent + "]\n")
	}

	if len(deps.EnvVars) > 0 {
		sb.WriteString(indent + "env_vars: [\n")
		for _, envVar := range deps.EnvVars {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range envVar.Alternatives {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString("{")
				fmt.Fprintf(sb, "name: %q", alt.Name)
				if alt.Validation != "" {
					fmt.Fprintf(sb, ", validation: %q", alt.Validation)
				}
				sb.WriteString("}")
			}
			sb.WriteString("]},\n")
		}
		sb.WriteString(indent + "]\n")
	}

	sb.WriteString(baseIndent + "}\n")
}

// generateCommand generates CUE for a single command
func generateCommand(sb *strings.Builder, cmd *Command) {
	sb.WriteString("\t{\n")
	fmt.Fprintf(sb, "\t\tname: %q\n", cmd.Name)
	if cmd.Description != "" {
		fmt.Fprintf(sb, "\t\tdescription: %q\n", cmd.Description)
	}

	// Generate implementations list
	sb.WriteString("\t\timplementations: [\n")
	for _, impl := range cmd.Implementations {
		generateImplementation(sb, &impl)
	}
	sb.WriteString("\t\t]\n")

	// Command-level env
	if cmd.Env != nil && (len(cmd.Env.Files) > 0 || len(cmd.Env.Vars) > 0) {
		sb.WriteString("\t\tenv: {\n")
		if len(cmd.Env.Files) > 0 {
			sb.WriteString("\t\t\tfiles: [")
			for i, ef := range cmd.Env.Files {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", ef)
			}
			sb.WriteString("]\n")
		}
		if len(cmd.Env.Vars) > 0 {
			sb.WriteString("\t\t\tvars: {\n")
			for k, v := range cmd.Env.Vars {
				fmt.Fprintf(sb, "\t\t\t\t%s: %q\n", k, v)
			}
			sb.WriteString("\t\t\t}\n")
		}
		sb.WriteString("\t\t}\n")
	}

	if cmd.WorkDir != "" {
		fmt.Fprintf(sb, "\t\tworkdir: %q\n", cmd.WorkDir)
	}

	// Command-level depends_on
	generateCommandDependsOn(sb, cmd.DependsOn)

	// Generate flags list
	if len(cmd.Flags) > 0 {
		sb.WriteString("\t\tflags: [\n")
		for _, flag := range cmd.Flags {
			sb.WriteString("\t\t\t{")
			fmt.Fprintf(sb, "name: %q, description: %q", flag.Name, flag.Description)
			if flag.DefaultValue != "" {
				fmt.Fprintf(sb, ", default_value: %q", flag.DefaultValue)
			}
			sb.WriteString("},\n")
		}
		sb.WriteString("\t\t]\n")
	}

	// Generate args list
	if len(cmd.Args) > 0 {
		sb.WriteString("\t\targs: [\n")
		for _, arg := range cmd.Args {
			sb.WriteString("\t\t\t{")
			fmt.Fprintf(sb, "name: %q, description: %q", arg.Name, arg.Description)
			if arg.Required {
				sb.WriteString(", required: true")
			}
			if arg.DefaultValue != "" {
				fmt.Fprintf(sb, ", default_value: %q", arg.DefaultValue)
			}
			if arg.Type != "" && arg.Type != ArgumentTypeString {
				fmt.Fprintf(sb, ", type: %q", arg.Type)
			}
			if arg.Validation != "" {
				fmt.Fprintf(sb, ", validation: %q", arg.Validation)
			}
			if arg.Variadic {
				sb.WriteString(", variadic: true")
			}
			sb.WriteString("},\n")
		}
		sb.WriteString("\t\t]\n")
	}

	sb.WriteString("\t},\n")
}

// generateImplementation generates CUE for a single implementation
func generateImplementation(sb *strings.Builder, impl *Implementation) {
	sb.WriteString("\t\t\t{\n")

	// Handle multi-line scripts with CUE's multi-line string syntax
	if strings.Contains(impl.Script, "\n") {
		sb.WriteString("\t\t\t\tscript: \"\"\"\n")
		for line := range strings.SplitSeq(impl.Script, "\n") {
			fmt.Fprintf(sb, "\t\t\t\t\t%s\n", line)
		}
		sb.WriteString("\t\t\t\t\t\"\"\"\n")
	} else {
		fmt.Fprintf(sb, "\t\t\t\tscript: %q\n", impl.Script)
	}

	// Runtimes
	sb.WriteString("\t\t\t\truntimes: [\n")
	for i := range impl.Runtimes {
		r := &impl.Runtimes[i]
		sb.WriteString("\t\t\t\t\t{")
		fmt.Fprintf(sb, "name: %q", r.Name)
		if r.Name == RuntimeContainer {
			if r.EnableHostSSH {
				sb.WriteString(", enable_host_ssh: true")
			}
			if r.Containerfile != "" {
				fmt.Fprintf(sb, ", containerfile: %q", r.Containerfile)
			}
			if r.Image != "" {
				fmt.Fprintf(sb, ", image: %q", r.Image)
			}
			if len(r.Volumes) > 0 {
				sb.WriteString(", volumes: [")
				for j, v := range r.Volumes {
					if j > 0 {
						sb.WriteString(", ")
					}
					fmt.Fprintf(sb, "%q", v)
				}
				sb.WriteString("]")
			}
			if len(r.Ports) > 0 {
				sb.WriteString(", ports: [")
				for j, p := range r.Ports {
					if j > 0 {
						sb.WriteString(", ")
					}
					fmt.Fprintf(sb, "%q", p)
				}
				sb.WriteString("]")
			}
		}
		sb.WriteString("},\n")
	}
	sb.WriteString("\t\t\t\t]\n")

	// Platforms (mandatory)
	sb.WriteString("\t\t\t\tplatforms: [\n")
	for _, p := range impl.Platforms {
		fmt.Fprintf(sb, "\t\t\t\t\t{name: %q},\n", p.Name)
	}
	sb.WriteString("\t\t\t\t]\n")

	// Implementation-level depends_on
	generateImplDependsOn(sb, impl.DependsOn)

	// Implementation-level env
	if impl.Env != nil && (len(impl.Env.Files) > 0 || len(impl.Env.Vars) > 0) {
		sb.WriteString("\t\t\t\tenv: {\n")
		if len(impl.Env.Files) > 0 {
			sb.WriteString("\t\t\t\t\tfiles: [")
			for i, ef := range impl.Env.Files {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", ef)
			}
			sb.WriteString("]\n")
		}
		if len(impl.Env.Vars) > 0 {
			sb.WriteString("\t\t\t\t\tvars: {\n")
			for k, v := range impl.Env.Vars {
				fmt.Fprintf(sb, "\t\t\t\t\t\t%s: %q\n", k, v)
			}
			sb.WriteString("\t\t\t\t\t}\n")
		}
		sb.WriteString("\t\t\t\t}\n")
	}

	// Implementation-level workdir
	if impl.WorkDir != "" {
		fmt.Fprintf(sb, "\t\t\t\tworkdir: %q\n", impl.WorkDir)
	}

	sb.WriteString("\t\t\t},\n")
}

// generateImplDependsOn generates CUE for implementation-level depends_on
func generateImplDependsOn(sb *strings.Builder, deps *DependsOn) {
	if deps == nil {
		return
	}
	if len(deps.Tools) == 0 && len(deps.Commands) == 0 && len(deps.Filepaths) == 0 &&
		len(deps.Capabilities) == 0 && len(deps.CustomChecks) == 0 && len(deps.EnvVars) == 0 {
		return
	}

	sb.WriteString("\t\t\t\tdepends_on: {\n")
	generateDependsOnContent(sb, deps, "\t\t\t\t\t")
	sb.WriteString("\t\t\t\t}\n")
}

// generateCommandDependsOn generates CUE for command-level depends_on
func generateCommandDependsOn(sb *strings.Builder, deps *DependsOn) {
	if deps == nil {
		return
	}
	if len(deps.Tools) == 0 && len(deps.Commands) == 0 && len(deps.Filepaths) == 0 &&
		len(deps.Capabilities) == 0 && len(deps.CustomChecks) == 0 && len(deps.EnvVars) == 0 {
		return
	}

	sb.WriteString("\t\tdepends_on: {\n")
	generateDependsOnContent(sb, deps, "\t\t\t")
	sb.WriteString("\t\t}\n")
}

// generateDependsOnContent generates the content of a depends_on block
func generateDependsOnContent(sb *strings.Builder, deps *DependsOn, indent string) {
	if len(deps.Tools) > 0 {
		sb.WriteString(indent + "tools: [\n")
		for _, tool := range deps.Tools {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range tool.Alternatives {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", alt)
			}
			sb.WriteString("]},\n")
		}
		sb.WriteString(indent + "]\n")
	}

	if len(deps.Commands) > 0 {
		sb.WriteString(indent + "cmds: [\n")
		for _, dep := range deps.Commands {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range dep.Alternatives {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", alt)
			}
			sb.WriteString("]},\n")
		}
		sb.WriteString(indent + "]\n")
	}

	if len(deps.Filepaths) > 0 {
		sb.WriteString(indent + "filepaths: [\n")
		for _, fp := range deps.Filepaths {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range fp.Alternatives {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", alt)
			}
			sb.WriteString("]")
			if fp.Readable {
				sb.WriteString(", readable: true")
			}
			if fp.Writable {
				sb.WriteString(", writable: true")
			}
			if fp.Executable {
				sb.WriteString(", executable: true")
			}
			sb.WriteString("},\n")
		}
		sb.WriteString(indent + "]\n")
	}

	if len(deps.Capabilities) > 0 {
		sb.WriteString(indent + "capabilities: [\n")
		for _, cap := range deps.Capabilities {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range cap.Alternatives {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", alt)
			}
			sb.WriteString("]},\n")
		}
		sb.WriteString(indent + "]\n")
	}

	if len(deps.CustomChecks) > 0 {
		sb.WriteString(indent + "custom_checks: [\n")
		for _, check := range deps.CustomChecks {
			if check.IsAlternatives() {
				sb.WriteString(indent + "\t{alternatives: [\n")
				for _, alt := range check.Alternatives {
					sb.WriteString(indent + "\t\t{")
					fmt.Fprintf(sb, "name: %q, check_script: %q", alt.Name, alt.CheckScript)
					if alt.ExpectedCode != nil {
						fmt.Fprintf(sb, ", expected_code: %d", *alt.ExpectedCode)
					}
					if alt.ExpectedOutput != "" {
						fmt.Fprintf(sb, ", expected_output: %q", alt.ExpectedOutput)
					}
					sb.WriteString("},\n")
				}
				sb.WriteString(indent + "\t]},\n")
			} else {
				sb.WriteString(indent + "\t{")
				fmt.Fprintf(sb, "name: %q, check_script: %q", check.Name, check.CheckScript)
				if check.ExpectedCode != nil {
					fmt.Fprintf(sb, ", expected_code: %d", *check.ExpectedCode)
				}
				if check.ExpectedOutput != "" {
					fmt.Fprintf(sb, ", expected_output: %q", check.ExpectedOutput)
				}
				sb.WriteString("},\n")
			}
		}
		sb.WriteString(indent + "]\n")
	}

	if len(deps.EnvVars) > 0 {
		sb.WriteString(indent + "env_vars: [\n")
		for _, envVar := range deps.EnvVars {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range envVar.Alternatives {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString("{")
				fmt.Fprintf(sb, "name: %q", alt.Name)
				if alt.Validation != "" {
					fmt.Fprintf(sb, ", validation: %q", alt.Validation)
				}
				sb.WriteString("}")
			}
			sb.WriteString("]},\n")
		}
		sb.WriteString(indent + "]\n")
	}
}
