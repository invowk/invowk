import type { Snippet } from '../snippets';

export const flagsArgsSnippets = {
  // =============================================================================
  // FLAGS AND ARGUMENTS
  // =============================================================================

  'flags-args/flags-basic': {
    language: 'cue',
    code: `{
    name: "deploy"
    flags: [
        {name: "env", description: "Target environment", required: true},
        {name: "dry-run", description: "Simulate deployment", type: "bool", default_value: "false"},
        {name: "replicas", description: "Number of replicas", type: "int", default_value: "3"}
    ]
    implementations: [
        {
            script: """
                echo "Deploying to $INVOWK_FLAG_ENV with $INVOWK_FLAG_REPLICAS replicas"
                if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                    echo "(dry run - no changes made)"
                fi
                """
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }
    ]
}`,
  },

  'flags-args/args-basic': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [
        {name: "name", description: "Name to greet", required: true},
        {name: "title", description: "Optional title", required: false, default_value: ""}
    ]
    implementations: [
        {
            script: """
                if [ -n "$INVOWK_ARG_TITLE" ]; then
                    echo "Hello, $INVOWK_ARG_TITLE $INVOWK_ARG_NAME!"
                else
                    echo "Hello, $INVOWK_ARG_NAME!"
                fi
                """
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }
    ]
}`,
  },

  // Overview page snippets
  'flags-args/overview-example-command': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy to an environment"
    
    // Flags - named options
    flags: [
        {name: "dry-run", description: "Preview without applying changes", type: "bool", default_value: "false"},
        {name: "replicas", description: "Number of replicas", type: "int", default_value: "1"},
    ]

    // Arguments - positional values
    args: [
        {name: "environment", description: "Target environment", required: true},
        {name: "services", description: "Services to deploy", variadic: true},
    ]
    
    implementations: [{
        script: """
            echo "Deploying to $INVOWK_ARG_ENVIRONMENT"
            echo "Replicas: $INVOWK_FLAG_REPLICAS"
            echo "Dry run: $INVOWK_FLAG_DRY_RUN"
            echo "Services: $INVOWK_ARG_SERVICES"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/overview-usage': {
    language: 'bash',
    code: `invowk cmd deploy production api web --dry-run --replicas=3`,
  },

  'flags-args/overview-flags-example': {
    language: 'cue',
    code: `flags: [
    {name: "verbose", description: "Enable verbose output", type: "bool", short: "v"},
    {name: "output", description: "Output directory", type: "string", short: "o", default_value: "./dist"},
    {name: "count", description: "Number of iterations", type: "int", default_value: "1"},
]`,
  },

  'flags-args/overview-flags-usage': {
    language: 'bash',
    code: `# Long form
invowk cmd build --verbose --output=./build --count=5

# Short form
invowk cmd build -v -o=./build`,
  },

  'flags-args/overview-args-example': {
    language: 'cue',
    code: `args: [
    {name: "source", description: "Source directory", required: true},
    {name: "destination", description: "Destination path", default_value: "./output"},
    {name: "files", description: "Files to copy", variadic: true},
]`,
  },

  'flags-args/overview-args-usage': {
    language: 'bash',
    code: `invowk cmd copy ./src ./dest file1.txt file2.txt`,
  },

  'flags-args/overview-shell-positional': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [
        {name: "first-name", description: "First name"},
        {name: "last-name", description: "Last name"},
    ]
    implementations: [{
        script: """
            # Using environment variables
            echo "Hello, $INVOWK_ARG_FIRST_NAME $INVOWK_ARG_LAST_NAME!"
            
            # Or using positional parameters
            echo "Hello, $1 $2!"
            
            # All arguments
            echo "All: $@"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/overview-mixing': {
    language: 'bash',
    code: `# All equivalent
invowk cmd deploy production --dry-run api web
invowk cmd deploy --dry-run production api web
invowk cmd deploy production api web --dry-run`,
  },

  'flags-args/overview-help': {
    language: 'bash',
    code: `invowk cmd deploy --help`,
  },

  'flags-args/overview-help-output': {
    language: 'text',
    code: `Usage:
  invowk cmd deploy <environment> [services]... [flags]

Arguments:
  environment          (required) - Target environment
  services             (optional) (variadic) - Services to deploy

Flags:
      --dry-run          Perform a dry run (default: false)
  -n, --replicas int     Number of replicas (default: 1)
  -h, --help             help for deploy`,
  },

  // Flags page snippets
  'flags-args/flags-defining': {
    language: 'cue',
    code: `{
    name: "deploy"
    flags: [
        {
            name: "environment"
            description: "Target environment"
            required: true
        },
        {
            name: "dry-run"
            description: "Simulate without changes"
            type: "bool"
            default_value: "false"
        },
        {
            name: "replicas"
            description: "Number of replicas"
            type: "int"
            default_value: "1"
        }
    ]
    implementations: [...]
}`,
  },

  'flags-args/flags-type-string': {
    language: 'cue',
    code: `{name: "message", description: "Custom message", type: "string"}
