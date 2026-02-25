# C4 Diagrams in D2

D2 provides first-class support for C4 model diagrams through its native shapes, containers, and layer system, producing superior layouts with TALA.

## C4 Model Overview

The C4 model provides four levels of abstraction:

1. **System Context** - Shows the system and its relationships with users and external systems
2. **Container** - Shows applications, databases, and services within the system
3. **Component** - Shows internal structure of containers
4. **Code** - Class diagrams (use standard D2 shapes)

## C4 Shape Mapping

| C4 Element | D2 Shape | Styling Convention |
|------------|----------|-------------------|
| Person | `shape: person` + `width: 70` + `height: 100` | Blue fill (#08427B) |
| External Person | `shape: person` + `width: 70` + `height: 100` | Gray fill (#999999) |
| System | Rectangle (default) | Blue fill (#1168BD) |
| External System | Rectangle | Gray fill (#999999) |
| Container | Rectangle | Light blue (#438DD5) |
| Database | `shape: cylinder` | Light blue (#438DD5) |
| Component | Rectangle | Lighter blue (#85BBF0) |
| System Boundary | Container with dashed stroke | Dashed border |

## System Context Diagram

Shows the big picture: your system and its external dependencies.

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

title: |md
  ## Internet Banking System
  System Context Diagram
|

# Actors
customer: Personal Banking Customer {
  shape: person
  width: 70
  height: 100
  style.fill: "#08427B"
  style.font-color: "#ffffff"
}

# Internal system
banking: Internet Banking System {
  style.fill: "#1168BD"
  style.font-color: "#ffffff"
}

# External systems
mainframe: Mainframe Banking System {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}

email: E-mail System {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}

# Relationships
customer -> banking: Views account balances\nand makes payments
banking -> mainframe: Gets account info\nand makes payments
banking -> email: Sends emails using
email -> customer: Sends emails to
```

## Container Diagram

Zooms into the system to show its containers (applications, databases, services).

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

title: |md
  ## Internet Banking System
  Container Diagram
|

# External actors
customer: Personal Banking Customer {
  shape: person
  width: 70
  height: 100
  style.fill: "#08427B"
  style.font-color: "#ffffff"
}

# System boundary
banking: Internet Banking System {
  style.stroke: "#1168BD"
  style.stroke-dash: 3
  style.fill: transparent

  web: Web Application {
    label: |md
      **Web Application**

      Java / Spring MVC

      Delivers static content and
      the single-page application
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  spa: Single-Page Application {
    label: |md
      **Single-Page App**

      JavaScript / React

      Provides banking functionality
      via the browser
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  mobile: Mobile App {
    label: |md
      **Mobile App**

      React Native

      Provides banking functionality
      via mobile devices
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  api: API Application {
    label: |md
      **API Application**

      Java / Spring Boot

      Provides banking functionality
      via JSON/HTTPS API
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  db: Database {
    label: |md
      **Database**

      PostgreSQL

      Stores user credentials,
      access logs, etc.
    |
    shape: cylinder
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }
}

# External systems
mainframe: Mainframe Banking System {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}

email: E-mail System {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}

# Relationships
customer -> banking.web: Visits\n[HTTPS]
customer -> banking.spa: Uses\n[HTTPS]
customer -> banking.mobile: Uses

banking.web -> banking.spa: Delivers
banking.spa -> banking.api: Makes API calls\n[JSON/HTTPS]
banking.mobile -> banking.api: Makes API calls\n[JSON/HTTPS]
banking.api -> banking.db: Reads/writes\n[SQL/TCP]
banking.api -> mainframe: Gets account info\n[XML/HTTPS]
banking.api -> email: Sends emails\n[SMTP]
```

## Component Diagram

Zooms into a container to show its internal components.

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

title: |md
  ## API Application
  Component Diagram
|

# External elements
spa: Single-Page Application {
  style.fill: "#438DD5"
  style.font-color: "#ffffff"
}

mobile: Mobile App {
  style.fill: "#438DD5"
  style.font-color: "#ffffff"
}

db: Database {
  shape: cylinder
  style.fill: "#438DD5"
  style.font-color: "#ffffff"
}

mainframe: Mainframe Banking System {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}

email: E-mail System {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}

# Container boundary
api: API Application {
  style.stroke: "#438DD5"
  style.stroke-dash: 3
  style.fill: transparent

  signin: Sign In Controller {
    label: |md
      **Sign In Controller**

      Spring MVC Controller

      Handles user authentication
    |
    style.fill: "#85BBF0"
  }

  accounts: Accounts Controller {
    label: |md
      **Accounts Controller**

      Spring MVC Controller

      Provides account information
    |
    style.fill: "#85BBF0"
  }

  reset: Reset Password Controller {
    label: |md
      **Reset Password Controller**

      Spring MVC Controller

      Handles password reset flow
    |
    style.fill: "#85BBF0"
  }

  security: Security Component {
    label: |md
      **Security Component**

      Spring Security

      Authentication and authorization
    |
    style.fill: "#85BBF0"
  }

  facade: Mainframe Facade {
    label: |md
      **Mainframe Facade**

      Java Component

      Facade to mainframe system
    |
    style.fill: "#85BBF0"
  }

  emailer: Email Component {
    label: |md
      **Email Component**

      Java Component

      Sends emails to users
    |
    style.fill: "#85BBF0"
  }
}

# Relationships
spa -> api.signin: Makes API calls\n[JSON/HTTPS]
spa -> api.accounts: Makes API calls\n[JSON/HTTPS]
spa -> api.reset: Makes API calls\n[JSON/HTTPS]
mobile -> api.signin: Makes API calls\n[JSON/HTTPS]
mobile -> api.accounts: Makes API calls\n[JSON/HTTPS]

api.signin -> api.security: Uses
api.accounts -> api.facade: Uses
api.reset -> api.security: Uses
api.reset -> api.emailer: Uses

api.security -> db: Reads/writes\n[SQL/TCP]
api.facade -> mainframe: Uses\n[XML/HTTPS]
api.emailer -> email: Sends using\n[SMTP]
```

## Multi-Level C4 with Layers

D2's layer system allows you to create all C4 levels in a single file, switching between views.

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

# Base elements (shared across layers)
customer: Customer {
  shape: person
  width: 70
  height: 100
  style.fill: "#08427B"
  style.font-color: "#ffffff"
}

# Layer 1: System Context
layers: {
  context: {
    # Inherit customer from parent

    system: Banking System {
      style.fill: "#1168BD"
      style.font-color: "#ffffff"
    }

    external: External Systems {
      style.fill: "#999999"
      style.font-color: "#ffffff"
    }

    customer -> system: Uses
    system -> external: Integrates with
  }

  # Layer 2: Container
  container: {
    system: Banking System {
      style.stroke: "#1168BD"
      style.stroke-dash: 3
      style.fill: transparent

      web: Web App { style.fill: "#438DD5" }
      api: API { style.fill: "#438DD5" }
      db: Database { shape: cylinder; style.fill: "#438DD5" }

      web -> api
      api -> db
    }

    customer -> system.web
  }
}
```

**Render specific layer:**
```bash
d2 --layout=tala --tala-seeds=100 --target context banking.d2 context.svg
d2 --layout=tala --tala-seeds=100 --target container banking.d2 container.svg
```

## The Suspend Pattern

For complex multi-view diagrams, use the suspend pattern to share a common model:

```d2
# shared-model.d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

# Define all elements (suspended - not rendered)
_.customer: Customer {
  shape: person
  width: 70
  height: 100
  style.fill: "#08427B"
  style.font-color: "#ffffff"
}

_.web: Web Application {
  style.fill: "#438DD5"
  style.font-color: "#ffffff"
}

_.api: API Application {
  style.fill: "#438DD5"
  style.font-color: "#ffffff"
}

_.db: Database {
  shape: cylinder
  style.fill: "#438DD5"
  style.font-color: "#ffffff"
}

_.mainframe: Mainframe {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}
```

```d2
# context-view.d2
...@shared-model.d2

# Unsuspend only context-level elements
customer: ${_.customer}
system: Banking System {
  style.fill: "#1168BD"
}
mainframe: ${_.mainframe}

customer -> system
system -> mainframe
```

```d2
# container-view.d2
...@shared-model.d2

# Unsuspend container-level elements
customer: ${_.customer}
web: ${_.web}
api: ${_.api}
db: ${_.db}

customer -> web -> api -> db
```

## E-Commerce Platform Example

Complete C4 diagrams for an e-commerce system:

### System Context

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

title: |md
  ## E-Commerce Platform
  System Context
|

# Actors
customer: Online Customer {
  shape: person
  width: 70
  height: 100
  style.fill: "#08427B"
  style.font-color: "#ffffff"
}

admin: Administrator {
  shape: person
  width: 70
  height: 100
  style.fill: "#08427B"
  style.font-color: "#ffffff"
}

warehouse: Warehouse Staff {
  shape: person
  width: 70
  height: 100
  style.fill: "#08427B"
  style.font-color: "#ffffff"
}

# Main system
ecommerce: E-Commerce Platform {
  style.fill: "#1168BD"
  style.font-color: "#ffffff"
}

# External systems
payment: Payment Gateway {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}

shipping: Shipping Provider {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}

email: Email Service {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}

analytics: Analytics Platform {
  style.fill: "#999999"
  style.font-color: "#ffffff"
}

# Relationships
customer -> ecommerce: Browses and purchases
admin -> ecommerce: Manages catalog and orders
warehouse -> ecommerce: Fulfills orders

ecommerce -> payment: Processes payments
ecommerce -> shipping: Ships orders
ecommerce -> email: Sends notifications
ecommerce -> analytics: Tracks events
```

### Container Diagram

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

title: |md
  ## E-Commerce Platform
  Container Diagram
|

# Actors
customer: Customer {
  shape: person
  width: 70
  height: 100
  style.fill: "#08427B"
  style.font-color: "#ffffff"
}

# External systems
payment: Payment Gateway {
  style.fill: "#999999"
}

email: Email Service {
  style.fill: "#999999"
}

# System boundary
platform: E-Commerce Platform {
  style.stroke: "#1168BD"
  style.stroke-dash: 3
  style.fill: transparent

  # Frontend
  web: Web App {
    label: |md
      **Web Application**
      Next.js / React
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  mobile: Mobile App {
    label: |md
      **Mobile App**
      React Native
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  # Backend
  gateway: API Gateway {
    label: |md
      **API Gateway**
      Kong
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  catalog: Catalog Service {
    label: |md
      **Catalog Service**
      Go
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  cart: Cart Service {
    label: |md
      **Cart Service**
      Node.js
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  order: Order Service {
    label: |md
      **Order Service**
      Java / Spring
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  notification: Notification Service {
    label: |md
      **Notification Service**
      Node.js
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  # Data
  catalogdb: Catalog DB {
    shape: cylinder
    label: |md
      **Catalog DB**
      MongoDB
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  orderdb: Order DB {
    shape: cylinder
    label: |md
      **Order DB**
      PostgreSQL
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  cache: Cache {
    shape: cylinder
    label: |md
      **Cache**
      Redis
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }

  queue: Event Bus {
    shape: queue
    label: |md
      **Event Bus**
      Kafka
    |
    style.fill: "#438DD5"
    style.font-color: "#ffffff"
  }
}

# Customer relationships
customer -> platform.web: HTTPS
customer -> platform.mobile: HTTPS

# Internal relationships
platform.web -> platform.gateway: REST
platform.mobile -> platform.gateway: REST

platform.gateway -> platform.catalog: gRPC
platform.gateway -> platform.cart: gRPC
platform.gateway -> platform.order: gRPC

platform.catalog -> platform.catalogdb: MongoDB
platform.cart -> platform.cache: Redis
platform.order -> platform.orderdb: SQL

platform.order -> platform.queue: Publishes events
platform.notification -> platform.queue: Consumes events

# External relationships
platform.order -> payment: HTTPS
platform.notification -> email: SMTP
```

## Best Practices

1. **Use consistent colors** - Establish a color palette matching C4 conventions
2. **Add descriptions** - Use Markdown labels for rich element descriptions
3. **Show technology** - Include tech stack in container/component labels
4. **Indicate protocols** - Label relationships with protocols (HTTPS, gRPC, SQL)
5. **Use boundaries** - Wrap related elements in dashed containers
6. **Layer for complexity** - Use D2 layers for multi-level views
7. **Keep focused** - One perspective per diagram
8. **Document relationships** - Label every connection with purpose
9. **Constrain person shapes** - Always set `width: 70` and `height: 100` to prevent TALA from stretching the figure based on label width
