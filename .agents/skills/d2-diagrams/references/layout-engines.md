# Layout Engines

D2 supports multiple layout engines, each with different strengths. Choosing the right engine significantly impacts diagram quality and rendering performance.

## Engine Comparison

| Engine | Cost | Quality | Speed | Best For |
|--------|------|---------|-------|----------|
| **dagre** | Free | Good | Fast | Simple diagrams, quick drafts |
| **ELK** | Free | Better | Medium | Complex graphs, orthogonal routing |
| **TALA** | $5-20/mo | Best | Medium | Production, precise layouts |

## Configuration

Set the layout engine in your D2 file:

```d2
vars: {
  d2-config: {
    layout-engine: tala  # or: dagre, elk
  }
}
```

Or via command line:

```bash
d2 --layout=tala diagram.d2 output.svg
d2 --layout=elk diagram.d2 output.svg
d2 --layout=dagre diagram.d2 output.svg
```

## Dagre (Default)

**Dagre** is D2's default layout engine. It's free, fast, and produces good results for most diagrams.

### Strengths

- Fast rendering
- Good for hierarchical layouts
- No cost, bundled with D2
- Predictable output

### Weaknesses

- Limited control over positioning
- May produce suboptimal layouts for complex graphs
- No grid support
- Less sophisticated edge routing

### When to Use

- Quick drafts and prototypes
- Simple flowcharts
- When render speed matters
- When TALA license isn't available

### Example

```d2
vars: {
  d2-config: {
    layout-engine: dagre
  }
}

a -> b -> c
a -> d -> c
```

## ELK (Eclipse Layout Kernel)

**ELK** provides more sophisticated algorithms than dagre, with better edge routing and crossing minimization.

### Strengths

- Orthogonal edge routing
- Better crossing minimization
- Good for complex graphs
- Free and open source

### Weaknesses

- Slower than dagre
- No grid support
- Less intuitive manual positioning
- May struggle with very large diagrams

### When to Use

- Complex dependency graphs
- When edge crossings are a problem
- Architectural diagrams with many connections
- When TALA isn't available but dagre isn't sufficient

### Configuration Options

```d2
vars: {
  d2-config: {
    layout-engine: elk
    elk: {
      # ELK-specific options
      nodeSpacingFactor: 2.0
      edgeSpacingFactor: 1.5
    }
  }
}
```

## TALA (Terrastruct Auto Layout Algorithm)

**TALA** is Terrastruct's proprietary layout engine, offering the highest quality layouts with unique features.

### Strengths

- Superior auto-layout quality
- Grid layouts
- `near` keyword for relative positioning
- Container-aware algorithms
- Deterministic with seeds
- Best for production diagrams

### Weaknesses

- Requires paid license ($5-20/month)
- Slightly slower than dagre
- Requires additional installation

### Unique Features

#### Grid Layouts

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

services: {
  grid-columns: 3

  auth: Auth
  users: Users
  orders: Orders
  products: Products
  payments: Payments
  notifications: Notifications
}
```

#### `near` Positioning

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

server: Web Server

# Position database near the server
db: Database {
  near: server
}

# Position at specific locations
title: System Architecture {
  near: top-center
}

legend: Legend {
  near: bottom-right
}
```

**Valid `near` values:**

- Relative to element: `near: element-name`
- Absolute positions:
  - `top-left`, `top-center`, `top-right`
  - `center-left`, `center`, `center-right`
  - `bottom-left`, `bottom-center`, `bottom-right`

#### Deterministic Seeds

**Use `--tala-seeds` CLI flag for reproducible layouts:**

```bash
# TALA seeds is a CLI flag, NOT a d2-config option
# Seed 100 provides optimal compactness for flowcharts while maintaining C4 quality
d2 --layout=tala --tala-seeds=100 diagram.d2 diagram.svg
```

> **⚠️ Common Mistake:** Putting `tala-seeds` inside `d2-config` causes `"tala-seeds" is not a valid config` errors. The `--tala-seeds` flag is TALA-specific and must be passed via CLI, not in the D2 file.

### When to Use

- Production documentation
- When layout quality matters
- Grid-based architectures
- When you need `near` positioning
- CI/CD pipelines (with deterministic seeds)

