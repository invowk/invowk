// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"maps"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
)

func cloneScannedInvowkfile(file *ScannedInvowkfile) *ScannedInvowkfile {
	if file == nil {
		return nil
	}
	cloned := *file
	if file.Invowkfile != nil {
		cloned.Invowkfile = cloneInvowkfile(file.Invowkfile)
	}
	return &cloned
}

func cloneScannedModule(module *ScannedModule) *ScannedModule {
	if module == nil {
		return nil
	}
	cloned := *module
	if module.Module != nil {
		mod := *module.Module
		if module.Module.Metadata != nil {
			metadata := *module.Module.Metadata
			metadata.Requires = append([]invowkmod.ModuleRequirement(nil), module.Module.Metadata.Requires...)
			mod.Metadata = &metadata
		}
		cloned.Module = &mod
	}
	if module.Invowkfile != nil {
		cloned.Invowkfile = cloneInvowkfile(module.Invowkfile)
	}
	if module.LockFile != nil {
		lockFile := *module.LockFile
		lockFile.Modules = cloneLockedModules(module.LockFile.Modules)
		cloned.LockFile = &lockFile
	}
	cloned.VendoredModules = cloneVendoredModules(module.VendoredModules)
	cloned.VendoredHashes = append([]invowkmod.VendoredHashEvaluation(nil), module.VendoredHashes...)
	cloned.Symlinks = append([]SymlinkRef(nil), module.Symlinks...)
	return &cloned
}

func cloneScriptRef(ref ScriptRef) ScriptRef {
	cloned := ref
	cloned.Runtimes = cloneRuntimeConfigs(ref.Runtimes)
	return cloned
}

func cloneInvowkfile(inv *invowkfile.Invowkfile) *invowkfile.Invowkfile {
	if inv == nil {
		return nil
	}
	cloned := *inv
	cloned.Env = cloneEnvConfig(inv.Env)
	cloned.DependsOn = cloneDependsOn(inv.DependsOn)
	cloned.Commands = cloneCommands(inv.Commands)
	return &cloned
}

func cloneCommands(commands []invowkfile.Command) []invowkfile.Command {
	if commands == nil {
		return nil
	}
	cloned := make([]invowkfile.Command, len(commands))
	for i := range commands {
		cloned[i] = cloneCommand(commands[i])
	}
	return cloned
}

func cloneCommand(command invowkfile.Command) invowkfile.Command {
	cloned := command
	cloned.Implementations = cloneImplementations(command.Implementations)
	cloned.Env = cloneEnvConfig(command.Env)
	cloned.DependsOn = cloneDependsOn(command.DependsOn)
	cloned.Flags = append([]invowkfile.Flag(nil), command.Flags...)
	cloned.Args = append([]invowkfile.Argument(nil), command.Args...)
	cloned.Watch = cloneWatchConfig(command.Watch)
	return cloned
}

func cloneImplementations(implementations []invowkfile.Implementation) []invowkfile.Implementation {
	if implementations == nil {
		return nil
	}
	cloned := make([]invowkfile.Implementation, len(implementations))
	for i := range implementations {
		cloned[i] = cloneImplementation(implementations[i])
	}
	return cloned
}

func cloneImplementation(impl invowkfile.Implementation) invowkfile.Implementation {
	cloned := impl
	cloned.Runtimes = cloneRuntimeConfigs(impl.Runtimes)
	cloned.Platforms = append([]invowkfile.PlatformConfig(nil), impl.Platforms...)
	cloned.Env = cloneEnvConfig(impl.Env)
	cloned.DependsOn = cloneDependsOn(impl.DependsOn)
	return cloned
}

func cloneRuntimeConfigs(runtimes []invowkfile.RuntimeConfig) []invowkfile.RuntimeConfig {
	if runtimes == nil {
		return nil
	}
	cloned := make([]invowkfile.RuntimeConfig, len(runtimes))
	for i := range runtimes {
		cloned[i] = runtimes[i]
		cloned[i].EnvInheritAllow = append([]invowkfile.EnvVarName(nil), runtimes[i].EnvInheritAllow...)
		cloned[i].EnvInheritDeny = append([]invowkfile.EnvVarName(nil), runtimes[i].EnvInheritDeny...)
		cloned[i].DependsOn = cloneDependsOn(runtimes[i].DependsOn)
		cloned[i].Volumes = append([]invowkfile.VolumeMountSpec(nil), runtimes[i].Volumes...)
		cloned[i].Ports = append([]invowkfile.PortMappingSpec(nil), runtimes[i].Ports...)
	}
	return cloned
}

func cloneEnvConfig(env *invowkfile.EnvConfig) *invowkfile.EnvConfig {
	if env == nil {
		return nil
	}
	cloned := *env
	cloned.Files = append([]invowkfile.DotenvFilePath(nil), env.Files...)
	cloned.Vars = maps.Clone(env.Vars)
	return &cloned
}

