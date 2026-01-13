---
sidebar_position: 2
---

# Input and Write

Text entry components for gathering user input.

## Input

Single-line text input with optional validation.

### Basic Usage

```bash
invowk tui input --title "What is your name?"
```

### Options

| Option | Description |
|--------|-------------|
| `--title` | Prompt text |
| `--placeholder` | Placeholder text |
| `--value` | Initial value |
| `--password` | Hide input (for secrets) |
| `--char-limit` | Maximum characters |

### Examples

```bash
# With placeholder
invowk tui input --title "Email" --placeholder "user@example.com"

# Password input (hidden)
invowk tui input --title "Password" --password

# With initial value
invowk tui input --title "Name" --value "John Doe"

# Limited length
invowk tui input --title "Username" --char-limit 20
```

### Capturing Output

```bash
NAME=$(invowk tui input --title "Enter your name:")
echo "Hello, $NAME!"
```

### In Scripts

```cue
{
    name: "create-user"
    implementations: [{
        script: """
            USERNAME=$(invowk tui input --title "Username:" --char-limit 20)
            EMAIL=$(invowk tui input --title "Email:" --placeholder "user@example.com")
            PASSWORD=$(invowk tui input --title "Password:" --password)
            
            echo "Creating user: $USERNAME ($EMAIL)"
            ./scripts/create-user.sh "$USERNAME" "$EMAIL" "$PASSWORD"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

---

## Write

Multi-line text editor for longer input like descriptions or commit messages.

### Basic Usage

```bash
invowk tui write --title "Enter description"
```

### Options

| Option | Description |
|--------|-------------|
| `--title` | Editor title |
| `--placeholder` | Placeholder text |
| `--value` | Initial content |
| `--show-line-numbers` | Display line numbers |
| `--char-limit` | Maximum characters |

### Examples

```bash
# Basic editor
invowk tui write --title "Description:"

# With line numbers
invowk tui write --title "Code:" --show-line-numbers

# With initial content
invowk tui write --title "Edit message:" --value "Initial text here"
```

### Use Cases

#### Git Commit Message

```bash
MESSAGE=$(invowk tui write --title "Commit message:")
git commit -m "$MESSAGE"
```

#### Multi-Line Configuration

```bash
CONFIG=$(invowk tui write --title "Enter YAML config:" --show-line-numbers)
echo "$CONFIG" > config.yaml
```

#### Release Notes

```bash
NOTES=$(invowk tui write --title "Release notes:")
gh release create v1.0.0 --notes "$NOTES"
```

### In Scripts

```cue
{
    name: "commit"
    description: "Interactive commit with editor"
    implementations: [{
        script: """
            # Show staged changes
            git diff --cached --stat
            
            # Get commit message
            MESSAGE=$(invowk tui write --title "Commit message:")
            
            if [ -z "$MESSAGE" ]; then
                echo "Commit cancelled (empty message)"
                exit 1
            fi
            
            git commit -m "$MESSAGE"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Tips

### Handling Empty Input

```bash
NAME=$(invowk tui input --title "Name:")
if [ -z "$NAME" ]; then
    echo "Name is required!"
    exit 1
fi
```

### Validation Loop

```bash
while true; do
    EMAIL=$(invowk tui input --title "Email:")
    if echo "$EMAIL" | grep -qE '^[^@]+@[^@]+\.[^@]+$'; then
        break
    fi
    echo "Invalid email format, try again."
done
```

### Default Values

```bash
# Use shell default if empty
NAME=$(invowk tui input --title "Name:" --placeholder "Anonymous")
NAME="${NAME:-Anonymous}"
```

## Next Steps

- [Choose and Confirm](./choose-and-confirm) - Selection components
- [Overview](./overview) - All TUI components
