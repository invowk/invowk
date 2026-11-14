# Layout Engines

D2 supports multiple layout engines, each with different strengths. Choosing the right engine significantly impacts diagram quality and rendering performance.

## Engine Comparison

| Engine | Availability | Routing | Speed | Best For |
|--------|--------------|---------|-------|----------|
| **dagre** | Bundled | Basic | Fast | Simple diagrams, quick drafts |
| **ELK** | Bundled | Orthogonal | Medium | Complex graphs and dense relationships |
| **TALA** | License-dependent | Constraint-aware | Medium | Layouts needing relative positioning (`near` to other objects) |

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

An explicit `--layout` flag overrides the file's `d2-config` engine (`d2 --debug` prints which
layout plugin was used).

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
- Less intuitive manual positioning
- May struggle with very large diagrams

### When to Use

- Complex dependency graphs
- When edge crossings are a problem
- Architectural diagrams with many connections
- When TALA isn't available but dagre isn't sufficient

### Configuration Options

ELK options are CLI flags, not `d2-config` keys (an `elk: {...}` block inside `d2-config` fails
to compile with `"elk" needs a value`). `d2 layout elk` lists them; in v0.7.1:

```bash
d2 --layout=elk \
  --elk-algorithm layered \
  --elk-nodeNodeBetweenLayers 70 \
  --elk-edgeNodeBetweenLayers 40 \
  --elk-nodeSelfLoop 50 \
  --elk-padding "[top=50,left=50,bottom=50,right=50]" \
  diagram.d2 output.svg
```

The values shown are the defaults. ELK is a local preview engine in this repository, so these
flags never belong in committed render commands.

## TALA (Terrastruct Auto Layout Algorithm)

**TALA** is a proprietary layout engine with constraint-oriented features that
are not available in every installation.

### Strengths

- `near` keyword for relative positioning
- Container-aware algorithms
- Deterministic with seeds
- Useful when the diagram needs explicit relative-layout constraints

### Weaknesses

- Requires separate installation and may require a commercial license
- Slightly slower than dagre
- Requires additional installation

### Unique Features

#### Grid Layouts

Grids are not TALA-only: D2 lays out `grid-*` containers itself, and dagre and ELK render the
same grid (checked with D2 v0.7.1). TALA is still the repository engine for the rest of the diagram.

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
- Committed repository renders (`make render-diagrams` applies the deterministic seed)

### Installation

```bash
# Check if TALA is installed
d2 layout

# Install TALA (requires license)
# Follow instructions at https://terrastruct.com/tala
```

### License and Availability

Do not encode prices or plan names in this skill because they are external and
volatile. Before introducing a new TALA dependency, verify current installation,
license, authentication, and CI terms in the official D2/TALA documentation.

For CI/CD, set the TALA API token via environment variable or a TStruct auth file:

```bash
export TSTRUCT_TOKEN="your-api-token"
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
| Grid Layout | Any engine (grids are laid out by D2 itself) |
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

### Grid Layout (All Engines)

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

## No Fallback for Committed Renders

This repository has no engine fallback. Committed renders require TALA and go through
`make render-diagrams` (`scripts/render-diagrams.sh` probes TALA and exits with an error when it
is missing). `./scripts/render-diagrams.sh --allow-elk` (or `INVOWK_ALLOW_ELK_DIAGRAMS=1`) renders
with ELK for local preview only; never commit those SVGs. CI does not render at all:
`validate-diagrams.yml` validates syntax, readability and the committed render stamps, so it
needs no TALA token.

If a diagram must also render elsewhere without TALA, avoid TALA-only features (`near`
pointing at another object) so it stays portable:

```d2
# Works with any engine
vars: {
  d2-config: {
    layout-engine: tala
  }
}

a -> b -> c
```

## Best Practices

1. **Default to TALA for production** - Best quality
2. **Use dagre for quick iterations** - Fastest feedback
3. **Always set seeds with TALA** - Reproducible builds
4. **Avoid TALA-only features for portability** - If sharing files
5. **Test with target engine** - Layouts differ between engines
6. **Document engine requirements** - In project README
7. **No ELK fallback for committed renders** - ELK is a local preview (`--allow-elk`) only
