import type { Snippet } from '../snippets';

export const tuiSnippets = {
  // =============================================================================
  // TUI COMPONENTS
  // =============================================================================

  'tui/input': {
    language: 'bash',
    code: `NAME=$(invowk tui input --title "What's your name?")
echo "Hello, $NAME!"`,
  },

  'tui/choose': {
    language: 'bash',
    code: `COLOR=$(invowk tui choose --title "Pick a color" red green blue)
echo "You picked: $COLOR"`,
  },

  'tui/confirm': {
    language: 'bash',
    code: `if invowk tui confirm --title "Are you sure?"; then
    echo "Proceeding..."
else
    echo "Cancelled"
fi`,
  },

  'tui/spin': {
    language: 'bash',
    code: `invowk tui spin --title "Installing dependencies..." -- npm install`,
  },

  'tui/filter': {
    language: 'bash',
    code: `SELECTED=$(ls | invowk tui filter --title "Select a file")`,
  },

  // =============================================================================
  // TUI - ADDITIONAL
  // =============================================================================

  'tui/input-options': {
    language: 'bash',
    code: `# With placeholder
invowk tui input --title "Email" --placeholder "user@example.com"

# Password input (hidden)
invowk tui input --title "Password" --password

# With initial value
invowk tui input --title "Name" --value "John Doe"

# Limited length
invowk tui input --title "Username" --char-limit 20`,
  },

  'tui/input-in-script': {
    language: 'cue',
    code: `{
    name: "create-user"
    implementations: [{
        script: """
            USERNAME=$(invowk tui input --title "Username:" --char-limit 20)
            EMAIL=$(invowk tui input --title "Email:" --placeholder "user@example.com")
            PASSWORD=$(invowk tui input --title "Password:" --password)
            
            echo "Creating user: $USERNAME ($EMAIL)"
            ./scripts/create-user.sh "$USERNAME" "$EMAIL" "$PASSWORD"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'tui/write-basic': {
    language: 'bash',
    code: `invowk tui write --title "Enter description"`,
  },

  'tui/write-options': {
    language: 'bash',
    code: `# Basic editor
invowk tui write --title "Description:"

# With line numbers
invowk tui write --title "Code:" --show-line-numbers

# With initial content
invowk tui write --title "Edit message:" --value "Initial text here"`,
  },

  'tui/write-commit': {
    language: 'cue',
    code: `{
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'tui/empty-validation': {
    language: 'bash',
    code: `NAME=$(invowk tui input --title "Name:")
if [ -z "$NAME" ]; then
    echo "Name is required!"
    exit 1
fi`,
  },

  'tui/validation-loop': {
    language: 'bash',
    code: `while true; do
    EMAIL=$(invowk tui input --title "Email:")
    if echo "$EMAIL" | grep -qE '^[^@]+@[^@]+\.[^@]+$'; then
        break
    fi
    echo "Invalid email format, try again."
done`,
  },

  // Overview page
  'tui/overview-quick-examples': {
    language: 'bash',
    code: `# Get user input
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
echo "Success!" | invowk tui style --foreground "#00FF00" --bold`,
  },

  'tui/overview-invowkfile-example': {
    language: 'cue',
    code: `{
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'tui/overview-input-validation': {
    language: 'bash',
    code: `while true; do
    EMAIL=$(invowk tui input --title "Email address:")
    if echo "$EMAIL" | grep -qE '^[^@]+@[^@]+\.[^@]+$'; then
        break
    fi
    echo "Invalid email format" | invowk tui style --foreground "#FF0000"
done`,
  },

  'tui/overview-menu-system': {
    language: 'bash',
    code: `ACTION=$(invowk tui choose --title "What would you like to do?" 
    "Build project" 
    "Run tests" 
    "Deploy" 
    "Exit")

case "$ACTION" in
    "Build project") make build ;;
    "Run tests") make test ;;
    "Deploy") make deploy ;;
    "Exit") exit 0 ;;
esac`,
  },

  'tui/overview-progress-feedback': {
    language: 'bash',
    code: `echo "Step 1: Installing dependencies..."
invowk tui spin --title "Installing..." -- npm install

echo "Step 2: Building..."
invowk tui spin --title "Building..." -- npm run build

