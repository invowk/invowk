---
sidebar_position: 2
---

# Quickstart

Let's run your first Invowk command in under 2 minutes. Seriously, grab a coffee first if you want - you'll have time to spare.

## Create Your First Invkfile

Navigate to any project directory and initialize an invkfile:

```bash
cd my-project
invowk init
```

This creates an `invkfile.cue` with a sample command. Let's peek inside:

```cue
group: "myproject"
version: "1.0"
description: "My project commands"

commands: [
    {
        name: "hello"
        description: "Say hello!"
        implementations: [
            {
                script: "echo 'Hello from Invowk!'"
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    }
]
```

Don't worry about understanding everything yet - we'll cover that soon!

## List Available Commands

See what commands are available:

```bash
invowk cmd --list
# or just
invowk cmd
```

You'll see something like:

```
Available Commands
  (* = default runtime)

From current directory:
  myproject hello - Say hello! [native*] (linux, macos, windows)
```

Notice how the command is prefixed with `myproject` (the group name). This keeps commands organized and prevents naming conflicts.

## Run a Command

Now let's run it:

```bash
invowk cmd myproject hello
```

Output:

```
Hello from Invowk!
```

That's it! You just ran your first Invowk command.

## Let's Make It More Interesting

Edit your `invkfile.cue` to add a more useful command:

```cue
group: "myproject"
version: "1.0"
description: "My project commands"

commands: [
    {
        name: "hello"
        description: "Say hello!"
        implementations: [
            {
                script: "echo 'Hello from Invowk!'"
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    },
    {
        name: "info"
        description: "Show system information"
        implementations: [
            {
                script: """
                    echo "=== System Info ==="
                    echo "User: $USER"
                    echo "Directory: $(pwd)"
                    echo "Date: $(date)"
                    """
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            }
        ]
    }
]
```

Now run:

```bash
invowk cmd myproject info
```

You'll see your system information printed out nicely.

## Try the Virtual Runtime

One of Invowk's superpowers is the **virtual runtime** - a built-in shell interpreter that works the same on every platform:

```cue
{
    name: "cross-platform"
    description: "Works the same everywhere!"
    implementations: [
        {
            script: "echo 'This runs identically on Linux, Mac, and Windows!'"
            target: {
                runtimes: [{name: "virtual"}]
            }
        }
    ]
}
```

The virtual runtime uses the [mvdan/sh](https://github.com/mvdan/sh) interpreter, giving you consistent POSIX shell behavior across all platforms.

## What's Next?

You've just scratched the surface! Head to [Your First Invkfile](./your-first-invkfile) to learn how to build more powerful commands with:

- Multiple runtime options (native, virtual, container)
- Dependencies that are validated before running
- Command flags and arguments
- Environment variables
- And much more!
