---
sidebar_position: 1
---

# Interpreters

By default, Invowk executes scripts using a shell. But you can use other interpreters like Python, Ruby, Node.js, or any executable that can run scripts.

## Auto-Detection from Shebang

When a script starts with a shebang (`#!`), Invowk automatically uses that interpreter:

```cue
{
    name: "analyze"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import sys
            import json
            
            data = {"status": "ok", "python": sys.version}
            print(json.dumps(data, indent=2))
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

Common shebang patterns:

| Shebang | Interpreter |
|---------|-------------|
| `#!/usr/bin/env python3` | Python 3 (portable) |
| `#!/usr/bin/env node` | Node.js |
| `#!/usr/bin/env ruby` | Ruby |
| `#!/usr/bin/env perl` | Perl |
| `#!/bin/bash` | Bash (direct path) |

## Explicit Interpreter

Specify an interpreter directly in the runtime config:

```cue
{
    name: "script"
    implementations: [{
        script: """
            import sys
            print(f"Python {sys.version_info.major}.{sys.version_info.minor}")
            """
        target: {
            runtimes: [{
                name: "native"
                interpreter: "python3"  // Explicit
            }]
        }
    }]
}
```

The explicit interpreter takes precedence over shebang detection.

## Interpreter with Arguments

Pass arguments to the interpreter:

```cue
{
    name: "unbuffered"
    implementations: [{
        script: """
            import time
            for i in range(5):
                print(f"Count: {i}")
                time.sleep(1)
            """
        target: {
            runtimes: [{
                name: "native"
                interpreter: "python3 -u"  // Unbuffered output
            }]
        }
    }]
}
```

More examples:

```cue
// Perl with warnings
interpreter: "perl -w"

// Ruby with debug mode
interpreter: "ruby -d"

// Node with specific options
interpreter: "node --max-old-space-size=4096"
```

## Container Interpreters

Interpreters work in containers too:

```cue
{
    name: "analyze"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import os
            print(f"Running in container at {os.getcwd()}")
            """
        target: {
            runtimes: [{
                name: "container"
                image: "python:3.11-alpine"
            }]
        }
    }]
}
```

Or with explicit interpreter:

```cue
{
    name: "script"
    implementations: [{
        script: """
            console.log('Hello from Node in container!')
            console.log('Node version:', process.version)
            """
        target: {
            runtimes: [{
                name: "container"
                image: "node:20-alpine"
                interpreter: "node"
            }]
        }
    }]
}
```

## Accessing Arguments

Arguments work the same with any interpreter:

```cue
{
    name: "greet"
    args: [{name: "name", default_value: "World"}]
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import sys
            import os
            
            # Via command line args
            name = sys.argv[1] if len(sys.argv) > 1 else "World"
            print(f"Hello, {name}!")
            
            # Or via environment variable
            name = os.environ.get("INVOWK_ARG_NAME", "World")
            print(f"Hello again, {name}!")
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Supported Interpreters

Any executable in PATH can be used:

- **Python**: `python3`, `python`
- **JavaScript**: `node`, `deno`, `bun`
- **Ruby**: `ruby`
- **Perl**: `perl`
- **PHP**: `php`
- **Lua**: `lua`
- **R**: `Rscript`
- **Shell**: `bash`, `sh`, `zsh`, `fish`
- **Custom**: Any executable

## Virtual Runtime Limitation

The `interpreter` field is **not supported** with the virtual runtime:

```cue
// This will NOT work!
{
    name: "bad"
    implementations: [{
        script: "print('hello')"
        target: {
            runtimes: [{
                name: "virtual"
                interpreter: "python3"  // ERROR!
            }]
        }
    }]
}
```

The virtual runtime uses the built-in mvdan/sh interpreter and cannot execute Python, Ruby, or other interpreters. Use native or container runtime instead.

## Fallback Behavior

When no shebang and no explicit interpreter:

- **Native runtime**: Uses system's default shell
- **Container runtime**: Uses `/bin/sh -c`

## Best Practices

1. **Use shebang for portability**: Scripts work standalone too
2. **Use `/usr/bin/env`**: More portable than direct paths
3. **Explicit interpreter for no-shebang scripts**: When you don't want a shebang line
4. **Match container image**: Ensure interpreter exists in the image

## Next Steps

- [Working Directory](./workdir) - Control execution location
- [Platform-Specific](./platform-specific) - Per-platform implementations
