# Sequence Diagrams

D2 supports sequence diagrams through a dedicated `shape: sequence_diagram` declaration. Sequence diagrams show temporal interactions between participants over time.

## Basic Syntax

```d2
shape: sequence_diagram

alice: Alice
bob: Bob

alice -> bob: Hello
bob -> alice: Hi there
```

**Key principles:**
- Declare `shape: sequence_diagram` at the top level
- Participants are auto-created when first referenced
- Messages flow in declaration order (top to bottom)
- Time flows downward

## Participants

### Implicit Declaration

Participants are created automatically when first used:

```d2
shape: sequence_diagram

# These are created implicitly
user -> frontend: Click login
frontend -> api: POST /auth
```

### Explicit Declaration with Labels

Declare participants explicitly for custom labels:

```d2
shape: sequence_diagram

# Explicit declarations
user: End User
fe: Frontend Application
api: Backend API
db: PostgreSQL Database

user -> fe: Enter credentials
fe -> api: Authenticate
api -> db: Query user
```

### Participant Ordering

Participants appear left-to-right in declaration order:

```d2
shape: sequence_diagram

# Order determines horizontal position
a: First
b: Second
c: Third
d: Fourth

a -> d: Spans all participants
```

## Message Types

### Solid Arrow (Synchronous Request)

```d2
shape: sequence_diagram

client -> server: Request
```

### Dashed Arrow (Response/Return)

```d2
shape: sequence_diagram

client -> server: Request
server --> client: Response
```

**Note:** Use `-->` for dashed arrows (returns/responses).

### Self-Messages

```d2
shape: sequence_diagram

service -> service: Process internally
```

### Bidirectional

```d2
shape: sequence_diagram

service1 <-> service2: Sync data
```

## Message Labels

### Simple Labels

```d2
shape: sequence_diagram

a -> b: Simple message
```

### Multi-line Labels

```d2
shape: sequence_diagram

a -> b: |md
  POST /api/users
  Content-Type: application/json
|
```

### Labels with Technical Details

```d2
shape: sequence_diagram

client -> server: GET /users\n[HTTPS]
server --> client: 200 OK\n[JSON]
```

## Spans and Groups

### Span (Grouping Messages)

Use spans to group related messages:

```d2
shape: sequence_diagram

user: User
auth: Auth Service
db: Database

user -> auth: Login request

auth.span: Authentication Flow {
  auth -> db: Check credentials
  db --> auth: User found
  auth -> auth: Generate token
}

auth --> user: JWT token
```

### Nested Spans

```d2
shape: sequence_diagram

client: Client
gateway: API Gateway
service: Service
cache: Cache
db: Database

client -> gateway: Request

gateway.span: Request Processing {
  gateway -> service: Forward request

  service.span: Data Retrieval {
    service -> cache: Check cache
    cache --> service: Cache miss
    service -> db: Query data
    db --> service: Data
    service -> cache: Update cache
  }

  service --> gateway: Response
}

gateway --> client: Final response
```

## Notes

### Note on Participant

```d2
shape: sequence_diagram

user: User
api: API

user -> api: Request

api.note: Validates JWT token and\nchecks permissions

api --> user: Response
```

### Note Spanning Multiple Participants

```d2
shape: sequence_diagram

frontend: Frontend
backend: Backend

frontend -> backend: HTTPS Request

# Note syntax for spanning
_.note: All communication is encrypted\nusing TLS 1.3 {
  near: frontend
}
```

## Comprehensive Example: User Authentication

```d2
shape: sequence_diagram

# Participants
user: User
browser: Browser
frontend: React App
gateway: API Gateway
auth: Auth Service
db: PostgreSQL
redis: Redis Cache
email: Email Service

# Initial request
user -> browser: Enter credentials
browser -> frontend: Submit form

frontend -> gateway: POST /api/auth/login\n[HTTPS/JSON]

gateway.span: Authentication {
  gateway -> auth: Authenticate user

  auth -> db: SELECT user WHERE email = ?
  db --> auth: User record

  auth.span: Credential Verification {
    auth -> auth: Verify password hash
    auth.note: Using bcrypt with cost=12
  }

  auth -> redis: Store session
  redis --> auth: OK

  auth --> gateway: JWT + Refresh token
}

gateway --> frontend: 200 OK\n{token, refreshToken}

frontend -> frontend: Store tokens
frontend --> browser: Redirect to dashboard
browser --> user: Show dashboard
```

## API Flow Example

```d2
shape: sequence_diagram

client: Mobile App
gateway: Kong Gateway
users: User Service
orders: Order Service
inventory: Inventory Service
payment: Payment Gateway
events: Kafka

client -> gateway: POST /orders\n[Bearer token]

gateway.span: Request Pipeline {
  gateway -> gateway: Validate JWT
  gateway -> gateway: Rate limit check
  gateway.note: 100 req/min per user
}

gateway -> orders: Create order

orders.span: Order Processing {
  orders -> users: Get user details
  users --> orders: User data

  orders -> inventory: Check stock
  inventory --> orders: Stock available

  orders -> inventory: Reserve items
  inventory --> orders: Reserved

  orders -> payment: Charge customer
  payment --> orders: Payment confirmed

  orders -> orders: Save order
}

orders -> events: Publish OrderCreated
events --> orders: ACK

orders --> gateway: 201 Created\n{orderId}
gateway --> client: Order confirmation
```

