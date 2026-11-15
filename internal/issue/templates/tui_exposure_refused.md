
# Interactive container run refused!

`-i` runs a TUI server that the container must reach. In this container engine mode, invowk has no narrow route to it, so it refuses the interactive run instead of binding the server to every network interface. Nothing was started.

## Things you can try:
- Run the same command without `-i`:
~~~
$ invowk cmd <command>
~~~

- For rootless Podman, use an ephemeral container (no `persistent` block) and install pasta (the `passt` package)

- To accept a TUI server on `0.0.0.0`, reachable from the network and guarded only by a per-session token, set this in the config file invowk reads (the `--ivk-config` or `INVOWK_CONFIG` file when one is given, otherwise your user config file):
~~~cue
container: {
	tui_server_bind: "all-interfaces"
}
~~~

## See which config file invowk reads:
~~~
$ invowk config path
~~~
