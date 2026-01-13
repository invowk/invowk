---
sidebar_position: 4
---

# Filter and File

Search and file selection components.

## Filter

Fuzzy filter through a list of options.

### Basic Usage

```bash
# From arguments
invowk tui filter "apple" "banana" "cherry" "date"

# From stdin
ls | invowk tui filter
```

### Options

| Option | Description |
|--------|-------------|
| `--title` | Filter prompt |
| `--placeholder` | Search placeholder |
| `--limit` | Max selections (default: 1) |
| `--no-limit` | Unlimited selections |
| `--indicator` | Selection indicator |

### Examples

```bash
# Filter files
FILE=$(ls | invowk tui filter --title "Select file")

# Multi-select filter
FILES=$(ls *.go | invowk tui filter --no-limit --title "Select Go files")

# With placeholder
ITEM=$(invowk tui filter --placeholder "Type to search..." opt1 opt2 opt3)
```

### Real-World Examples

#### Select Git Branch

```bash
BRANCH=$(git branch --list | tr -d '* ' | invowk tui filter --title "Checkout branch")
git checkout "$BRANCH"
```

#### Select Docker Container

```bash
CONTAINER=$(docker ps --format "{{.Names}}" | invowk tui filter --title "Select container")
docker logs -f "$CONTAINER"
```

#### Select Process to Kill

```bash
PID=$(ps aux | invowk tui filter --title "Select process" | awk '{print $2}')
if [ -n "$PID" ]; then
    kill "$PID"
fi
```

#### Filter Commands

```bash
CMD=$(invowk cmd --list 2>/dev/null | grep "^  " | invowk tui filter --title "Run command")
# Extract command name and run it
```

---

## File

File and directory picker with navigation.

### Basic Usage

```bash
# Pick any file from current directory
invowk tui file

# Start in specific directory
invowk tui file /home/user/documents
```

### Options

| Option | Description |
|--------|-------------|
| `--directory` | Only show directories |
| `--hidden` | Show hidden files |
| `--allowed` | Filter by extensions |
| `--cursor` | Cursor character |
| `--height` | Picker height |

### Examples

```bash
# Pick a file
FILE=$(invowk tui file)
echo "Selected: $FILE"

# Only directories
DIR=$(invowk tui file --directory)

# Show hidden files
FILE=$(invowk tui file --hidden)

# Filter by extension
FILE=$(invowk tui file --allowed ".go,.md,.txt")

# Multiple extensions
CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json,.toml")
```

### Navigation

In the file picker:
- **↑/↓**: Navigate
- **Enter**: Select file or enter directory
- **Backspace**: Go to parent directory
- **Esc/Ctrl+C**: Cancel

### Real-World Examples

#### Select Config File

```bash
CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json" /etc/myapp)
echo "Using config: $CONFIG"
./myapp --config "$CONFIG"
```

#### Select Project Directory

```bash
PROJECT=$(invowk tui file --directory ~/projects)
cd "$PROJECT"
code .
```

#### Select Script to Run

```bash
SCRIPT=$(invowk tui file --allowed ".sh,.bash" ./scripts)
if [ -n "$SCRIPT" ]; then
    chmod +x "$SCRIPT"
    "$SCRIPT"
fi
```

#### Select Log File

```bash
LOG=$(invowk tui file --allowed ".log" /var/log)
less "$LOG"
```

### In Scripts

```cue
{
    name: "edit-config"
    description: "Edit a configuration file"
    implementations: [{
        script: """
            CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json,.toml" ./config)
            
            if [ -z "$CONFIG" ]; then
                echo "No file selected."
                exit 0
            fi
            
            # Open in default editor
            ${EDITOR:-vim} "$CONFIG"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Combined Patterns

### Search Then Edit

```bash
# Find file by content, then pick from results
FILE=$(grep -l "TODO" *.go 2>/dev/null | invowk tui filter --title "Select file with TODOs")
if [ -n "$FILE" ]; then
    vim "$FILE"
fi
```

### Directory Then File

```bash
# First pick directory
DIR=$(invowk tui file --directory ~/projects)

# Then pick file in that directory
FILE=$(invowk tui file "$DIR" --allowed ".go")

echo "Selected: $FILE"
```

## Next Steps

- [Table and Spin](./table-and-spin) - Data display and spinners
- [Overview](./overview) - All TUI components
