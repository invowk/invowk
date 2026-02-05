# Flowcharts

D2 excels at flowcharts with its intuitive syntax, extensive shape library, and powerful layout engines. TALA provides superior auto-arrangement with grid support.

## Basic Syntax

```d2
direction: down

start: Start
process: Process data
end: End

start -> process -> end
```

**Key principles:**
- Direction controls flow: `up`, `down`, `right`, `left`
- Nodes are created by naming them
- Connections use arrows: `->`
- Default shape is rectangle

## Direction Control

```d2
direction: down   # Top to bottom (default for flowcharts)
direction: right  # Left to right
direction: up     # Bottom to top
direction: left   # Right to left
```

**Example:**

```d2
direction: right

a: Step 1
b: Step 2
c: Step 3

a -> b -> c
```

## Node Shapes

### Basic Shapes

```d2
# Rectangle (default)
rect: Process Step

# Oval (start/end)
start: Start {
  shape: oval
}

# Diamond (decision)
decision: Valid? {
  shape: diamond
}

# Cylinder (database/storage)
db: Database {
  shape: cylinder
}

# Circle
node: Junction {
  shape: circle
}

# Parallelogram (input/output)
input: User Input {
  shape: parallelogram
}
```

### All Available Shapes

```d2
shapes: {
  rectangle: Rectangle (default)
  square: Square { shape: square }
  oval: Oval { shape: oval }
  circle: Circle { shape: circle }
  diamond: Diamond { shape: diamond }
  hexagon: Hexagon { shape: hexagon }
  parallelogram: I/O { shape: parallelogram }
  cylinder: Storage { shape: cylinder }
  queue: Queue { shape: queue }
  package: Package { shape: package }
  step: Step { shape: step }
  page: Document { shape: page }
  document: Document { shape: document }
  cloud: Cloud { shape: cloud }
  person: Actor { shape: person }
  callout: Note { shape: callout }
  stored_data: Stored Data { shape: stored_data }
}
```

## Connections

### Basic Arrows

```d2
a -> b      # Directed
a <- b      # Reverse direction
a <-> b     # Bidirectional
a -- b      # Undirected (no arrow)
```

### Connection Labels

```d2
a -> b: Yes
c -> d: |md
  **HTTP POST**
  /api/users
|
```

### Connection Styling

```d2
a -> b: Normal
c -> d: Dashed {
  style.stroke-dash: 5
}
e -> f: Thick {
  style.stroke-width: 3
}
g -> h: Colored {
  style.stroke: "#ff0000"
}
```

### Multiple Connections

```d2
# Chain connections
a -> b -> c -> d

# Fan out
source -> target1
source -> target2
source -> target3

# Fan in
input1 -> destination
input2 -> destination
input3 -> destination
```

## Containers (Subgraphs)

Group related nodes:

```d2
direction: down

processing: Data Processing {
  extract: Extract
  transform: Transform
  load: Load

  extract -> transform -> load
}

start: Start
end: End

start -> processing.extract
processing.load -> end
```

### Nested Containers

```d2
direction: down

outer: System {
  middle: Subsystem {
    inner: Component {
      core: Core Logic
    }
  }
}
```

### Container Styling

```d2
backend: Backend Services {
  style.fill: "#e8f4f8"
  style.stroke: "#2196F3"
  style.stroke-dash: 3

  api: API Server
  worker: Worker
}
```

## Grid Layout (TALA)

TALA's grid feature creates aligned layouts:

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

grid-rows: 2
grid-columns: 3

a: Cell 1
b: Cell 2
c: Cell 3
d: Cell 4
e: Cell 5
f: Cell 6
```

### Grid with Containers

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

services: Microservices {
  grid-columns: 3

  auth: Auth Service
  users: User Service
  orders: Order Service
  products: Product Service
  payments: Payment Service
  notifications: Notification Service
}
```

## Decision Flow Pattern

```d2
direction: down

start: Start {
  shape: oval
}

input: Get Input

validate: Valid? {
  shape: diamond
}

process: Process Data

error: Show Error

end: End {
  shape: oval
}

start -> input -> validate
validate -> process: Yes
validate -> error: No
error -> input
process -> end
```

## Comprehensive Example: User Registration

```d2
vars: {
  d2-config: {
    layout-engine: tala
    tala-seeds: 42
  }
}

direction: down

# Start
start: User visits registration {
  shape: oval
  style.fill: "#90EE90"
}

# Form
form: Show registration form
input: User enters details

# Validation
validate_input: Valid input? {
  shape: diamond
}
show_validation_error: Show validation errors

# Email check
check_email: Email exists? {
  shape: diamond
}
show_email_error: Show email error

# Account creation
create: Create Account {
  style.fill: "#87CEEB"

  hash: Hash password
  save: Save to database {
    shape: cylinder
  }
  token: Generate verification token
}

# Notifications
send_email: Send verification email
success: Show success message

# End
redirect: Redirect to login {
  shape: oval
  style.fill: "#90EE90"
}

# Flow
start -> form -> input -> validate_input

validate_input -> show_validation_error: No
show_validation_error -> form

validate_input -> check_email: Yes
check_email -> show_email_error: Yes
show_email_error -> form

check_email -> create.hash: No
create.hash -> create.save -> create.token

create.token -> send_email -> success -> redirect
```

