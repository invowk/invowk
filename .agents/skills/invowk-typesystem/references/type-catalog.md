# Invowk Value-Type Catalog

This catalog is generated from repository source and documents current type-system coverage.

## Coverage Summary

- `Validate()` value types: 110
- Primitive-wrapper + `Validate()` types: 86
- Composite validator + `Validate()` types: 24
- Primitive-wrapper declarations (all): 86
- Alias/re-export type declarations: 13

## All Types With `Validate() error`

| Type | Kind | Source |
| --- | --- | --- |
| `RuntimeSelection` | `composite-validator` | `internal/app/execute/orchestrator.go:108` |
| `LoadOptions` | `composite-validator` | `internal/config/provider.go:48` |
| `AutoProvisionConfig` | `composite-validator` | `internal/config/types.go:285` |
| `BinaryFilePath` | `primitive-wrapper` | `internal/config/types.go:396` |
| `CacheDirPath` | `primitive-wrapper` | `internal/config/types.go:420` |
| `ColorScheme` | `primitive-wrapper` | `internal/config/types.go:498` |
| `Config` | `composite-validator` | `internal/config/types.go:337` |
| `ContainerConfig` | `composite-validator` | `internal/config/types.go:314` |
| `ContainerEngine` | `primitive-wrapper` | `internal/config/types.go:452` |
| `IncludeEntry` | `composite-validator` | `internal/config/types.go:236` |
| `ModuleIncludePath` | `primitive-wrapper` | `internal/config/types.go:375` |
| `RuntimeMode` | `primitive-wrapper` | `internal/config/types.go:475` |
| `UIConfig` | `composite-validator` | `internal/config/types.go:262` |
| `HostFilesystemPath` | `primitive-wrapper` | `internal/container/engine_base.go:278` |
| `MountTargetPath` | `primitive-wrapper` | `internal/container/engine_base.go:300` |
| `NetworkPort` | `primitive-wrapper` | `internal/container/engine_base.go:256` |
| `PortMapping` | `composite-validator` | `internal/container/engine_base.go:369` |
| `PortProtocol` | `primitive-wrapper` | `internal/container/engine_base.go:217` |
| `SELinuxLabel` | `primitive-wrapper` | `internal/container/engine_base.go:239` |
| `VolumeMount` | `composite-validator` | `internal/container/engine_base.go:329` |
| `BuildOptions` | `composite-validator` | `internal/container/engine.go:325` |
| `ContainerID` | `composite-validator` | `internal/container/engine.go:232` |
| `ContainerName` | `composite-validator` | `internal/container/engine.go:274` |
| `EngineType` | `primitive-wrapper` | `internal/container/engine.go:394` |
| `HostMapping` | `primitive-wrapper` | `internal/container/engine.go:299` |
| `ImageTag` | `primitive-wrapper` | `internal/container/engine.go:254` |
| `RunOptions` | `composite-validator` | `internal/container/engine.go:356` |
| `State` | `primitive-wrapper` | `internal/core/serverbase/state.go:71` |
| `Diagnostic` | `composite-validator` | `internal/discovery/diagnostic.go:312` |
| `DiagnosticCode` | `primitive-wrapper` | `internal/discovery/diagnostic.go:196` |
| `Severity` | `primitive-wrapper` | `internal/discovery/diagnostic.go:152` |
| `SourceID` | `primitive-wrapper` | `internal/discovery/discovery_commands.go:101` |
| `Source` | `primitive-wrapper` | `internal/discovery/discovery_files.go:61` |
| `HttpLink` | `primitive-wrapper` | `internal/issue/issue.go:246` |
| `Id` | `primitive-wrapper` | `internal/issue/issue.go:192` |
| `MarkdownMsg` | `primitive-wrapper` | `internal/issue/issue.go:222` |
| `Config` | `composite-validator` | `internal/provision/config.go:78` |
| `InitDiagnosticCode` | `primitive-wrapper` | `internal/runtime/registry_factory.go:75` |
| `EnvContext` | `composite-validator` | `internal/runtime/runtime.go:355` |
| `ExecutionID` | `primitive-wrapper` | `internal/runtime/runtime.go:273` |
| `RuntimeType` | `primitive-wrapper` | `internal/runtime/runtime.go:254` |
| `TUIContext` | `composite-validator` | `internal/runtime/runtime.go:330` |
| `TUIServerToken` | `primitive-wrapper` | `internal/runtime/tui_types.go:81` |
| `TUIServerURL` | `primitive-wrapper` | `internal/runtime/tui_types.go:57` |
| `Config` | `composite-validator` | `internal/sshserver/server.go:110` |
| `HostAddress` | `primitive-wrapper` | `internal/sshserver/types.go:67` |
| `TokenValue` | `primitive-wrapper` | `internal/sshserver/types.go:81` |
| `BorderStyle` | `primitive-wrapper` | `internal/tui/border_style.go:45` |
| `ColorSpec` | `primitive-wrapper` | `internal/tui/color_spec.go:33` |
| `ComponentType` | `primitive-wrapper` | `internal/tui/embeddable.go:160` |
| `FormatType` | `primitive-wrapper` | `internal/tui/format.go:71` |
| `SelectionIndex` | `primitive-wrapper` | `internal/tui/selection_index.go:30` |
| `Component` | `primitive-wrapper` | `internal/tuiserver/protocol.go:254` |
| `AuthToken` | `primitive-wrapper` | `internal/tuiserver/types.go:33` |
| `SpinnerType` | `primitive-wrapper` | `internal/tui/spin.go:129` |
| `TerminalDimension` | `primitive-wrapper` | `internal/tui/terminal_dimension.go:32` |
| `TextAlign` | `primitive-wrapper` | `internal/tui/text_align.go:40` |
| `Config` | `composite-validator` | `internal/tui/tui.go:151` |
| `Theme` | `primitive-wrapper` | `internal/tui/tui.go:132` |
| `Config` | `composite-validator` | `internal/watch/watcher.go:134` |
| `ArgumentName` | `primitive-wrapper` | `pkg/invowkfile/argument.go:113` |
| `ArgumentType` | `primitive-wrapper` | `pkg/invowkfile/argument.go:92` |
| `CapabilityName` | `primitive-wrapper` | `pkg/invowkfile/capabilities.go:269` |
| `CommandCategory` | `primitive-wrapper` | `pkg/invowkfile/command.go:110` |
| `CommandName` | `primitive-wrapper` | `pkg/invowkfile/command.go:89` |
| `ContainerfilePath` | `primitive-wrapper` | `pkg/invowkfile/containerfile_path.go:33` |
| `BinaryName` | `primitive-wrapper` | `pkg/invowkfile/dependency.go:228` |
| `CheckName` | `primitive-wrapper` | `pkg/invowkfile/dependency.go:249` |
| `ScriptContent` | `primitive-wrapper` | `pkg/invowkfile/dependency.go:269` |
| `DotenvFilePath` | `primitive-wrapper` | `pkg/invowkfile/dotenv_path.go:35` |
| `DurationString` | `primitive-wrapper` | `pkg/invowkfile/duration.go:41` |
| `EnvVarName` | `primitive-wrapper` | `pkg/invowkfile/env.go:57` |
| `FlagName` | `primitive-wrapper` | `pkg/invowkfile/flag.go:130` |
| `FlagShorthand` | `primitive-wrapper` | `pkg/invowkfile/flag.go:158` |
| `FlagType` | `primitive-wrapper` | `pkg/invowkfile/flag.go:109` |
| `PlatformRuntimeKey` | `composite-validator` | `pkg/invowkfile/implementation.go:74` |
| `InterpreterSpec` | `primitive-wrapper` | `pkg/invowkfile/interpreter_spec.go:32` |
| `ShellPath` | `primitive-wrapper` | `pkg/invowkfile/invowkfile.go:89` |
| `ModuleMetadata` | `composite-validator` | `pkg/invowkfile/module.go:125` |
| `PortMappingSpec` | `primitive-wrapper` | `pkg/invowkfile/port_mapping.go:34` |
| `RegexPattern` | `primitive-wrapper` | `pkg/invowkfile/regex_pattern.go:41` |
| `ContainerImage` | `primitive-wrapper` | `pkg/invowkfile/runtime.go:285` |
| `EnvInheritMode` | `primitive-wrapper` | `pkg/invowkfile/runtime.go:247` |
| `PlatformType` | `primitive-wrapper` | `pkg/invowkfile/runtime.go:264` |
| `RuntimeMode` | `primitive-wrapper` | `pkg/invowkfile/runtime.go:231` |
| `ValidationSeverity` | `primitive-wrapper` | `pkg/invowkfile/validation_types.go:116` |
| `ValidatorName` | `primitive-wrapper` | `pkg/invowkfile/validation_types.go:149` |
| `VolumeMountSpec` | `primitive-wrapper` | `pkg/invowkfile/volume_mount.go:34` |
| `GlobPattern` | `primitive-wrapper` | `pkg/invowkfile/watch.go:59` |
| `WorkDir` | `primitive-wrapper` | `pkg/invowkfile/workdir.go:38` |
| `GitCommit` | `primitive-wrapper` | `pkg/invowkmod/git_types.go:78` |
| `GitURL` | `primitive-wrapper` | `pkg/invowkmod/git_types.go:55` |
| `Invowkmod` | `composite-validator` | `pkg/invowkmod/invowkmod.go:289` |
| `ModuleAlias` | `primitive-wrapper` | `pkg/invowkmod/invowkmod.go:340` |
| `ModuleID` | `primitive-wrapper` | `pkg/invowkmod/invowkmod.go:272` |
| `ModuleRequirement` | `composite-validator` | `pkg/invowkmod/invowkmod.go:418` |
| `SubdirectoryPath` | `primitive-wrapper` | `pkg/invowkmod/invowkmod.go:367` |
| `ValidationIssueType` | `primitive-wrapper` | `pkg/invowkmod/invowkmod.go:456` |
| `LockFileVersion` | `primitive-wrapper` | `pkg/invowkmod/lockfile.go:123` |
| `ModuleNamespace` | `primitive-wrapper` | `pkg/invowkmod/lockfile.go:108` |
| `ModuleRefKey` | `primitive-wrapper` | `pkg/invowkmod/lockfile.go:145` |
| `ModuleShortName` | `primitive-wrapper` | `pkg/invowkmod/module_short_name.go:47` |
| `ConstraintOp` | `primitive-wrapper` | `pkg/invowkmod/semver_types.go:121` |
| `SemVer` | `primitive-wrapper` | `pkg/invowkmod/semver_types.go:80` |
| `SemVerConstraint` | `primitive-wrapper` | `pkg/invowkmod/semver_types.go:100` |
| `SandboxType` | `primitive-wrapper` | `pkg/platform/sandbox.go:72` |
| `DescriptionText` | `primitive-wrapper` | `pkg/types/description.go:38` |
| `ExitCode` | `primitive-wrapper` | `pkg/types/exit_code.go:36` |
| `FilesystemPath` | `primitive-wrapper` | `pkg/types/filesystem_path.go:34` |
| `ListenPort` | `primitive-wrapper` | `pkg/types/listen_port.go:33` |

