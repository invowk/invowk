---
sidebar_position: 5
---

# Table and Spin

Data display and loading indicator components.

## Table

Display and optionally select from tabular data.

### Basic Usage

```bash
# From a CSV file
invowk tui table --file data.csv

# From stdin with separator
echo -e "name|age|city\nAlice|30|NYC\nBob|25|LA" | invowk tui table --separator "|"
```

### Options

| Option | Description |
|--------|-------------|
| `--file` | CSV file to display |
| `--separator` | Column separator (default: `,`) |
| `--selectable` | Allow row selection |
| `--height` | Table height |

### Examples

```bash
# Display CSV
invowk tui table --file users.csv

# Custom separator (TSV)
invowk tui table --file data.tsv --separator $'\t'

# Pipe-separated
cat data.txt | invowk tui table --separator "|"
```

### Selectable Tables

```bash
# Select a row
SELECTED=$(invowk tui table --file servers.csv --selectable)
echo "Selected: $SELECTED"
```

The selected row is returned as the full CSV line.

### Real-World Examples

#### Display Server List

```bash
# servers.csv:
# name,ip,status
# web-1,10.0.0.1,running
# web-2,10.0.0.2,running
# db-1,10.0.0.3,stopped

invowk tui table --file servers.csv
```

#### Select and SSH

```bash
# Select a server
SERVER=$(cat servers.csv | invowk tui table --selectable | cut -d',' -f2)
ssh "user@$SERVER"
```

#### Process List

```bash
ps aux --no-headers | awk '{print $1","$2","$11}' | \
    (echo "USER,PID,COMMAND"; cat) | \
    invowk tui table --selectable
```

---

## Spin

Show a spinner while running a long command.

### Basic Usage

```bash
invowk tui spin --title "Installing..." -- npm install
```

### Options

| Option | Description |
|--------|-------------|
| `--title` | Spinner title/message |
| `--type` | Spinner animation type |
| `--show-output` | Show command output |

### Spinner Types

Available spinner animations:

- `line` - Simple line
- `dot` - Dots
- `minidot` - Small dots
- `jump` - Jumping dots
- `pulse` - Pulsing dot
- `points` - Points
- `globe` - Spinning globe
- `moon` - Moon phases
- `monkey` - Monkey
- `meter` - Progress meter
- `hamburger` - Hamburger menu
- `ellipsis` - Ellipsis

```bash
invowk tui spin --type globe --title "Downloading..." -- curl -O https://example.com/file
invowk tui spin --type moon --title "Building..." -- make build
invowk tui spin --type pulse --title "Testing..." -- npm test
```

### Examples

```bash
# Basic spinner
invowk tui spin --title "Building..." -- go build ./...

# With specific type
invowk tui spin --type dot --title "Installing dependencies..." -- npm install

# Long-running task
invowk tui spin --title "Compiling assets..." -- webpack --mode production
```

### Chained Spinners

```bash
echo "Step 1/3: Dependencies"
invowk tui spin --title "Installing..." -- npm install

echo "Step 2/3: Build"
invowk tui spin --title "Building..." -- npm run build

echo "Step 3/3: Tests"
invowk tui spin --title "Testing..." -- npm test

echo "Done!" | invowk tui style --foreground "#00FF00" --bold
```

### Exit Code Handling

The spin command returns the exit code of the wrapped command:

```bash
if invowk tui spin --title "Testing..." -- npm test; then
    echo "Tests passed!"
else
    echo "Tests failed!"
    exit 1
fi
```

### In Scripts

```cue
{
    name: "deploy"
    description: "Deploy with progress indication"
    implementations: [{
        script: """
            echo "Deploying application..."
            
            invowk tui spin --title "Building Docker image..." -- \
                docker build -t myapp .
            
            invowk tui spin --title "Pushing to registry..." -- \
                docker push myapp
            
            invowk tui spin --title "Updating Kubernetes..." -- \
                kubectl rollout restart deployment/myapp
            
            invowk tui spin --title "Waiting for rollout..." -- \
                kubectl rollout status deployment/myapp
            
            echo "Deployment complete!" | invowk tui style --foreground "#00FF00" --bold
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Combined Patterns

### Select Then Execute with Spinner

```bash
# Choose what to build
PROJECT=$(invowk tui choose --title "Build which project?" api web worker)

# Build with spinner
invowk tui spin --title "Building $PROJECT..." -- make "build-$PROJECT"
```

### Table Selection with Spinner Action

```bash
# Select server
SERVER=$(invowk tui table --file servers.csv --selectable | cut -d',' -f1)

# Restart with spinner
invowk tui spin --title "Restarting $SERVER..." -- ssh "$SERVER" "systemctl restart myapp"
```

## Next Steps

- [Format and Style](./format-and-style) - Text formatting
- [Overview](./overview) - All TUI components