## Microservices Communication

```d2
shape: sequence_diagram

user: User
web: Web App
gateway: API Gateway
catalog: Catalog
cart: Cart Service
order: Order Service
payment: Payment
notification: Notifications
queue: Message Queue

user -> web: Add to cart
web -> gateway: POST /cart/items

gateway -> cart: Add item
cart --> gateway: Cart updated
gateway --> web: 200 OK
web --> user: Item added

user -> web: Checkout
web -> gateway: POST /checkout

gateway -> order: Create order

order.span: Order Fulfillment {
  order -> cart: Get cart items
  cart --> order: Items

  order -> payment: Process payment
  payment --> order: Payment ID

  order -> queue: Publish OrderPaid

  # Async processing
  queue -> notification: OrderPaid event
  notification -> user: Send confirmation email
}

order --> gateway: Order confirmed
gateway --> web: 200 OK\n{orderId}
web --> user: Order confirmation
```

## Parallel Processing

Show concurrent operations with multiple spans:

```d2
shape: sequence_diagram

api: API
service: Service
cache: Cache
db: Database
search: Search Index
analytics: Analytics

api -> service: Process request

# These happen in parallel
service -> cache: Update cache
service -> db: Persist data
service -> search: Index document
service -> analytics: Track event

# Responses come back
cache --> service: OK
db --> service: OK
search --> service: OK
analytics --> service: OK

service --> api: All operations complete
```

## Error Handling

```d2
shape: sequence_diagram

client: Client
api: API
service: Service
db: Database

client -> api: Request

api -> service: Process

service -> db: Query
db --> service: Connection timeout

service.span: Error Handling {
  service -> service: Log error
  service -> service: Increment retry counter
  service -> db: Retry query
  db --> service: Success
}

service --> api: Response
api --> client: 200 OK
```

## Best Practices

1. **Declare participants explicitly** - Improves readability and controls ordering
2. **Use spans for logical groups** - Makes complex flows easier to follow
3. **Add notes for business logic** - Explain non-obvious decisions
4. **Show error paths** - Document failure scenarios
5. **Label with protocols** - Include HTTP methods, status codes
6. **Keep diagrams focused** - One scenario per diagram
7. **Use dashed arrows for returns** - Visual distinction for responses
8. **Order participants by interaction** - Left-to-right follows the flow

## Comparison with Mermaid

| Feature | D2 | Mermaid |
|---------|-----|---------|
| Participant declaration | Implicit or explicit | `participant` keyword |
| Grouping | Spans with labels | `rect`, `loop`, `alt` |
| Message styling | Full styling support | Limited |
| Notes | `.note` on any element | `Note` keyword |
| Parallel flows | Multiple spans | `par` block |
| Layout control | TALA positioning | Automatic only |

## Common Patterns

### Request-Response

```d2
shape: sequence_diagram

client -> server: Request
server --> client: Response
```

### Fire and Forget (Async)

```d2
shape: sequence_diagram

publisher -> queue: Publish event
queue -> subscriber: Deliver event
```

### Saga Pattern

```d2
shape: sequence_diagram

orchestrator: Saga Orchestrator
service1: Service 1
service2: Service 2
service3: Service 3

orchestrator -> service1: Step 1
service1 --> orchestrator: Done

orchestrator -> service2: Step 2
service2 --> orchestrator: Failed

orchestrator.span: Compensation {
  orchestrator -> service1: Rollback Step 1
  service1 --> orchestrator: Rolled back
}

orchestrator --> orchestrator: Saga failed
```

## Common Pitfalls

### Incorrect Phase/Group Syntax

**Problem:** Using `Participant."Label": { }` syntax to group messages by phase causes all arrows to route back to the first participant.

```d2
# ❌ WRONG - Creates a scoped element on User, breaks arrow routing
shape: sequence_diagram

User
CLI
Config

User."1. Initialization": {
  CLI -> Config: Load configuration    # Arrow incorrectly routes to User
  Config -> CLI: Done                  # Also routes to User
}
```

**Why it fails:** D2 interprets `Participant."Label": { }` as creating a nested scope element on that participant's lifeline. All connections inside the block become scoped to that participant rather than connecting different participants.

**Fix:** Use flat structure with comments for phase grouping, or use proper `.span:` syntax on the originating participant:

```d2
# ✅ CORRECT - Flat structure with comments
shape: sequence_diagram

User
CLI
Config

# 1. Initialization Phase
CLI -> Config: Load configuration
Config -> CLI: Done
```

```d2
# ✅ CORRECT - Proper span syntax on originating participant
shape: sequence_diagram

CLI
Config
Database

CLI.span: Initialization {
  CLI -> Config: Load configuration
  Config -> Database: Fetch defaults
  Database -> Config: Defaults
  Config -> CLI: Configuration ready
}
```

**Key insight:** Spans create visual grouping boxes around messages that originate from a specific participant. The correct syntax is `participant.span: Label { ... }`, not `participant."Label": { ... }`.
