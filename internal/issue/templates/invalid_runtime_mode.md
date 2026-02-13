
# Invalid runtime mode!

The specified runtime mode is not recognized.

## Valid runtime modes:
- **native**: Execute using system shell
- **virtual**: Execute using built-in sh interpreter
- **container**: Execute inside a container

## Example:
~~~cue
default_runtime: "native"

cmds: [
  {
    name: "build"
    implementations: [
      {
        script: "make build"
          runtimes: [{name: "container"}]  // Override for this command
      }
    ]
  }
]
~~~