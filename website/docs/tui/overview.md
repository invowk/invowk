---
sidebar_position: 1
---

# TUI Components Overview

Invowk includes a set of interactive terminal UI components inspired by [gum](https://github.com/charmbracelet/gum). Use them in your scripts to create interactive prompts, selections, and styled output.

## Available Components

| Component | Description | Use Case |
|-----------|-------------|----------|
| [input](./input-and-write#input) | Single-line text input | Names, paths, simple values |
| [write](./input-and-write#write) | Multi-line text editor | Descriptions, commit messages |
| [choose](./choose-and-confirm#choose) | Select from options | Menus, choices |
| [confirm](./choose-and-confirm#confirm) | Yes/no prompt | Confirmations |
| [filter](./filter-and-file#filter) | Fuzzy filter list | Search through options |
| [file](./filter-and-file#file) | File picker | Select files/directories |
| [table](./table-and-spin#table) | Display tabular data | CSV, data tables |
| [spin](./table-and-spin#spin) | Spinner with command | Long-running tasks |
| [format](./format-and-style#format) | Format text (markdown, code) | Rendering content |
| [style](./format-and-style#style) | Style text (colors, bold) | Decorating output |

## Quick Examples

```bash
# Get user input
NAME=$(invowk tui input --title "What's your name?")

# Choose from options
COLOR=$(invowk tui choose --title "Pick a color" red green blue)

# Confirm action
if invowk tui confirm "Continue?"; then
    echo "Proceeding..."
fi

# Show spinner during long task
invowk tui spin --title "Installing..." -- npm install

# Style output
echo "Success!" | invowk tui style --foreground "#00FF00" --bold
```

## Using in Invkfiles

TUI components work great inside command scripts:

```cue
{
    name: "setup"
    description: "Interactive project setup"
    implementations: [{
        script: """
            #!/bin/bash
            
            # Gather information
            NAME=$(invowk tui input --title "Project name:")
            TYPE=$(invowk tui choose --title "Type:" cli library api)
            
            # Confirm
            echo "Creating $TYPE project: $NAME"
            if ! invowk tui confirm "Proceed?"; then
                echo "Cancelled."
                exit 0
            fi
            
            # Execute with spinner
            invowk tui spin --title "Creating project..." -- mkdir -p "$NAME"
            
            # Success message
            echo "Project created!" | invowk tui style --foreground "#00FF00" --bold
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Common Patterns

### Input with Validation

```bash
while true; do
    EMAIL=$(invowk tui input --title "Email address:")
    if echo "$EMAIL" | grep -qE '^[^@]+@[^@]+\.[^@]+$'; then
        break
    fi
    echo "Invalid email format" | invowk tui style --foreground "#FF0000"
done
```

### Menu System

```bash
ACTION=$(invowk tui choose --title "What would you like to do?" \
    "Build project" \
    "Run tests" \
    "Deploy" \
    "Exit")

case "$ACTION" in
    "Build project") make build ;;
    "Run tests") make test ;;
    "Deploy") make deploy ;;
    "Exit") exit 0 ;;
esac
```

### Progress Feedback

```bash
echo "Step 1: Installing dependencies..."
invowk tui spin --title "Installing..." -- npm install

echo "Step 2: Building..."
invowk tui spin --title "Building..." -- npm run build

echo "Done!" | invowk tui style --foreground "#00FF00" --bold
```

### Styled Headers

```bash
invowk tui style --bold --foreground "#00BFFF" "=== Project Setup ==="
echo ""
# ... rest of script
```

## Piping and Capture

Most components work with pipes:

```bash
# Pipe to filter
ls | invowk tui filter --title "Select file"

# Capture output
SELECTED=$(invowk tui choose opt1 opt2 opt3)
echo "You selected: $SELECTED"

# Pipe for styling
echo "Important message" | invowk tui style --bold
```

## Exit Codes

Components use exit codes to communicate:

| Component | Exit 0 | Exit 1 |
|-----------|--------|--------|
| confirm | User said yes | User said no |
| input | Value entered | Cancelled |
| choose | Option selected | Cancelled |
| filter | Option selected | Cancelled |

Use in conditionals:

```bash
if invowk tui confirm "Delete files?"; then
    rm -rf ./temp
fi
```

## Next Steps

Explore each component in detail:

- [Input and Write](./input-and-write) - Text entry
- [Choose and Confirm](./choose-and-confirm) - Selection and confirmation
- [Filter and File](./filter-and-file) - Search and file picking
- [Table and Spin](./table-and-spin) - Data display and spinners
- [Format and Style](./format-and-style) - Text formatting and styling