## Primitive-Wrapper Value Types (All Declarations)

| Type | Underlying | Source |
| --- | --- | --- |
| `ArgumentName` | `string` | `pkg/invowkfile/argument.go:45` |
| `ArgumentType` | `string` | `pkg/invowkfile/argument.go:34` |
| `AuthToken` | `string` | `internal/tuiserver/types.go:17` |
| `BinaryFilePath` | `string` | `internal/config/types.go:107` |
| `BinaryName` | `string` | `pkg/invowkfile/dependency.go:25` |
| `BorderStyle` | `string` | `internal/tui/border_style.go:31` |
| `CacheDirPath` | `string` | `internal/config/types.go:118` |
| `CapabilityName` | `string` | `pkg/invowkfile/capabilities.go:36` |
| `CheckName` | `string` | `pkg/invowkfile/dependency.go:36` |
| `ColorScheme` | `string` | `internal/config/types.go:86` |
| `ColorSpec` | `string` | `internal/tui/color_spec.go:18` |
| `CommandCategory` | `string` | `pkg/invowkfile/command.go:34` |
| `CommandName` | `string` | `pkg/invowkfile/command.go:24` |
| `Component` | `string` | `internal/tuiserver/protocol.go:41` |
| `ComponentType` | `string` | `internal/tui/embeddable.go:128` |
| `ConstraintOp` | `string` | `pkg/invowkmod/semver_types.go:59` |
| `ContainerEngine` | `string` | `internal/config/types.go:66` |
| `ContainerfilePath` | `string` | `pkg/invowkfile/containerfile_path.go:19` |
| `ContainerImage` | `string` | `pkg/invowkfile/runtime.go:101` |
| `DescriptionText` | `string` | `pkg/types/description.go:24` |
| `DiagnosticCode` | `string` | `internal/discovery/diagnostic.go:73` |
| `DotenvFilePath` | `string` | `pkg/invowkfile/dotenv_path.go:19` |
| `DurationString` | `string` | `pkg/invowkfile/duration.go:18` |
| `EngineType` | `string` | `internal/container/engine.go:50` |
| `EnvInheritMode` | `string` | `pkg/invowkfile/runtime.go:74` |
| `EnvVarName` | `string` | `pkg/invowkfile/env.go:24` |
| `ExecutionID` | `string` | `internal/runtime/runtime.go:50` |
| `ExitCode` | `int` | `pkg/types/exit_code.go:18` |
| `FilesystemPath` | `string` | `pkg/types/filesystem_path.go:18` |
| `FlagName` | `string` | `pkg/invowkfile/flag.go:53` |
| `FlagShorthand` | `string` | `pkg/invowkfile/flag.go:65` |
| `FlagType` | `string` | `pkg/invowkfile/flag.go:42` |
| `FormatType` | `string` | `internal/tui/format.go:30` |
| `GitCommit` | `string` | `pkg/invowkmod/git_types.go:34` |
| `GitURL` | `string` | `pkg/invowkmod/git_types.go:25` |
| `GlobPattern` | `string` | `pkg/invowkfile/watch.go:19` |
| `HostAddress` | `string` | `internal/sshserver/types.go:27` |
| `HostFilesystemPath` | `string` | `internal/container/engine_base.go:161` |
| `HostMapping` | `string` | `internal/container/engine.go:65` |
| `HttpLink` | `string` | `internal/issue/issue.go:149` |
| `Id` | `int` | `internal/issue/issue.go:143` |
| `ImageTag` | `string` | `internal/container/engine.go:58` |
| `InitDiagnosticCode` | `string` | `internal/runtime/registry_factory.go:34` |
| `InterpreterSpec` | `string` | `pkg/invowkfile/interpreter_spec.go:18` |
| `ListenPort` | `int` | `pkg/types/listen_port.go:18` |
| `LockFileVersion` | `string` | `pkg/invowkmod/lockfile.go:37` |
| `MarkdownMsg` | `string` | `internal/issue/issue.go:146` |
| `ModuleAlias` | `string` | `pkg/invowkmod/invowkmod.go:93` |
| `ModuleID` | `string` | `pkg/invowkmod/invowkmod.go:82` |
| `ModuleIncludePath` | `string` | `internal/config/types.go:96` |
| `ModuleNamespace` | `string` | `pkg/invowkmod/lockfile.go:27` |
| `ModuleRefKey` | `string` | `pkg/invowkmod/lockfile.go:48` |
| `ModuleShortName` | `string` | `pkg/invowkmod/module_short_name.go:30` |
| `MountTargetPath` | `string` | `internal/container/engine_base.go:170` |
| `NetworkPort` | `uint16` | `internal/container/engine_base.go:152` |
| `PlatformType` | `string` | `pkg/invowkfile/runtime.go:77` |
| `PortMappingSpec` | `string` | `pkg/invowkfile/port_mapping.go:17` |
| `PortProtocol` | `string` | `internal/container/engine_base.go:134` |
| `RegexPattern` | `string` | `pkg/invowkfile/regex_pattern.go:18` |
| `RuntimeMode` | `string` | `internal/config/types.go:77` |
| `RuntimeMode` | `string` | `pkg/invowkfile/runtime.go:71` |
| `RuntimeType` | `string` | `internal/runtime/runtime.go:220` |
| `SandboxType` | `string` | `pkg/platform/sandbox.go:41` |
| `ScriptContent` | `string` | `pkg/invowkfile/dependency.go:46` |
| `SelectionIndex` | `int` | `internal/tui/selection_index.go:16` |
| `SELinuxLabel` | `string` | `internal/container/engine_base.go:143` |
| `SemVerConstraint` | `string` | `pkg/invowkmod/semver_types.go:49` |
| `SemVer` | `string` | `pkg/invowkmod/semver_types.go:39` |
| `Severity` | `string` | `internal/discovery/diagnostic.go:70` |
| `ShellPath` | `string` | `pkg/invowkfile/invowkfile.go:30` |
| `SourceID` | `string` | `internal/discovery/discovery_commands.go:31` |
| `Source` | `int` | `internal/discovery/discovery_files.go:26` |
| `SpinnerType` | `int` | `internal/tui/spin.go:53` |
| `State` | `int32` | `internal/core/serverbase/state.go:30` |
| `SubdirectoryPath` | `string` | `pkg/invowkmod/invowkmod.go:104` |
| `TerminalDimension` | `int` | `internal/tui/terminal_dimension.go:18` |
| `TextAlign` | `string` | `internal/tui/text_align.go:25` |
| `Theme` | `string` | `internal/tui/tui.go:51` |
| `TokenValue` | `string` | `internal/sshserver/types.go:31` |
| `TUIServerToken` | `string` | `internal/runtime/tui_types.go:36` |
| `TUIServerURL` | `string` | `internal/runtime/tui_types.go:25` |
| `ValidationIssueType` | `string` | `pkg/invowkmod/invowkmod.go:132` |
| `ValidationSeverity` | `int` | `pkg/invowkfile/validation_types.go:28` |
| `ValidatorName` | `string` | `pkg/invowkfile/validation_types.go:38` |
| `VolumeMountSpec` | `string` | `pkg/invowkfile/volume_mount.go:17` |
| `WorkDir` | `string` | `pkg/invowkfile/workdir.go:18` |