// or simply
{name: "message", description: "Custom message"}`,
  },

  'flags-args/flags-type-string-usage': {
    language: 'bash',
    code: `invowk cmd run --message="Hello World"`,
  },

  'flags-args/flags-type-bool': {
    language: 'cue',
    code: `{name: "verbose", description: "Enable verbose output", type: "bool", default_value: "false"}`,
  },

  'flags-args/flags-type-bool-usage': {
    language: 'bash',
    code: `# Enable
invowk cmd run --verbose
invowk cmd run --verbose=true

# Disable (explicit)
invowk cmd run --verbose=false`,
  },

  'flags-args/flags-type-int': {
    language: 'cue',
    code: `{name: "count", description: "Number of iterations", type: "int", default_value: "5"}`,
  },

  'flags-args/flags-type-int-usage': {
    language: 'bash',
    code: `invowk cmd run --count=10
invowk cmd run --count=-1  # Negative allowed`,
  },

  'flags-args/flags-type-float': {
    language: 'cue',
    code: `{name: "threshold", description: "Confidence threshold", type: "float", default_value: "0.95"}`,
  },

  'flags-args/flags-type-float-usage': {
    language: 'bash',
    code: `invowk cmd run --threshold=0.8
invowk cmd run --threshold=1.5e-3  # Scientific notation`,
  },

  'flags-args/flags-required': {
    language: 'cue',
    code: `{
    name: "target"
    description: "Deployment target"
    required: true  // Must be provided
}`,
  },

  'flags-args/flags-required-usage': {
    language: 'bash',
    code: `# Error: missing required flag
invowk cmd deploy
# Error: flag 'target' is required

# Success
invowk cmd deploy --target=production`,
  },

  'flags-args/flags-optional': {
    language: 'cue',
    code: `{
    name: "timeout"
    description: "Request timeout in seconds"
    type: "int"
    default_value: "30"  // Used if not provided
}`,
  },

  'flags-args/flags-optional-usage': {
    language: 'bash',
    code: `# Uses default (30)
invowk cmd request

# Override
invowk cmd request --timeout=60`,
  },

  'flags-args/flags-short-aliases': {
    language: 'cue',
    code: `flags: [
    {name: "verbose", description: "Verbose output", type: "bool", short: "v"},
    {name: "output", description: "Output file", short: "o"},
    {name: "force", description: "Force overwrite", type: "bool", short: "f"},
]`,
  },

  'flags-args/flags-short-usage': {
    language: 'bash',
    code: `# Long form
invowk cmd build --verbose --output=./dist --force

# Short form
invowk cmd build -v -o=./dist -f

# Mixed
invowk cmd build -v --output=./dist -f`,
  },

  'flags-args/flags-validation': {
    language: 'cue',
    code: `flags: [
    {
        name: "env"
        description: "Environment name"
        validation: "^(dev|staging|prod)$"
        default_value: "dev"
    },
    {
        name: "version"
        description: "Semantic version"
        validation: "^[0-9]+\.[0-9]+\.[0-9]+$"
    }
]`,
  },

  'flags-args/flags-validation-usage': {
    language: 'bash',
    code: `# Valid
invowk cmd deploy --env=prod --version=1.2.3

# Invalid - fails before execution
invowk cmd deploy --env=production
# Error: flag 'env' value 'production' does not match required pattern '^(dev|staging|prod)$'`,
  },

  'flags-args/flags-accessing': {
    language: 'cue',
    code: `{
    name: "deploy"
    flags: [
        {name: "env", description: "Environment", required: true},
        {name: "dry-run", description: "Dry run", type: "bool", default_value: "false"},
        {name: "replica-count", description: "Replicas", type: "int", default_value: "1"},
    ]
    implementations: [{
        script: """
            echo "Environment: $INVOWK_FLAG_ENV"
            echo "Dry run: $INVOWK_FLAG_DRY_RUN"
            echo "Replicas: $INVOWK_FLAG_REPLICA_COUNT"
            
            if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                echo "Would deploy $INVOWK_FLAG_REPLICA_COUNT replicas to $INVOWK_FLAG_ENV"
            else
                ./deploy.sh "$INVOWK_FLAG_ENV" "$INVOWK_FLAG_REPLICA_COUNT"
            fi
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/flags-build-example': {
    language: 'cue',
    code: `{
    name: "build"
    description: "Build the application"
    flags: [
        {name: "mode", description: "Build mode", validation: "^(debug|release)$", default_value: "debug"},
        {name: "output", description: "Output directory", short: "o", default_value: "./build"},
        {name: "verbose", description: "Verbose output", type: "bool", short: "v"},
        {name: "parallel", description: "Parallel jobs", type: "int", short: "j", default_value: "4"},
    ]
    implementations: [{
        script: """
            mkdir -p "$INVOWK_FLAG_OUTPUT"
            
            VERBOSE=""
            if [ "$INVOWK_FLAG_VERBOSE" = "true" ]; then
                VERBOSE="-v"
            fi
            
            go build $VERBOSE -o "$INVOWK_FLAG_OUTPUT/app" ./...
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/flags-deploy-example': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy to cloud"
    flags: [
        {
            name: "env"
            description: "Target environment"
            short: "e"
            required: true
            validation: "^(dev|staging|prod)$"
        },
        {
            name: "version"
            description: "Version to deploy"
            short: "v"
            validation: "^[0-9]+\.[0-9]+\.[0-9]+$"
        },
        {
            name: "dry-run"
            description: "Simulate deployment"
            type: "bool"
            short: "n"
            default_value: "false"
        },
        {
            name: "timeout"
            description: "Deployment timeout (seconds)"
            type: "int"
            default_value: "300"
        }
    ]
    implementations: [{
        script: """
            echo "Deploying version \${INVOWK_FLAG_VERSION:-latest} to $INVOWK_FLAG_ENV"
            
            ARGS="--timeout=$INVOWK_FLAG_TIMEOUT"
            if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                ARGS="$ARGS --dry-run"
            fi
            
            ./scripts/deploy.sh "$INVOWK_FLAG_ENV" $ARGS
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/flags-syntax': {
    language: 'bash',
    code: `# Equals sign
--output=./dist

# Space separator
--output ./dist

# Short with equals
-o=./dist

# Short with value
-o ./dist

# Boolean toggle (enables)
--verbose
-v

# Boolean explicit
--verbose=true
--verbose=false`,
  },

  // Positional arguments page snippets
  'flags-args/args-defining': {
    language: 'cue',
    code: `{
    name: "copy"
    args: [
        {
            name: "source"
            description: "Source file or directory"
            required: true
        },
        {
            name: "destination"
            description: "Destination path"
            required: true
        }
    ]
    implementations: [...]
}`,
  },

  'flags-args/args-defining-usage': {
    language: 'bash',
    code: `invowk cmd copy ./src ./dest`,
  },

  'flags-args/args-type-string': {
    language: 'cue',
    code: `{name: "filename", description: "File to process", type: "string"}`,
  },

  'flags-args/args-type-int': {
    language: 'cue',
    code: `{name: "count", description: "Number of items", type: "int", default_value: "10"}`,
  },

  'flags-args/args-type-int-usage': {
    language: 'bash',
    code: `invowk cmd generate 5`,
  },

  'flags-args/args-type-float': {
    language: 'cue',
    code: `{name: "ratio", description: "Scaling ratio", type: "float", default_value: "1.0"}`,
  },

  'flags-args/args-type-float-usage': {
    language: 'bash',
    code: `invowk cmd scale 0.5`,
  },

  'flags-args/args-required': {
    language: 'cue',
    code: `args: [
    {name: "input", description: "Input file", required: true},
    {name: "output", description: "Output file", required: true},
]`,
  },

  'flags-args/args-required-usage': {
    language: 'bash',
    code: `# Error: missing required argument
invowk cmd convert input.txt
# Error: argument 'output' is required

# Success
invowk cmd convert input.txt output.txt`,
  },

  'flags-args/args-optional': {
    language: 'cue',
    code: `args: [
    {name: "input", description: "Input file", required: true},
    {name: "format", description: "Output format", default_value: "json"},
]`,
  },

  'flags-args/args-optional-usage': {
    language: 'bash',
    code: `# Uses default format (json)
invowk cmd parse input.txt

# Override format
invowk cmd parse input.txt yaml`,
  },

  'flags-args/args-ordering': {
    language: 'cue',
    code: `// Good
args: [
    {name: "input", description: "Input file", required: true},      // Required first
    {name: "output", description: "Output file", required: true},     // Required second
    {name: "format", description: "Output format", default_value: "json"}, // Optional last
]

// Bad - will cause validation error
args: [
    {name: "format", description: "Output format", default_value: "json"}, // Optional can't come first
    {name: "input", description: "Input file", required: true},
]`,
  },

  'flags-args/args-variadic': {
    language: 'cue',
    code: `{
    name: "process"
    args: [
        {name: "output", description: "Output file", required: true},
        {name: "inputs", description: "Input files", variadic: true},
    ]
    implementations: [{
        script: """
            echo "Output: $INVOWK_ARG_OUTPUT"
            echo "Inputs: $INVOWK_ARG_INPUTS"
            echo "Count: $INVOWK_ARG_INPUTS_COUNT"

            for i in $(seq 1 $INVOWK_ARG_INPUTS_COUNT); do
                eval "file=\$INVOWK_ARG_INPUTS_$i"
                echo "Processing: $file"
            done
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/args-variadic-usage': {
    language: 'bash',
    code: `invowk cmd process out.txt a.txt b.txt c.txt
# Output: out.txt
# Inputs: a.txt b.txt c.txt
# Count: 3
# Processing: a.txt
# Processing: b.txt
# Processing: c.txt`,
  },

  'flags-args/args-validation': {
    language: 'cue',
    code: `args: [
    {
        name: "environment"
        description: "Target environment"
        required: true
        validation: "^(dev|staging|prod)$"
    },
    {
        name: "version"
        description: "Version number"
        validation: "^[0-9]+\.[0-9]+\.[0-9]+$"
    }
]`,
  },

  'flags-args/args-validation-usage': {
    language: 'bash',
    code: `# Valid
invowk cmd deploy prod 1.2.3

# Invalid
invowk cmd deploy production
# Error: argument 'environment' value 'production' does not match pattern '^(dev|staging|prod)$'`,
  },

  'flags-args/args-accessing': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [
        {name: "first-name", description: "First name", required: true},
        {name: "last-name", description: "Last name", default_value: "User"},
    ]
    implementations: [{
        script: """
            echo "Hello, $INVOWK_ARG_FIRST_NAME $INVOWK_ARG_LAST_NAME!"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/args-positional-params': {
    language: 'cue',
    code: `{
    name: "copy"
    args: [
        {name: "source", description: "Source path", required: true},
        {name: "dest", description: "Destination path", required: true},
    ]
    implementations: [{
        script: """
            # Using environment variables
            cp "$INVOWK_ARG_SOURCE" "$INVOWK_ARG_DEST"
            
            # Or positional parameters
            cp "$1" "$2"
            
            # All arguments
            echo "Args: $@"
            echo "Count: $#"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/args-convert-example': {
    language: 'cue',
    code: `{
    name: "convert"
    description: "Convert file format"
    args: [
        {
            name: "input"
            description: "Input file"
            required: true
        },
        {
            name: "output"
            description: "Output file"
            required: true
        },
        {
            name: "format"
            description: "Output format"
            default_value: "json"
            validation: "^(json|yaml|toml|xml)$"
        }
    ]
    implementations: [{
        script: """
            echo "Converting $1 to $2 as $3"
            ./converter --input="$1" --output="$2" --format="$3"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/args-compress-example': {
    language: 'cue',
    code: `{
    name: "compress"
    description: "Compress files into archive"
    args: [
        {
            name: "archive"
            description: "Output archive name"
            required: true
            validation: "\.(zip|tar\.gz|tgz)$"
        },
        {
            name: "files"
            description: "Files to compress"
            variadic: true
        }
    ]
    implementations: [{
        script: """
            if [ -z "$INVOWK_ARG_FILES" ]; then
                echo "No files specified!"
                exit 1
            fi
            
            # Use the space-separated list
            tar -czvf "$INVOWK_ARG_ARCHIVE" $INVOWK_ARG_FILES
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/args-deploy-example': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy services"
    args: [
        {
            name: "env"
            description: "Target environment"
            required: true
            validation: "^(dev|staging|prod)$"
        },
        {
            name: "replicas"
            description: "Number of replicas"
            type: "int"
            default_value: "1"
        },
        {
            name: "services"
            description: "Services to deploy"
            variadic: true
        }
    ]
    implementations: [{
        script: """
            echo "Deploying to $INVOWK_ARG_ENV with $INVOWK_ARG_REPLICAS replicas"
            
            if [ -n "$INVOWK_ARG_SERVICES" ]; then
                for i in $(seq 1 $INVOWK_ARG_SERVICES_COUNT); do
                    eval "service=\$INVOWK_ARG_SERVICES_$i"
                    echo "Deploying $service..."
                    kubectl scale deployment/$service --replicas=$INVOWK_ARG_REPLICAS
                done
            else
                echo "Deploying all services..."
                kubectl scale deployment --all --replicas=$INVOWK_ARG_REPLICAS
            fi
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/args-mixing-flags': {
    language: 'bash',
    code: `# All equivalent
invowk cmd deploy prod 3 --dry-run
invowk cmd deploy --dry-run prod 3
invowk cmd deploy prod --dry-run 3`,
  },
} satisfies Record<string, Snippet>;
