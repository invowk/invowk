---
sidebar_position: 3
---

# Choose and Confirm

Selection and confirmation components for user decisions.

## Choose

Select one or more options from a list.

### Basic Usage

```bash
invowk tui choose "Option 1" "Option 2" "Option 3"
```

### Options

| Option | Description |
|--------|-------------|
| `--title` | Selection prompt |
| `--limit` | Max selections (default: 1) |
| `--no-limit` | Unlimited selections |
| `--cursor` | Cursor character |
| `--selected` | Pre-selected items |

### Single Selection

```bash
# Basic
COLOR=$(invowk tui choose red green blue)
echo "You chose: $COLOR"

# With title
ENV=$(invowk tui choose --title "Select environment" dev staging prod)
```

### Multiple Selection

```bash
# Limited multi-select (up to 3)
ITEMS=$(invowk tui choose --limit 3 "One" "Two" "Three" "Four" "Five")

# Unlimited multi-select
ITEMS=$(invowk tui choose --no-limit "One" "Two" "Three" "Four" "Five")
```

Multiple selections are returned as newline-separated values:

```bash
SERVICES=$(invowk tui choose --no-limit --title "Select services to deploy" \
    api web worker scheduler)

echo "$SERVICES" | while read -r service; do
    echo "Deploying: $service"
done
```

### Pre-Selected Options

```bash
invowk tui choose --selected "Two" "One" "Two" "Three"
```

### Real-World Examples

#### Environment Selection

```bash
ENV=$(invowk tui choose --title "Deploy to which environment?" \
    development staging production)

case "$ENV" in
    production)
        if ! invowk tui confirm "Are you sure? This is PRODUCTION!"; then
            exit 1
        fi
        ;;
esac

./deploy.sh "$ENV"
```

#### Service Selection

```bash
SERVICES=$(invowk tui choose --no-limit --title "Which services?" \
    api web worker cron)

for service in $SERVICES; do
    echo "Restarting $service..."
    systemctl restart "$service"
done
```

---

## Confirm

Yes/no confirmation prompt.

### Basic Usage

```bash
invowk tui confirm "Are you sure?"
```

Returns:
- Exit code 0 if user confirms (yes)
- Exit code 1 if user declines (no)

### Options

| Option | Description |
|--------|-------------|
| `--affirmative` | Custom "yes" label |
| `--negative` | Custom "no" label |
| `--default` | Default to yes |

### Examples

```bash
# Basic confirmation
if invowk tui confirm "Continue?"; then
    echo "Continuing..."
else
    echo "Cancelled."
fi

# Custom labels
if invowk tui confirm --affirmative "Delete" --negative "Cancel" "Delete all files?"; then
    rm -rf ./temp/*
fi

# Default to yes (user just presses Enter)
if invowk tui confirm --default "Proceed with defaults?"; then
    echo "Using defaults..."
fi
```

### Conditional Execution

```bash
# Simple pattern
invowk tui confirm "Run tests?" && npm test

# Negation
invowk tui confirm "Skip build?" || npm run build
```

### Dangerous Operations

```bash
# Double confirmation for dangerous actions
if invowk tui confirm "Delete production database?"; then
    echo "This cannot be undone!" | invowk tui style --foreground "#FF0000" --bold
    if invowk tui confirm --affirmative "YES, DELETE IT" --negative "No, abort" "Type to confirm:"; then
        ./scripts/delete-production-db.sh
    fi
fi
```

### In Scripts

```cue
{
    name: "clean"
    description: "Clean build artifacts"
    implementations: [{
        script: """
            echo "This will delete:"
            echo "  - ./build/"
            echo "  - ./dist/"
            echo "  - ./node_modules/"
            
            if invowk tui confirm "Proceed with cleanup?"; then
                rm -rf build/ dist/ node_modules/
                echo "Cleaned!" | invowk tui style --foreground "#00FF00"
            else
                echo "Cancelled."
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Combined Patterns

### Selection with Confirmation

```bash
ACTION=$(invowk tui choose --title "Select action" \
    "Deploy to staging" \
    "Deploy to production" \
    "Rollback" \
    "Cancel")

case "$ACTION" in
    "Cancel")
        exit 0
        ;;
    "Deploy to production")
        if ! invowk tui confirm --affirmative "Yes, deploy" "Deploy to PRODUCTION?"; then
            echo "Aborted."
            exit 1
        fi
        ;;
esac

echo "Executing: $ACTION"
```

### Multi-Step Wizard

```bash
# Step 1: Choose action
ACTION=$(invowk tui choose --title "What would you like to do?" \
    "Create new project" \
    "Import existing" \
    "Exit")

if [ "$ACTION" = "Exit" ]; then
    exit 0
fi

# Step 2: Get details
NAME=$(invowk tui input --title "Project name:")

# Step 3: Confirm
echo "Action: $ACTION"
echo "Name: $NAME"

if invowk tui confirm "Create project?"; then
    # proceed
fi
```

## Next Steps

- [Filter and File](./filter-and-file) - Search and file picking
- [Overview](./overview) - All TUI components
