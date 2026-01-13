---
sidebar_position: 6
---

# Format and Style

Text formatting and styling components for beautiful terminal output.

## Format

Format and render text as markdown, code, or emoji.

### Basic Usage

```bash
echo "# Hello World" | invowk tui format --type markdown
```

### Options

| Option | Description |
|--------|-------------|
| `--type` | Format type: `markdown`, `code`, `emoji` |
| `--language` | Language for code highlighting |

### Markdown

Render markdown with colors and formatting:

```bash
# From stdin
echo "# Heading\n\nSome **bold** and *italic* text" | invowk tui format --type markdown

# From file
cat README.md | invowk tui format --type markdown
```

### Code Highlighting

Syntax highlight code:

```bash
# Specify language
cat main.go | invowk tui format --type code --language go

# Python
cat script.py | invowk tui format --type code --language python

# JavaScript
cat app.js | invowk tui format --type code --language javascript
```

### Emoji Conversion

Convert emoji shortcodes to actual emojis:

```bash
echo "Hello :wave: World :smile:" | invowk tui format --type emoji
# Output: Hello 👋 World 😄
```

### Real-World Examples

#### Display README

```bash
cat README.md | invowk tui format --type markdown
```

#### Show Code Diff

```bash
git diff | invowk tui format --type code --language diff
```

#### Welcome Message

```bash
echo ":rocket: Welcome to MyApp :sparkles:" | invowk tui format --type emoji
```

---

## Style

Apply terminal styling to text.

### Basic Usage

```bash
invowk tui style --foreground "#FF0000" "Red text"
```

### Options

| Option | Description |
|--------|-------------|
| `--foreground` | Text color (hex or name) |
| `--background` | Background color |
| `--bold` | Bold text |
| `--italic` | Italic text |
| `--underline` | Underlined text |
| `--strikethrough` | Strikethrough text |
| `--faint` | Dimmed text |
| `--border` | Border style |
| `--padding-*` | Padding (left, right, top, bottom) |
| `--margin-*` | Margin (left, right, top, bottom) |
| `--width` | Fixed width |
| `--height` | Fixed height |
| `--align` | Text alignment: `left`, `center`, `right` |

### Colors

Use hex colors or names:

```bash
# Hex colors
invowk tui style --foreground "#FF0000" "Red"
invowk tui style --foreground "#00FF00" "Green"
invowk tui style --foreground "#0000FF" "Blue"

# With background
invowk tui style --foreground "#FFFFFF" --background "#FF0000" "White on Red"
```

### Text Decorations

```bash
# Bold
invowk tui style --bold "Bold text"

# Italic
invowk tui style --italic "Italic text"

# Combined
invowk tui style --bold --italic --underline "All decorations"

# Dimmed
invowk tui style --faint "Subtle text"
```

### Piping

Style text from stdin:

```bash
echo "Important message" | invowk tui style --bold --foreground "#FF0000"
```

### Borders

Add borders around text:

```bash
# Simple border
invowk tui style --border normal "Boxed text"

# Rounded border
invowk tui style --border rounded "Rounded box"

# Double border
invowk tui style --border double "Double border"

# With padding
invowk tui style --border rounded --padding-left 2 --padding-right 2 "Padded"
```

Border styles: `normal`, `rounded`, `double`, `thick`, `hidden`

### Layout

```bash
# Fixed width
invowk tui style --width 40 --align center "Centered"

# With margins
invowk tui style --margin-left 4 "Indented text"

# Box with all options
invowk tui style \
    --border rounded \
    --foreground "#FFFFFF" \
    --background "#333333" \
    --padding-left 2 \
    --padding-right 2 \
    --width 50 \
    --align center \
    "Styled Box"
```

### Real-World Examples

#### Success/Error Messages

```bash
# Success
echo "Build successful!" | invowk tui style --foreground "#00FF00" --bold

# Error
echo "Build failed!" | invowk tui style --foreground "#FF0000" --bold

# Warning
echo "Deprecated feature" | invowk tui style --foreground "#FFA500" --italic
```

#### Headers and Sections

```bash
# Main header
invowk tui style --bold --foreground "#00BFFF" "=== Project Setup ==="
echo ""

# Subheader
invowk tui style --foreground "#888888" "Configuration Options:"
```

#### Status Boxes

```bash
# Info box
invowk tui style \
    --border rounded \
    --foreground "#FFFFFF" \
    --background "#0066CC" \
    --padding-left 1 \
    --padding-right 1 \
    "ℹ️  Info: Server is running on port 3000"

# Warning box
invowk tui style \
    --border rounded \
    --foreground "#000000" \
    --background "#FFCC00" \
    --padding-left 1 \
    --padding-right 1 \
    "⚠️  Warning: API key will expire soon"
```

### In Scripts

```cue
{
    name: "status"
    description: "Show system status"
    implementations: [{
        script: """
            invowk tui style --bold --foreground "#00BFFF" "System Status"
            echo ""
            
            # Check services
            if systemctl is-active nginx > /dev/null 2>&1; then
                echo "nginx: " | tr -d '\n'
                invowk tui style --foreground "#00FF00" "running"
            else
                echo "nginx: " | tr -d '\n'
                invowk tui style --foreground "#FF0000" "stopped"
            fi
            
            if systemctl is-active postgresql > /dev/null 2>&1; then
                echo "postgres: " | tr -d '\n'
                invowk tui style --foreground "#00FF00" "running"
            else
                echo "postgres: " | tr -d '\n'
                invowk tui style --foreground "#FF0000" "stopped"
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Combined Patterns

### Formatted Output

```bash
# Header
invowk tui style --bold --foreground "#FFD700" "📦 Package Info"
echo ""

# Render package description as markdown
cat package.md | invowk tui format --type markdown
```

### Interactive with Styled Output

```bash
NAME=$(invowk tui input --title "Project name:")

if invowk tui confirm "Create $NAME?"; then
    invowk tui spin --title "Creating..." -- mkdir -p "$NAME"
    echo "" 
    invowk tui style --foreground "#00FF00" --bold "✓ Created $NAME successfully!"
else
    invowk tui style --foreground "#FF0000" "✗ Cancelled"
fi
```

## Next Steps

- [Overview](./overview) - All TUI components
- [Input and Write](./input-and-write) - Text entry
