
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
includes: [
    {path: "/home/user/global-commands/com.example.tools.invowkmod"}
]

ui: {
  color_scheme: "auto"
  verbose: false
}
~~~
