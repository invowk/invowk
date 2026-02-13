
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
~~~