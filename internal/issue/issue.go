// SPDX-License-Identifier: EPL-2.0

package issue

import (
	"github.com/charmbracelet/glamour"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type Id int

const (
	FileNotFoundId Id = iota + 1
	TuiServerStartFailedId
	InvkfileNotFoundId
	InvkfileParseErrorId
	CommandNotFoundId
	RuntimeNotAvailableId
	ContainerEngineNotFoundId
	DockerfileNotFoundId
	ScriptExecutionFailedId
	ConfigLoadFailedId
	InvalidRuntimeModeId
	DependencyCycleId
	ShellNotFoundId
	PermissionDeniedId
	DependenciesNotSatisfiedId
	HostNotSupportedId
)

type MarkdownMsg string

type HttpLink string

type Renderer interface {
	Render(in string, stylePath string) (string, error)
}

type Issue struct {
	id       Id          // ID used to lookup the issue
	mdMsg    MarkdownMsg // Markdown text that will be rendered
	docLinks []HttpLink  // must never be empty, because we need to have docs about all issue types
	extLinks []HttpLink  // external links that might be useful for the user
}

func (i *Issue) Id() Id {
	return i.id
}

func (i *Issue) MarkdownMsg() MarkdownMsg {
	return i.mdMsg
}

func (i *Issue) DocLinks() []HttpLink {
	return slices.Clone(i.docLinks)
}

func (i *Issue) ExtLinks() []HttpLink {
	return slices.Clone(i.extLinks)
}

func (i *Issue) Render(stylePath string) (string, error) {
	extraMd := ""
	if len(i.docLinks) > 0 || len(i.extLinks) > 0 {
		extraMd += "\n\n"
		extraMd += "## See also: "
		for _, link := range i.docLinks {
			extraMd += "- [" + string(link) + "]"
		}
		for _, link := range i.extLinks {
			extraMd += "- [" + string(link) + "]"
		}
	}
	return render(string(i.mdMsg)+extraMd, stylePath)
}

var (
	render = glamour.Render

	fileNotFoundIssue = &Issue{
		id: FileNotFoundId,
		mdMsg: `
# Dang, we have run into an issue!
We have failed to start our super powered TUI Server due to weird conditions.

## Things you can try to fix and retry
- Run this command  
~~~
$ invowk fix
~~~  
    and try again what you doing before.`,
	}

	invkfileNotFoundIssue = &Issue{
		id: InvkfileNotFoundId,
		mdMsg: `
# No invkfile found!

We searched for an invkfile but couldn't find one in the expected locations.

## Search locations (in order of precedence):
1. Current directory
2. ~/.invowk/cmds/
3. Paths configured in your config file

## Things you can try:
- Create an invkfile in your current directory:
~~~
$ invowk init
~~~

- Or specify a different directory:
~~~
$ cd /path/to/your/project
$ invowk cmd list
~~~

## Example invkfile structure:
~~~cue
version: "1.0"
description: "My project commands"

commands: [
  {
    name: "build"
    description: "Build the project"
    script: "go build -o myapp ./..."
  },
  {
    name: "test"
    description: "Run tests"
    script: "go test ./..."
  },
]
~~~`,
	}

	invkfileParseErrorIssue = &Issue{
		id: InvkfileParseErrorId,
		mdMsg: `
# Failed to parse invkfile!

Your invkfile contains syntax errors or invalid configuration.

## Common issues:
- Invalid CUE syntax (missing quotes, braces, etc.)
- Unknown field names
- Invalid values for known fields
- Missing required fields (name, script for commands)

## Things you can try:
- Check the error message above for the specific line/column
- Validate your CUE syntax using the cue command-line tool
- Run with verbose mode for more details:
~~~
$ invowk --verbose cmd list
~~~

## Example of valid command definition:
~~~cue
commands: [
  {
    name: "build"
    description: "Build the project"
    implementations: [
      {
        script: """
          echo "Building..."
          go build ./...
          """
        target: {
          runtimes: [{name: "native"}]  // or "virtual", "container"
        }
      }
    ]
  }
]
~~~`,
	}

	commandNotFoundIssue = &Issue{
		id: CommandNotFoundId,
		mdMsg: `
# Command not found!

The command you specified was not found in any of the available invkfiles.

## Things you can try:
- List all available commands:
~~~
$ invowk cmd list
~~~

- Check for typos in the command name
- Verify the invkfile contains your command definition
- Use tab completion:
~~~
$ invowk cmd <TAB>
~~~`,
	}

	runtimeNotAvailableIssue = &Issue{
		id: RuntimeNotAvailableId,
		mdMsg: `
# Runtime not available!

The specified runtime mode is not available on your system.

## Available runtimes:
- **native**: Uses your system's default shell (bash, sh, powershell, etc.)
- **virtual**: Uses the built-in mvdan/sh interpreter
- **container**: Runs commands inside a Docker/Podman container

## Things you can try:
- Change the runtime in your invkfile:
~~~cue
default_runtime: "native"
~~~

- Or specify runtime per-command:
~~~cue
commands: [
  {
    name: "build"
    implementations: [
      {
        script: "echo 'hello'"
        target: {
          runtimes: [{name: "virtual"}]
        }
      }
    ]
  }
]
~~~`,
	}

	containerEngineNotFoundIssue = &Issue{
		id: ContainerEngineNotFoundId,
		mdMsg: `
# Container engine not found!

You tried to use the 'container' runtime but no container engine is available.

## Supported container engines:
- **Podman** (recommended for rootless containers)
- **Docker**

## Things you can try:
- Install Podman:
  - Linux: ` + "`sudo apt install podman`" + ` or ` + "`sudo dnf install podman`" + `
  - macOS: ` + "`brew install podman`" + `
  - Windows: Download from https://podman.io

- Install Docker:
  - https://docs.docker.com/get-docker/

- Switch to a different runtime:
~~~cue
default_runtime: "native"  // or "virtual"
~~~

- Configure your preferred engine in ~/.config/invowk/config.cue:
~~~cue
container_engine: "podman"  // or "docker"
~~~`,
	}

	dockerfileNotFoundIssue = &Issue{
		id: DockerfileNotFoundId,
		mdMsg: `
# Dockerfile not found!

The 'container' runtime requires a Dockerfile to build the execution environment.

## Things you can try:
- Create a Dockerfile in the same directory as your invkfile:
~~~dockerfile
FROM alpine:latest
RUN apk add --no-cache bash coreutils
WORKDIR /workspace
~~~

- Or specify a Dockerfile path in your invkfile:
~~~cue
container: {
  dockerfile: "path/to/Dockerfile"
}
~~~

- Or use a pre-built image:
~~~cue
container: {
  image: "ubuntu:22.04"
}
~~~`,
	}

	scriptExecutionFailedIssue = &Issue{
		id: ScriptExecutionFailedId,
		mdMsg: `
# Script execution failed!

The command's script failed to execute properly.

## Common causes:
- Command not found in PATH
- Permission denied
- Syntax error in script
- Missing dependencies

## Things you can try:
- Run with verbose mode for more details:
~~~
$ invowk --verbose cmd <command>
~~~

- Test the script manually in your shell
- Check file permissions and PATH settings
- For container runtime, check the container has required tools`,
	}

	configLoadFailedIssue = &Issue{
		id: ConfigLoadFailedId,
		mdMsg: `
# Failed to load configuration!

Could not load the invowk configuration file.

## Configuration file locations:
- Linux: ~/.config/invowk/config.cue
- macOS: ~/Library/Application Support/invowk/config.cue
- Windows: %APPDATA%\invowk\config.cue

## Things you can try:
- Create a default configuration:
~~~
$ invowk config init
~~~

- Check the configuration syntax
- Remove the config file to use defaults:
~~~
$ rm ~/.config/invowk/config.cue
~~~

## Example configuration:
~~~cue
container_engine: "podman"
default_runtime: "native"
search_paths: [
    "/home/user/global-commands"
]

ui: {
  color_scheme: "auto"
  verbose: false
}
~~~`,
	}

	invalidRuntimeModeIssue = &Issue{
		id: InvalidRuntimeModeId,
		mdMsg: `
# Invalid runtime mode!

The specified runtime mode is not recognized.

## Valid runtime modes:
- **native**: Execute using system shell
- **virtual**: Execute using built-in sh interpreter
- **container**: Execute inside a container

## Example:
~~~cue
default_runtime: "native"

commands: [
  {
    name: "build"
    implementations: [
      {
        script: "make build"
        target: {
          runtimes: [{name: "container"}]  // Override for this command
        }
      }
    ]
  }
]
~~~`,
	}

	dependencyCycleIssue = &Issue{
		id: DependencyCycleId,
		mdMsg: `
# Dependency cycle detected!

Your command dependencies form a cycle, which would cause infinite execution.

## Example of a cycle:
~~~cue
commands: [
  {
    name: "a"
    depends_on: {
      commands: [{alternatives: ["b"]}]
    }
  },
  {
    name: "b"
    depends_on: {
      commands: [{alternatives: ["a"]}]  // Cycle: a -> b -> a
    }
  }
]
~~~

## Things you can try:
- Review the depends_on fields in your invkfile
- Remove the circular dependency
- Use a linear dependency chain instead`,
	}

	shellNotFoundIssue = &Issue{
		id: ShellNotFoundId,
		mdMsg: `
# Shell not found!

Could not find a suitable shell for the 'native' runtime.

## Shells we look for:
- Linux/macOS: $SHELL, bash, sh
- Windows: pwsh, powershell, cmd

## Things you can try:
- Install bash or another POSIX shell
- Set the SHELL environment variable
- Use the 'virtual' runtime instead (built-in shell):
~~~cue
default_runtime: "virtual"
~~~`,
	}

	permissionDeniedIssue = &Issue{
		id: PermissionDeniedId,
		mdMsg: `
# Permission denied!

You don't have permission to perform this operation.

## Common causes:
- Trying to write to a protected directory
- Script file is not executable
- Container engine requires elevated permissions

## Things you can try:
- Check file/directory permissions
- For containers, ensure you're in the docker/podman group:
~~~
$ sudo usermod -aG docker $USER
~~~

- Use rootless containers with Podman
- Run invowk from a directory you own`,
	}

	dependenciesNotSatisfiedIssue = &Issue{
		id: DependenciesNotSatisfiedId,
		mdMsg: `
# Dependencies not satisfied!

The command cannot run because some dependencies are not available.

## Things you can try:
- Install the missing tools listed above
- Check that the tools are in your PATH
- Run the required commands before this one
- Update your invkfile to remove unnecessary dependencies`,
	}

	hostNotSupportedIssue = &Issue{
		id: HostNotSupportedId,
		mdMsg: `
# Host not supported!

This command cannot run on your current operating system.

## Things you can try:
- Check the command's 'works_on.hosts' setting in your invkfile
- Run this command on a supported operating system
- Use a container runtime to run the command on a different OS`,
	}

	issues = map[Id]*Issue{
		fileNotFoundIssue.Id():             fileNotFoundIssue,
		invkfileNotFoundIssue.Id():         invkfileNotFoundIssue,
		invkfileParseErrorIssue.Id():       invkfileParseErrorIssue,
		commandNotFoundIssue.Id():          commandNotFoundIssue,
		runtimeNotAvailableIssue.Id():      runtimeNotAvailableIssue,
		containerEngineNotFoundIssue.Id():  containerEngineNotFoundIssue,
		dockerfileNotFoundIssue.Id():       dockerfileNotFoundIssue,
		scriptExecutionFailedIssue.Id():    scriptExecutionFailedIssue,
		configLoadFailedIssue.Id():         configLoadFailedIssue,
		invalidRuntimeModeIssue.Id():       invalidRuntimeModeIssue,
		dependencyCycleIssue.Id():          dependencyCycleIssue,
		shellNotFoundIssue.Id():            shellNotFoundIssue,
		permissionDeniedIssue.Id():         permissionDeniedIssue,
		dependenciesNotSatisfiedIssue.Id(): dependenciesNotSatisfiedIssue,
		hostNotSupportedIssue.Id():         hostNotSupportedIssue,
	}
)

func Values() []*Issue {
	return maps.Values(issues)
}

func Get(id Id) *Issue {
	return issues[id]
}
