
# Failed to parse invowkfile!

Your invowkfile contains syntax errors or invalid configuration.

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
$ invowk --ivk-verbose cmd
~~~

## Example of valid command definition:
~~~cue
cmds: [
  {
    name: "build"
    description: "Build the project"
    implementations: [
      {
        script: """
          echo "Building..."
          go build ./...
          """
          runtimes: [{name: "native"}]  // or "virtual", "container"
      }
    ]
  }
]
~~~