func cloneDependsOn(dependsOn *invowkfile.DependsOn) *invowkfile.DependsOn {
	if dependsOn == nil {
		return nil
	}
	cloned := *dependsOn
	cloned.Tools = cloneToolDependencies(dependsOn.Tools)
	cloned.Commands = cloneCommandDependencies(dependsOn.Commands)
	cloned.Filepaths = cloneFilepathDependencies(dependsOn.Filepaths)
	cloned.Capabilities = cloneCapabilityDependencies(dependsOn.Capabilities)
	cloned.CustomChecks = cloneCustomCheckDependencies(dependsOn.CustomChecks)
	cloned.EnvVars = cloneEnvVarDependencies(dependsOn.EnvVars)
	return &cloned
}

func cloneToolDependencies(deps []invowkfile.ToolDependency) []invowkfile.ToolDependency {
	if deps == nil {
		return nil
	}
	cloned := make([]invowkfile.ToolDependency, len(deps))
	for i := range deps {
		cloned[i] = deps[i]
		cloned[i].Alternatives = append([]invowkfile.BinaryName(nil), deps[i].Alternatives...)
	}
	return cloned
}

func cloneCommandDependencies(deps []invowkfile.CommandDependency) []invowkfile.CommandDependency {
	if deps == nil {
		return nil
	}
	cloned := make([]invowkfile.CommandDependency, len(deps))
	for i := range deps {
		cloned[i] = deps[i]
		cloned[i].Alternatives = append([]invowkfile.CommandName(nil), deps[i].Alternatives...)
	}
	return cloned
}

func cloneFilepathDependencies(deps []invowkfile.FilepathDependency) []invowkfile.FilepathDependency {
	if deps == nil {
		return nil
	}
	cloned := make([]invowkfile.FilepathDependency, len(deps))
	for i := range deps {
		cloned[i] = deps[i]
		cloned[i].Alternatives = append([]invowkfile.FilesystemPath(nil), deps[i].Alternatives...)
	}
	return cloned
}

func cloneCapabilityDependencies(deps []invowkfile.CapabilityDependency) []invowkfile.CapabilityDependency {
	if deps == nil {
		return nil
	}
	cloned := make([]invowkfile.CapabilityDependency, len(deps))
	for i := range deps {
		cloned[i] = deps[i]
		cloned[i].Alternatives = append([]invowkfile.CapabilityName(nil), deps[i].Alternatives...)
	}
	return cloned
}

func cloneCustomCheckDependencies(deps []invowkfile.CustomCheckDependency) []invowkfile.CustomCheckDependency {
	if deps == nil {
		return nil
	}
	cloned := make([]invowkfile.CustomCheckDependency, len(deps))
	for i := range deps {
		cloned[i] = deps[i]
		if deps[i].ExpectedCode != nil {
			expectedCode := *deps[i].ExpectedCode
			cloned[i].ExpectedCode = &expectedCode
		}
		cloned[i].Alternatives = cloneCustomChecks(deps[i].Alternatives)
	}
	return cloned
}

func cloneCustomChecks(checks []invowkfile.CustomCheck) []invowkfile.CustomCheck {
	if checks == nil {
		return nil
	}
	cloned := make([]invowkfile.CustomCheck, len(checks))
	for i := range checks {
		cloned[i] = checks[i]
		if checks[i].ExpectedCode != nil {
			expectedCode := *checks[i].ExpectedCode
			cloned[i].ExpectedCode = &expectedCode
		}
	}
	return cloned
}

func cloneEnvVarDependencies(deps []invowkfile.EnvVarDependency) []invowkfile.EnvVarDependency {
	if deps == nil {
		return nil
	}
	cloned := make([]invowkfile.EnvVarDependency, len(deps))
	for i := range deps {
		cloned[i] = deps[i]
		cloned[i].Alternatives = append([]invowkfile.EnvVarCheck(nil), deps[i].Alternatives...)
	}
	return cloned
}

func cloneWatchConfig(watch *invowkfile.WatchConfig) *invowkfile.WatchConfig {
	if watch == nil {
		return nil
	}
	cloned := *watch
	cloned.Patterns = append([]invowkfile.GlobPattern(nil), watch.Patterns...)
	cloned.Ignore = append([]invowkfile.GlobPattern(nil), watch.Ignore...)
	return &cloned
}

func cloneLockedModules(modules map[invowkmod.ModuleRefKey]invowkmod.LockedModule) map[invowkmod.ModuleRefKey]invowkmod.LockedModule {
	if modules == nil {
		return nil
	}
	cloned := make(map[invowkmod.ModuleRefKey]invowkmod.LockedModule, len(modules))
	maps.Copy(cloned, modules)
	return cloned
}

func cloneVendoredModules(modules []*invowkmod.Module) []*invowkmod.Module {
	cloned := make([]*invowkmod.Module, 0, len(modules))
	for _, module := range modules {
		cloned = append(cloned, cloneVendoredModule(module))
	}
	return cloned
}

func cloneVendoredModule(module *invowkmod.Module) *invowkmod.Module {
	if module == nil {
		return nil
	}
	mod := *module
	mod.Metadata = cloneModuleMetadata(module.Metadata)
	return &mod
}

func cloneModuleMetadata(metadata *invowkmod.Invowkmod) *invowkmod.Invowkmod {
	if metadata == nil {
		return nil
	}
	cloned := *metadata
	cloned.Requires = append([]invowkmod.ModuleRequirement(nil), metadata.Requires...)
	return &cloned
}
