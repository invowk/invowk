
# No invowkfile found!

We searched for an invowkfile but couldn't find one in the expected locations.

## Search locations (in order of precedence):
1. Current directory (invowkfile and sibling modules)
2. Configured includes (module paths from config)
3. ~/.invowk/cmds/ (modules only)

## Things you can try:
- Create an invowkfile in your current directory:
~~~
$ invowk init
~~~

- Or specify a different directory:
~~~
$ cd /path/to/your/project
$ invowk cmd
~~~

## Example invowkfile structure:
~~~cue
version: "1.0"
description: "My project commands"

cmds: [
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
~~~