## Algorithm Example: Binary Search

```d2
vars: {
  d2-config: {
    layout-engine: tala
    tala-seeds: 42
  }
}

direction: down

start: Binary Search {
  shape: oval
  style.fill: "#4CAF50"
  style.font-color: "#ffffff"
}

init: |md
  low = 0
  high = length - 1
|

check_bounds: low <= high? {
  shape: diamond
}

calc_mid: |md
  mid = low + (high - low) / 2
|

compare: arr[mid] == target? {
  shape: diamond
}

found: Return mid {
  shape: oval
  style.fill: "#4CAF50"
  style.font-color: "#ffffff"
}

not_found: Return -1 {
  shape: oval
  style.fill: "#f44336"
  style.font-color: "#ffffff"
}

check_less: arr[mid] < target? {
  shape: diamond
}

move_low: low = mid + 1
move_high: high = mid - 1

# Flow
start -> init -> check_bounds

check_bounds -> not_found: No
check_bounds -> calc_mid: Yes

calc_mid -> compare
compare -> found: Yes
compare -> check_less: No

check_less -> move_low: Yes
check_less -> move_high: No

move_low -> check_bounds
move_high -> check_bounds
```

## CI/CD Pipeline

```d2
vars: {
  d2-config: {
    layout-engine: tala
    tala-seeds: 42
  }
}

direction: right

# Development
dev: Development {
  style.fill: "#e3f2fd"

  commit: Developer commits
  push: Push to repo
  commit -> push
}

# CI
ci: Continuous Integration {
  style.fill: "#fff3e0"

  direction: down
  trigger: Trigger pipeline
  checkout: Checkout code
  install: Install deps
  lint: Run linters
  test: Run tests
  build: Build app

  trigger -> checkout -> install -> lint -> test -> build
}

# QA
qa: QA Environment {
  style.fill: "#f3e5f5"

  deploy_staging: Deploy to staging
  e2e: E2E tests
  approval: Manual approval? {
    shape: diamond
  }
}

# Production
prod: Production {
  style.fill: "#e8f5e9"

  deploy_prod: Deploy to production
  health: Health check? {
    shape: diamond
  }
  success: Success {
    shape: oval
    style.fill: "#4CAF50"
  }
  rollback: Rollback
}

# Connections
dev.push -> ci.trigger
ci.build -> qa.deploy_staging
qa.deploy_staging -> qa.e2e -> qa.approval

qa.approval -> prod.deploy_prod: Approved
qa.approval -> dev.commit: Rejected {
  style.stroke-dash: 5
}

prod.deploy_prod -> prod.health
prod.health -> prod.success: Pass
prod.health -> prod.rollback: Fail
```

## Error Handling Pattern

```d2
direction: down

try: Try Operation

check: Success? {
  shape: diamond
}

continue: Continue Processing

handle: Handle Error

retry_check: Retry? {
  shape: diamond
}

abort: Abort {
  shape: oval
  style.fill: "#f44336"
  style.font-color: "#ffffff"
}

increment: Increment retry count

try -> check
check -> continue: Yes
check -> handle: No

handle -> retry_check
retry_check -> increment: Yes
retry_check -> abort: No

increment -> try
```

## Loop Pattern

```d2
direction: down

init: Initialize

process: Process Item

more: More items? {
  shape: diamond
}

next: Get Next Item

done: Done {
  shape: oval
}

init -> process -> more
more -> next: Yes
more -> done: No
next -> process
```

## State Machine

```d2
direction: right

idle: Idle {
  style.fill: "#e0e0e0"
}

loading: Loading {
  style.fill: "#fff9c4"
}

success: Success {
  style.fill: "#c8e6c9"
}

error: Error {
  style.fill: "#ffcdd2"
}

# Transitions
idle -> loading: fetch()
loading -> success: data received
loading -> error: request failed
success -> idle: reset()
error -> loading: retry()
error -> idle: dismiss()
```

## Best Practices

1. **Set direction explicitly** - `direction: down` for traditional flowcharts
2. **Use semantic shapes** - Diamond for decisions, oval for start/end
3. **Group related steps** - Use containers for logical groupings
4. **Style consistently** - Same colors for same types of nodes
5. **Label all decisions** - Every diamond branch needs a label
6. **Keep it focused** - One process per diagram
7. **Use TALA for complex flows** - Better auto-layout than dagre
8. **Add seeds for reproducibility** - Consistent rendering in CI

## Shape Selection Guide

| Purpose | Shape | Example |
|---------|-------|---------|
| Start/End | `oval` | Entry and exit points |
| Process | `rectangle` | Actions and operations |
| Decision | `diamond` | Yes/No branches |
| Data store | `cylinder` | Databases |
| Manual input | `parallelogram` | User input |
| Document | `document` | Reports, logs |
| Predefined process | `step` | Subroutines |
| External entity | `cloud` | External systems |
| User/Actor | `person` | Human participants |