## Alias/Re-export Types

| Alias | Target | Source |
| --- | --- | --- |
| `CommandScope` | `invowkmod.CommandScope` | `pkg/invowkfile/module.go:46` |
| `DescriptionText` | `types.DescriptionText` | `pkg/invowkfile/description.go:16` |
| `ExitCode` | `types.ExitCode` | `internal/runtime/exit_code.go:15` |
| `FilesystemPath` | `types.FilesystemPath` | `pkg/invowkfile/filesystem_path.go:16` |
| `InvalidDescriptionTextError` | `types.InvalidDescriptionTextError` | `pkg/invowkfile/description.go:19` |
| `InvalidExitCodeError` | `types.InvalidExitCodeError` | `internal/runtime/exit_code.go:18` |
| `InvalidFilesystemPathError` | `types.InvalidFilesystemPathError` | `pkg/invowkfile/filesystem_path.go:19` |
| `InvalidListenPortError` | `types.InvalidListenPortError` | `internal/sshserver/types.go:50` |
| `Invowkmod` | `invowkmod.Invowkmod` | `pkg/invowkfile/module.go:42` |
| `ListenPort` | `types.ListenPort` | `internal/sshserver/types.go:35` |
| `Module` | `invowkmod.Module` | `pkg/invowkfile/parse.go:25` |
| `ModuleRequirement` | `invowkmod.ModuleRequirement` | `pkg/invowkfile/module.go:37` |
| `Platform` | `PlatformType` | `pkg/invowkfile/invowkfile.go:40` |

## Regeneration

Run:

```bash
.agents/skills/invowk-typesystem/scripts/extract_value_types.sh > .agents/skills/invowk-typesystem/references/type-catalog.md
```
