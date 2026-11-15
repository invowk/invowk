
# Failed to load the configuration file you named!

The file given by `--ivk-config` or `INVOWK_CONFIG` could not be loaded. When you name a file explicitly, invowk does not fall back to your user config file or to the defaults.

## Things you can try:
- Check that the path exists and is a readable file
- `--ivk-config` may be relative to the current directory; `INVOWK_CONFIG` may not
- Check the file's CUE syntax and fields
- To use your user config file instead, drop `--ivk-config` or unset `INVOWK_CONFIG`:
~~~
$ unset INVOWK_CONFIG
~~~

## Inspect which file invowk would use:
~~~
$ invowk config path
~~~