### Installation

```bash
# Check if TALA is installed
d2 layout

# Install TALA (requires license)
# Follow instructions at https://terrastruct.com/tala
```

### License Types

| Plan | Price | Features |
|------|-------|----------|
| Individual | $5/mo | Personal use |
| Team | $10/mo/user | Team collaboration |
| Enterprise | Custom | Volume licensing, support |

For CI/CD, set the license via environment variable:

```bash
export TALA_LICENSE="your-license-key"
```

## Engine Selection Guide

### Decision Tree

```
Is this a quick draft?
├─ Yes → Use dagre
└─ No → Is TALA available?
         ├─ Yes → Use TALA
         └─ No → Is the graph complex?
                  ├─ Yes → Use ELK
                  └─ No → Use dagre
```

### By Diagram Type

| Diagram Type | Recommended Engine |
|--------------|-------------------|
| C4 Architecture | TALA (or ELK) |
| Sequence Diagram | dagre |
| Simple Flowchart | dagre |
| Complex Flowchart | TALA |
| Grid Layout | TALA (required) |
| Dependency Graph | ELK or TALA |
| State Machine | dagre or ELK |
| Network Topology | TALA |

## Performance Considerations

### Rendering Speed

```
dagre < ELK < TALA
(fastest)    (slowest)
```

### Diagram Size Limits

| Engine | Recommended Max Nodes |
|--------|----------------------|
| dagre | 200 |
| ELK | 150 |
| TALA | 100 |

For larger diagrams, consider splitting into multiple files.

## Comparison Examples

### Simple Graph

All engines produce similar results:

```d2
a -> b -> c -> d
```

### Complex Graph

Differences become apparent:

```d2
vars: {
  d2-config: {
    layout-engine: tala  # Try switching to dagre or elk
  }
}

# For deterministic output, use CLI flag: d2 --layout=tala --tala-seeds=100

# Complex interconnections
a -> b
a -> c
a -> d
b -> e
b -> f
c -> e
c -> g
d -> f
d -> g
e -> h
f -> h
g -> h
```

### Grid Layout (TALA Only)

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

dashboard: Dashboard Components {
  grid-columns: 3
  grid-gap: 16

  header: Header
  sidebar: Sidebar
  main: Main Content
  footer: Footer
  nav: Navigation
  widgets: Widgets
}
```

## Fallback Strategy

For projects that may not have TALA:

```bash
#!/bin/bash
# render.sh - Render with fallback

D2_FILE="$1"
OUTPUT="${D2_FILE%.d2}.svg"

# Try TALA first
if d2 layout | grep -q tala; then
    d2 --layout=tala "$D2_FILE" "$OUTPUT"
else
    echo "TALA not available, using ELK"
    d2 --layout=elk "$D2_FILE" "$OUTPUT"
fi
```

Or in the D2 file, provide both versions:

```d2
# diagram.d2 - Works with any engine
vars: {
  d2-config: {
    # TALA preferred, but works with dagre/elk
    layout-engine: tala
  }
}

# Avoid TALA-only features (grid, near) for portability
a -> b -> c
```

## CI/CD Considerations

### With TALA License

```yaml
env:
  TALA_LICENSE: ${{ secrets.TALA_LICENSE }}

steps:
  - run: d2 --layout=tala diagram.d2 output.svg
```

### Without TALA License

```yaml
steps:
  - run: d2 --layout=elk diagram.d2 output.svg
```

### Portable Approach

```yaml
steps:
  - name: Render diagrams
    run: |
      if [ -n "$TALA_LICENSE" ]; then
        LAYOUT="tala"
      else
        LAYOUT="elk"
      fi
      d2 --layout=$LAYOUT diagram.d2 output.svg
```

## Best Practices

1. **Default to TALA for production** - Best quality
2. **Use dagre for quick iterations** - Fastest feedback
3. **Always set seeds with TALA** - Reproducible builds
4. **Avoid TALA-only features for portability** - If sharing files
5. **Test with target engine** - Layouts differ between engines
6. **Document engine requirements** - In project README
7. **Use CI fallback** - For open source projects