echo "Done!" | invowk tui style --foreground "#00FF00" --bold`,
  },

  'tui/overview-styled-headers': {
    language: 'bash',
    code: `invowk tui style --bold --foreground "#00BFFF" "=== Project Setup ==="
echo ""
# ... rest of script`,
  },

  'tui/overview-piping': {
    language: 'bash',
    code: `# Pipe to filter
ls | invowk tui filter --title "Select file"

# Capture output
SELECTED=$(invowk tui choose opt1 opt2 opt3)
echo "You selected: $SELECTED"

# Pipe for styling
echo "Important message" | invowk tui style --bold`,
  },

  'tui/overview-exit-codes': {
    language: 'bash',
    code: `if invowk tui confirm "Delete files?"; then
    rm -rf ./temp
fi`,
  },

  // Input component
  'tui/input-basic': {
    language: 'bash',
    code: `invowk tui input --title "What is your name?"`,
  },

  'tui/input-examples': {
    language: 'bash',
    code: `# With placeholder
invowk tui input --title "Email" --placeholder "user@example.com"

# Password input (hidden)
invowk tui input --title "Password" --password

# With initial value
invowk tui input --title "Name" --value "John Doe"

# Limited length
invowk tui input --title "Username" --char-limit 20`,
  },

  'tui/input-capture': {
    language: 'bash',
    code: `NAME=$(invowk tui input --title "Enter your name:")
echo "Hello, $NAME!"`,
  },

  'tui/input-script': {
    language: 'cue',
    code: `{
    name: "create-user"
    implementations: [{
        script: """
            USERNAME=$(invowk tui input --title "Username:" --char-limit 20)
            EMAIL=$(invowk tui input --title "Email:" --placeholder "user@example.com")
            PASSWORD=$(invowk tui input --title "Password:" --password)
            
            echo "Creating user: $USERNAME ($EMAIL)"
            ./scripts/create-user.sh "$USERNAME" "$EMAIL" "$PASSWORD"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  // Write component
  'tui/write-examples': {
    language: 'bash',
    code: `# Basic editor
invowk tui write --title "Description:"

# With line numbers
invowk tui write --title "Code:" --show-line-numbers

# With initial content
invowk tui write --title "Edit message:" --value "Initial text here"`,
  },

  'tui/write-git-commit': {
    language: 'bash',
    code: `MESSAGE=$(invowk tui write --title "Commit message:")
git commit -m "$MESSAGE"`,
  },

  'tui/write-yaml-config': {
    language: 'bash',
    code: `CONFIG=$(invowk tui write --title "Enter YAML config:" --show-line-numbers)
echo "$CONFIG" > config.yaml`,
  },

  'tui/write-release-notes': {
    language: 'bash',
    code: `NOTES=$(invowk tui write --title "Release notes:")
gh release create v1.0.0 --notes "$NOTES"`,
  },

  'tui/write-script': {
    language: 'cue',
    code: `{
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'tui/input-empty-handling': {
    language: 'bash',
    code: `NAME=$(invowk tui input --title "Name:")
if [ -z "$NAME" ]; then
    echo "Name is required!"
    exit 1
fi`,
  },

  'tui/input-validation': {
    language: 'bash',
    code: `while true; do
    EMAIL=$(invowk tui input --title "Email:")
    if echo "$EMAIL" | grep -qE '^[^@]+@[^@]+\.[^@]+$'; then
        break
    fi
    echo "Invalid email format, try again."
done`,
  },

  'tui/input-default-value': {
    language: 'bash',
    code: `# Use shell default if empty
NAME=$(invowk tui input --title "Name:" --placeholder "Anonymous")
NAME="\${NAME:-Anonymous}"`,
  },

  // Choose component
  'tui/choose-basic': {
    language: 'bash',
    code: `invowk tui choose "Option 1" "Option 2" "Option 3"`,
  },

  'tui/choose-single': {
    language: 'bash',
    code: `# Basic
COLOR=$(invowk tui choose red green blue)
echo "You chose: $COLOR"

# With title
ENV=$(invowk tui choose --title "Select environment" dev staging prod)`,
  },

  'tui/choose-multiple': {
    language: 'bash',
    code: `# Limited multi-select (up to 3)
ITEMS=$(invowk tui choose --limit 3 "One" "Two" "Three" "Four" "Five")

# Unlimited multi-select
ITEMS=$(invowk tui choose --no-limit "One" "Two" "Three" "Four" "Five")`,
  },

  'tui/choose-multi-process': {
    language: 'bash',
    code: `SERVICES=$(invowk tui choose --no-limit --title "Select services to deploy" 
    api web worker scheduler)

echo "$SERVICES" | while read -r service; do
    echo "Deploying: $service"
done`,
  },

  'tui/choose-env-selection': {
    language: 'bash',
    code: `ENV=$(invowk tui choose --title "Deploy to which environment?" 
    development staging production)

case "$ENV" in
    production)
        if ! invowk tui confirm "Are you sure? This is PRODUCTION!"; then
            exit 1
        fi
        ;;
esac

./deploy.sh "$ENV"`,
  },

  'tui/choose-service-selection': {
    language: 'bash',
    code: `SERVICES=$(invowk tui choose --no-limit --title "Which services?" 
    api web worker cron)

for service in $SERVICES; do
    echo "Restarting $service..."
    systemctl restart "$service"
done`,
  },

  // Confirm component
  'tui/confirm-basic': {
    language: 'bash',
    code: `invowk tui confirm "Are you sure?"`,
  },

  'tui/confirm-examples': {
    language: 'bash',
    code: `# Basic confirmation
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
fi`,
  },

  'tui/confirm-conditional': {
    language: 'bash',
    code: `# Simple pattern
invowk tui confirm "Run tests?" && npm test

# Negation
invowk tui confirm "Skip build?" || npm run build`,
  },

  'tui/confirm-dangerous': {
    language: 'bash',
    code: `# Double confirmation for dangerous actions
if invowk tui confirm "Delete production database?"; then
    echo "This cannot be undone!" | invowk tui style --foreground "#FF0000" --bold
    if invowk tui confirm --affirmative "YES, DELETE IT" --negative "No, abort" "Type to confirm:"; then
        ./scripts/delete-production-db.sh
    fi
fi`,
  },

  'tui/confirm-script': {
    language: 'cue',
    code: `{
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'tui/choose-confirm-combined': {
    language: 'bash',
    code: `ACTION=$(invowk tui choose --title "Select action" 
    "Deploy to staging" 
    "Deploy to production" 
    "Rollback" 
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

echo "Executing: $ACTION"`,
  },

  'tui/multistep-wizard': {
    language: 'bash',
    code: `# Step 1: Choose action
ACTION=$(invowk tui choose --title "What would you like to do?" 
    "Create new project" 
    "Import existing" 
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
fi`,
  },

  // Filter component
  'tui/filter-basic': {
    language: 'bash',
    code: `# From arguments
invowk tui filter "apple" "banana" "cherry" "date"

# From stdin
ls | invowk tui filter`,
  },

  'tui/filter-examples': {
    language: 'bash',
    code: `# Filter files
FILE=$(ls | invowk tui filter --title "Select file")

# Multi-select filter
FILES=$(ls *.go | invowk tui filter --no-limit --title "Select Go files")

# With placeholder
ITEM=$(invowk tui filter --placeholder "Type to search..." opt1 opt2 opt3)`,
  },

  'tui/filter-git-branch': {
    language: 'bash',
    code: `BRANCH=$(git branch --list | tr -d '* ' | invowk tui filter --title "Checkout branch")
git checkout "$BRANCH"`,
  },

  'tui/filter-docker-container': {
    language: 'bash',
    code: `CONTAINER=$(docker ps --format "{{.Names}}" | invowk tui filter --title "Select container")
docker logs -f "$CONTAINER"`,
  },

  'tui/filter-kill-process': {
    language: 'bash',
    code: `PID=$(ps aux | invowk tui filter --title "Select process" | awk '{print $2}')
if [ -n "$PID" ]; then
    kill "$PID"
fi`,
  },

  'tui/filter-commands': {
    language: 'bash',
    code: `CMD=$(invowk cmd 2>/dev/null | grep "^  " | invowk tui filter --title "Run command")
# Extract command name and run it`,
  },

  // File component
  'tui/file-basic': {
    language: 'bash',
    code: `# Pick any file from current directory
invowk tui file

# Start in specific directory
invowk tui file /home/user/documents`,
  },

  'tui/file-examples': {
    language: 'bash',
    code: `# Pick a file
FILE=$(invowk tui file)
echo "Selected: $FILE"

# Only directories
DIR=$(invowk tui file --directory)

# Show hidden files
FILE=$(invowk tui file --hidden)

# Filter by extension
FILE=$(invowk tui file --allowed ".go,.md,.txt")

# Multiple extensions
CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json,.toml")`,
  },

  'tui/file-config': {
    language: 'bash',
    code: `CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json" /etc/myapp)
echo "Using config: $CONFIG"
./myapp --config "$CONFIG"`,
  },

  'tui/file-project-dir': {
    language: 'bash',
    code: `PROJECT=$(invowk tui file --directory ~/projects)
cd "$PROJECT"
code .`,
  },

  'tui/file-script-run': {
    language: 'bash',
    code: `SCRIPT=$(invowk tui file --allowed ".sh,.bash" ./scripts)
if [ -n "$SCRIPT" ]; then
    chmod +x "$SCRIPT"
    "$SCRIPT"
fi`,
  },

  'tui/file-log': {
    language: 'bash',
    code: `LOG=$(invowk tui file --allowed ".log" /var/log)
less "$LOG"`,
  },

  'tui/file-script': {
    language: 'cue',
    code: `{
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
            \${EDITOR:-vim} "$CONFIG"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'tui/filter-file-search-edit': {
    language: 'bash',
    code: `# Find file by content, then pick from results
FILE=$(grep -l "TODO" *.go 2>/dev/null | invowk tui filter --title "Select file with TODOs")
if [ -n "$FILE" ]; then
    vim "$FILE"
fi`,
  },

  'tui/filter-file-dir-then-file': {
    language: 'bash',
    code: `# First pick directory
DIR=$(invowk tui file --directory ~/projects)

# Then pick file in that directory
FILE=$(invowk tui file "$DIR" --allowed ".go")

echo "Selected: $FILE"`,
  },

  // Table component
  'tui/table-basic': {
    language: 'bash',
    code: `# From a CSV file
invowk tui table --file data.csv

# From stdin with separator
echo -e "name|age|city
Alice|30|NYC
Bob|25|LA" | invowk tui table --separator "|"`,
  },

  'tui/table-examples': {
    language: 'bash',
    code: `# Display CSV
invowk tui table --file users.csv

# Custom separator (TSV)
invowk tui table --file data.tsv --separator $'	'

# Pipe-separated
cat data.txt | invowk tui table --separator "|"`,
  },

  'tui/table-selectable': {
    language: 'bash',
    code: `# Select a row
SELECTED=$(invowk tui table --file servers.csv --selectable)
echo "Selected: $SELECTED"`,
  },

  'tui/table-servers': {
    language: 'bash',
    code: `# servers.csv:
# name,ip,status
# web-1,10.0.0.1,running
# web-2,10.0.0.2,running
# db-1,10.0.0.3,stopped

invowk tui table --file servers.csv`,
  },

  'tui/table-ssh': {
    language: 'bash',
    code: `# Select a server
SERVER=$(cat servers.csv | invowk tui table --selectable | cut -d',' -f2)
ssh "user@$SERVER"`,
  },

  'tui/table-process': {
    language: 'bash',
    code: `ps aux --no-headers | awk '{print $1","$2","$11}' | 
    (echo "USER,PID,COMMAND"; cat) | 
    invowk tui table --selectable`,
  },

  // Spin component
  'tui/spin-basic': {
    language: 'bash',
    code: `invowk tui spin --title "Installing..." -- npm install`,
  },

  'tui/spin-types': {
    language: 'bash',
    code: `invowk tui spin --type globe --title "Downloading..." -- curl -O https://example.com/file
invowk tui spin --type moon --title "Building..." -- make build
invowk tui spin --type pulse --title "Testing..." -- npm test`,
  },

  'tui/spin-examples': {
    language: 'bash',
    code: `# Basic spinner
invowk tui spin --title "Building..." -- go build ./...

# With specific type
invowk tui spin --type dot --title "Installing dependencies..." -- npm install

# Long-running task
invowk tui spin --title "Compiling assets..." -- webpack --mode production`,
  },

  'tui/spin-chained': {
    language: 'bash',
    code: `echo "Step 1/3: Dependencies"
invowk tui spin --title "Installing..." -- npm install

echo "Step 2/3: Build"
invowk tui spin --title "Building..." -- npm run build

echo "Step 3/3: Tests"
invowk tui spin --title "Testing..." -- npm test

echo "Done!" | invowk tui style --foreground "#00FF00" --bold`,
  },

  'tui/spin-exit-code': {
    language: 'bash',
    code: `if invowk tui spin --title "Testing..." -- npm test; then
    echo "Tests passed!"
else
    echo "Tests failed!"
    exit 1
fi`,
  },

  'tui/spin-script': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy with progress indication"
    implementations: [{
        script: """
            echo "Deploying application..."
            
            invowk tui spin --title "Building Docker image..." -- 
                docker build -t myapp .
            
            invowk tui spin --title "Pushing to registry..." -- 
                docker push myapp
            
            invowk tui spin --title "Updating Kubernetes..." -- 
                kubectl rollout restart deployment/myapp
            
            invowk tui spin --title "Waiting for rollout..." -- 
                kubectl rollout status deployment/myapp
            
            echo "Deployment complete!" | invowk tui style --foreground "#00FF00" --bold
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'tui/spin-select-execute': {
    language: 'bash',
    code: `# Choose what to build
PROJECT=$(invowk tui choose --title "Build which project?" api web worker)

# Build with spinner
invowk tui spin --title "Building $PROJECT..." -- make "build-$PROJECT"`,
  },

  'tui/table-spin-combined': {
    language: 'bash',
    code: `# Select server
SERVER=$(invowk tui table --file servers.csv --selectable | cut -d',' -f1)

# Restart with spinner
invowk tui spin --title "Restarting $SERVER..." -- ssh "$SERVER" "systemctl restart myapp"`,
  },

  // Format component
  'tui/format-basic': {
    language: 'bash',
    code: `echo "# Hello World" | invowk tui format --type markdown`,
  },

  'tui/format-markdown': {
    language: 'bash',
    code: `# From stdin
echo "# Heading

Some **bold** and *italic* text" | invowk tui format --type markdown

# From file
cat README.md | invowk tui format --type markdown`,
  },

  'tui/format-code': {
    language: 'bash',
    code: `# Specify language
cat main.go | invowk tui format --type code --language go

# Python
cat script.py | invowk tui format --type code --language python

# JavaScript
cat app.js | invowk tui format --type code --language javascript`,
  },

  'tui/format-emoji': {
    language: 'bash',
    code: `echo "Hello :wave: World :smile:" | invowk tui format --type emoji
# Output: Hello \ud83d\udc4b World \ud83d\ude04`,
  },

  'tui/format-readme': {
    language: 'bash',
    code: `cat README.md | invowk tui format --type markdown`,
  },

  'tui/format-diff': {
    language: 'bash',
    code: `git diff | invowk tui format --type code --language diff`,
  },

  'tui/format-welcome': {
    language: 'bash',
    code: `echo ":rocket: Welcome to MyApp :sparkles:" | invowk tui format --type emoji`,
  },

  // Style component
  'tui/style-basic': {
    language: 'bash',
    code: `invowk tui style --foreground "#FF0000" "Red text"`,
  },

  'tui/style-colors': {
    language: 'bash',
    code: `# Hex colors
invowk tui style --foreground "#FF0000" "Red"
invowk tui style --foreground "#00FF00" "Green"
invowk tui style --foreground "#0000FF" "Blue"

# With background
invowk tui style --foreground "#FFFFFF" --background "#FF0000" "White on Red"`,
  },

  'tui/style-decorations': {
    language: 'bash',
    code: `# Bold
invowk tui style --bold "Bold text"

# Italic
invowk tui style --italic "Italic text"

# Combined
invowk tui style --bold --italic --underline "All decorations"

# Dimmed
invowk tui style --faint "Subtle text"`,
  },

  'tui/style-piping': {
    language: 'bash',
    code: `echo "Important message" | invowk tui style --bold --foreground "#FF0000"`,
  },

  'tui/style-borders': {
    language: 'bash',
    code: `# Simple border
invowk tui style --border normal "Boxed text"

# Rounded border
invowk tui style --border rounded "Rounded box"

# Double border
invowk tui style --border double "Double border"

# With padding
invowk tui style --border rounded --padding-left 2 --padding-right 2 "Padded"`,
  },

  'tui/style-layout': {
    language: 'bash',
    code: `# Fixed width
invowk tui style --width 40 --align center "Centered"

# With margins
invowk tui style --margin-left 4 "Indented text"

# Box with all options
invowk tui style 
    --border rounded 
    --foreground "#FFFFFF" 
    --background "#333333" 
    --padding-left 2 
    --padding-right 2 
    --width 50 
    --align center 
    "Styled Box"`,
  },

  'tui/style-messages': {
    language: 'bash',
    code: `# Success
echo "Build successful!" | invowk tui style --foreground "#00FF00" --bold

# Error
echo "Build failed!" | invowk tui style --foreground "#FF0000" --bold

# Warning
echo "Deprecated feature" | invowk tui style --foreground "#FFA500" --italic`,
  },

  'tui/style-headers': {
    language: 'bash',
    code: `# Main header
invowk tui style --bold --foreground "#00BFFF" "=== Project Setup ==="
echo ""

# Subheader
invowk tui style --foreground "#888888" "Configuration Options:"`,
  },

  'tui/style-boxes': {
    language: 'bash',
    code: `# Info box
invowk tui style 
    --border rounded 
    --foreground "#FFFFFF" 
    --background "#0066CC" 
    --padding-left 1 
    --padding-right 1 
    "Info: Server is running on port 3000"

# Warning box
invowk tui style 
    --border rounded 
    --foreground "#000000" 
    --background "#FFCC00" 
    --padding-left 1 
    --padding-right 1 
    "Warning: API key will expire soon"`,
  },

  'tui/style-script': {
    language: 'cue',
    code: `{
    name: "status"
    description: "Show system status"
    implementations: [{
        script: """
            invowk tui style --bold --foreground "#00BFFF" "System Status"
            echo ""
            
            # Check services
            if systemctl is-active nginx > /dev/null 2>&1; then
                echo "nginx: " | tr -d '
'
                invowk tui style --foreground "#00FF00" "running"
            else
                echo "nginx: " | tr -d '
'
                invowk tui style --foreground "#FF0000" "stopped"
            fi
            
            if systemctl is-active postgresql > /dev/null 2>&1; then
                echo "postgres: " | tr -d '
'
                invowk tui style --foreground "#00FF00" "running"
            else
                echo "postgres: " | tr -d '
'
                invowk tui style --foreground "#FF0000" "stopped"
            fi
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'tui/format-style-combined': {
    language: 'bash',
    code: `# Header
invowk tui style --bold --foreground "#FFD700" "Package Info"
echo ""

# Render package description as markdown
cat package.md | invowk tui format --type markdown`,
  },

  'tui/interactive-styled': {
    language: 'bash',
    code: `NAME=$(invowk tui input --title "Project name:")

if invowk tui confirm "Create $NAME?"; then
    invowk tui spin --title "Creating..." -- mkdir -p "$NAME"
    echo "" 
    invowk tui style --foreground "#00FF00" --bold "Created $NAME successfully!"
else
    invowk tui style --foreground "#FF0000" "Cancelled"
fi`,
  },

  // =============================================================================
  // TUI - PAGER COMPONENT
  // =============================================================================

  'tui/pager-basic': {
    language: 'bash',
    code: `# View a file
invowk tui pager README.md

# Pipe content
cat long-output.txt | invowk tui pager`,
  },

  'tui/pager-options': {
    language: 'bash',
    code: `# With title
invowk tui pager --title "Log Output" app.log

# Show line numbers
invowk tui pager --line-numbers main.go

# Soft wrap long lines
invowk tui pager --soft-wrap document.txt

# Combine options
git log | invowk tui pager --title "Git History" --line-numbers`,
  },

  'tui/pager-script': {
    language: 'cue',
    code: `{
    name: "view-logs"
    description: "View application logs interactively"
    implementations: [{
        script: """
            # Get recent logs and display in pager
            journalctl -u myapp --no-pager -n 500 | 
                invowk tui pager --title "Application Logs" --soft-wrap
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'tui/pager-code-review': {
    language: 'bash',
    code: `# Review code with line numbers
invowk tui pager --line-numbers --title "Code Review" src/main.go

# View diff output
git diff HEAD~5 | invowk tui pager --title "Recent Changes"`,
  },
};
