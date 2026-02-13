
# Runtime not available!

The specified runtime mode is not available on your system.

## Available runtimes:
- **native**: Uses your system's default shell (bash, sh, powershell, etc.)
- **virtual**: Uses the built-in mvdan/sh interpreter
- **container**: Runs commands inside a Docker/Podman container

## Things you can try:
- Change the runtime in your invowkfile:
~~~cue
default_runtime: "native"
~~~

- Or specify runtime per-command:
~~~cue
cmds: [
  {
    name: "build"
    implementations: [
      {
        script: "echo 'hello'"
          runtimes: [{name: "virtual"}]
      }
    ]
  }
]
~~~