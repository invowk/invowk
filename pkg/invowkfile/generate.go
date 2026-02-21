// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// GenerateCUE generates CUE text from an Invowkfile struct.
// This is useful for creating invowkfile.cue files programmatically.
func GenerateCUE(inv *Invowkfile) string {
	var sb strings.Builder

	sb.WriteString("// Invowkfile - Command definitions for invowk\n")
	sb.WriteString("// See https://github.com/invowk/invowk for documentation\n\n")

	if inv.DefaultShell != "" {
		fmt.Fprintf(&sb, "default_shell: %q\n", inv.DefaultShell)
	}
	if inv.WorkDir != "" {
		fmt.Fprintf(&sb, "workdir: %q\n", inv.WorkDir)
	}

	// Root-level env
	generateEnvBlock(&sb, inv.Env, "")

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

// generateEnvBlock generates a CUE env: {...} block at the given indentation.
// No-op when env is nil or has no files/vars.
func generateEnvBlock(sb *strings.Builder, env *EnvConfig, indent string) {
	if env == nil || (len(env.Files) == 0 && len(env.Vars) == 0) {
		return
	}
	sb.WriteString(indent + "env: {\n")
	if len(env.Files) > 0 {
		sb.WriteString(indent + "\tfiles: [")
		for i, ef := range env.Files {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%q", ef)
		}
		sb.WriteString("]\n")
	}
	if len(env.Vars) > 0 {
		sb.WriteString(indent + "\tvars: {\n")
		for _, k := range slices.Sorted(maps.Keys(env.Vars)) {
			fmt.Fprintf(sb, "%s\t\t%s: %q\n", indent, k, env.Vars[k])
		}
		sb.WriteString(indent + "\t}\n")
	}
	sb.WriteString(indent + "}\n")
}

// generateDependsOn generates CUE for a DependsOn block at any nesting level.
// The indent parameter controls the indentation depth for the block's fields.
func generateDependsOn(sb *strings.Builder, deps *DependsOn, indent string) {
	if deps == nil {
		return
	}
	if deps.IsEmpty() {
		return
	}

	baseIndent := strings.TrimSuffix(indent, "\t")
	sb.WriteString(baseIndent + "depends_on: {\n")
	generateDependsOnContent(sb, deps, indent)
	sb.WriteString(baseIndent + "}\n")
}

// generateCommand generates CUE for a single command
func generateCommand(sb *strings.Builder, cmd *Command) {
	sb.WriteString("\t{\n")
	fmt.Fprintf(sb, "\t\tname: %q\n", cmd.Name)
	if cmd.Description != "" {
		fmt.Fprintf(sb, "\t\tdescription: %q\n", cmd.Description)
	}
	if cmd.Category != "" {
		fmt.Fprintf(sb, "\t\tcategory: %q\n", cmd.Category)
	}

	// Generate implementations list
	sb.WriteString("\t\timplementations: [\n")
	for i := range cmd.Implementations {
		generateImplementation(sb, &cmd.Implementations[i])
	}
	sb.WriteString("\t\t]\n")

	// Command-level env
	generateEnvBlock(sb, cmd.Env, "\t\t")

	if cmd.WorkDir != "" {
		fmt.Fprintf(sb, "\t\tworkdir: %q\n", cmd.WorkDir)
	}

	// Command-level depends_on
	generateDependsOn(sb, cmd.DependsOn, "\t\t\t")

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

	// Generate watch config
	if cmd.Watch != nil && len(cmd.Watch.Patterns) > 0 {
		sb.WriteString("\t\twatch: {\n")
		sb.WriteString("\t\t\tpatterns: [")
		for i, p := range cmd.Watch.Patterns {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%q", p)
		}
		sb.WriteString("]\n")
		if cmd.Watch.Debounce != "" {
			fmt.Fprintf(sb, "\t\t\tdebounce: %q\n", cmd.Watch.Debounce)
		}
		if cmd.Watch.ClearScreen {
			sb.WriteString("\t\t\tclear_screen: true\n")
		}
		if len(cmd.Watch.Ignore) > 0 {
			sb.WriteString("\t\t\tignore: [")
			for i, ig := range cmd.Watch.Ignore {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%q", ig)
			}
			sb.WriteString("]\n")
		}
		sb.WriteString("\t\t}\n")
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
		generateRuntimeConfig(sb, r)
	}
	sb.WriteString("\t\t\t\t]\n")

	// Platforms (mandatory)
	sb.WriteString("\t\t\t\tplatforms: [\n")
	for _, p := range impl.Platforms {
		fmt.Fprintf(sb, "\t\t\t\t\t{name: %q},\n", p.Name)
	}
	sb.WriteString("\t\t\t\t]\n")

	// Implementation-level depends_on
	generateDependsOn(sb, impl.DependsOn, "\t\t\t\t\t")

	// Implementation-level env
	generateEnvBlock(sb, impl.Env, "\t\t\t\t")

	// Implementation-level workdir
	if impl.WorkDir != "" {
		fmt.Fprintf(sb, "\t\t\t\tworkdir: %q\n", impl.WorkDir)
	}

	// Implementation-level timeout
	if impl.Timeout != "" {
		fmt.Fprintf(sb, "\t\t\t\ttimeout: %q\n", impl.Timeout)
	}

	sb.WriteString("\t\t\t},\n")
}

// generateRuntimeConfig generates CUE for a single runtime config entry.
// Uses compact single-line format when there are no depends_on, and multi-line
// format when depends_on is present (since it requires nested block generation).
func generateRuntimeConfig(sb *strings.Builder, r *RuntimeConfig) {
	hasDeps := r.Name == RuntimeContainer && r.DependsOn != nil && (len(r.DependsOn.Tools) > 0 || len(r.DependsOn.Commands) > 0 ||
		len(r.DependsOn.Filepaths) > 0 || len(r.DependsOn.Capabilities) > 0 ||
		len(r.DependsOn.CustomChecks) > 0 || len(r.DependsOn.EnvVars) > 0)

	if hasDeps {
		// Multi-line format for runtimes with depends_on
		sb.WriteString("\t\t\t\t\t{\n")
		fmt.Fprintf(sb, "\t\t\t\t\t\tname: %q\n", r.Name)
		generateRuntimeConfigFields(sb, r, "\t\t\t\t\t\t", true)
		sb.WriteString("\t\t\t\t\t\tdepends_on: {\n")
		generateDependsOnContent(sb, r.DependsOn, "\t\t\t\t\t\t\t")
		sb.WriteString("\t\t\t\t\t\t}\n")
		sb.WriteString("\t\t\t\t\t},\n")
	} else {
		// Compact single-line format (existing behavior)
		sb.WriteString("\t\t\t\t\t{")
		fmt.Fprintf(sb, "name: %q", r.Name)
		generateRuntimeConfigFields(sb, r, "", false)
		sb.WriteString("},\n")
	}
}

// generateRuntimeConfigFields writes runtime-specific container fields.
// When multiLine is true, each field is written on its own line with the given indent.
// When multiLine is false, fields are written inline prefixed with ", ".
func generateRuntimeConfigFields(sb *strings.Builder, r *RuntimeConfig, indent string, multiLine bool) {
	if r.Name != RuntimeContainer {
		return
	}

	// writeField writes a scalar field in either format.
	writeField := func(key, value string) {
		if multiLine {
			fmt.Fprintf(sb, "%s%s: %s\n", indent, key, value)
		} else {
			fmt.Fprintf(sb, ", %s: %s", key, value)
		}
	}

	// writeList writes a bracketed list field.
	writeList := func(key string, items []string) {
		if multiLine {
			sb.WriteString(indent + key + ": [")
		} else {
			sb.WriteString(", " + key + ": [")
		}
		for j, item := range items {
			if j > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%q", item)
		}
		if multiLine {
			sb.WriteString("]\n")
		} else {
			sb.WriteString("]")
		}
	}

	if r.EnableHostSSH {
		writeField("enable_host_ssh", "true")
	}
	if r.Containerfile != "" {
		writeField("containerfile", fmt.Sprintf("%q", r.Containerfile))
	}
	if r.Image != "" {
		writeField("image", fmt.Sprintf("%q", r.Image))
	}
	if len(r.Volumes) > 0 {
		writeList("volumes", r.Volumes)
	}
	if len(r.Ports) > 0 {
		writeList("ports", r.Ports)
	}
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
			sb.WriteString("]")
			if dep.Execute {
				sb.WriteString(", execute: true")
			}
			sb.WriteString("},\n")
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
		for _, capDep := range deps.Capabilities {
			sb.WriteString(indent + "\t{alternatives: [")
			for i, alt := range capDep.Alternatives {
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
