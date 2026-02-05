# Docusaurus and GitHub Integration

This reference covers integrating D2 diagrams with Docusaurus documentation sites and GitHub workflows.

## Docusaurus Integration

### Option 1: remark-d2 Plugin

The `remark-d2` plugin renders D2 code blocks at build time.

#### Installation

```bash
npm install remark-d2
```

#### Configuration (docusaurus.config.js)

```javascript
// ES Module import (Docusaurus 3.x)
import remarkD2 from 'remark-d2';

export default {
  presets: [
    [
      '@docusaurus/preset-classic',
      {
        docs: {
          remarkPlugins: [
            [remarkD2, {
              // Plugin options
              compilePath: 'd2',  // Path to d2 binary
              layout: 'tala',     // Layout engine
              theme: 0,           // Theme ID (0 = default)
              darkTheme: 200,     // Dark mode theme
              pad: 10,            // Padding around diagram
            }],
          ],
        },
      },
    ],
  ],
};
```

#### Usage in MDX

````mdx
# Architecture Overview

Here's our system architecture:

```d2
vars: {
  d2-config: {
    layout-engine: tala
    tala-seeds: 42
  }
}

user: User {
  shape: person
}

api: API Server
db: Database {
  shape: cylinder
}

user -> api -> db
```
````

### Option 2: Pre-rendered SVGs

For more control, pre-render D2 to SVG and import:

#### Directory Structure

```
docs/
├── architecture/
│   ├── overview.mdx
│   └── diagrams/
│       ├── system-context.d2
│       └── system-context.svg
```

#### Render Script

```bash
#!/bin/bash
# scripts/render-diagrams.sh

for d2file in docs/**/diagrams/*.d2; do
    svgfile="${d2file%.d2}.svg"
    echo "Rendering: $d2file -> $svgfile"
    d2 fmt "$d2file"
    d2 --layout=tala "$d2file" "$svgfile"
done
```

#### Usage in MDX

```mdx
import SystemContext from './diagrams/system-context.svg';

# Architecture Overview

<SystemContext />

Or as an image:

![System Context](./diagrams/system-context.svg)
```

### Dark Mode Support

D2 supports theme variants for light/dark modes:

```javascript
// docusaurus.config.js
import remarkD2 from 'remark-d2';

export default {
  presets: [
    [
      '@docusaurus/preset-classic',
      {
        docs: {
          remarkPlugins: [
            [remarkD2, {
              theme: 0,       // Light theme
              darkTheme: 200, // Dark theme (Terrastruct Dark)
            }],
          ],
        },
      },
    ],
  ],
};
```

**Available themes:**
- `0` - Default (light)
- `1` - Neutral Grey
- `3` - Flagship Terrastruct
- `4` - Cool Classics
- `5` - Mixed Berry Blue
- `6` - Grape Soda
- `7` - Aubergine
- `8` - Colorblind Clear
- `100-105` - Dark variants
- `200` - Terrastruct Dark

### Handling ESM Import Issues

If you encounter ESM/CommonJS issues:

```javascript
// docusaurus.config.js

// Dynamic import workaround
const getConfig = async () => {
  const remarkD2 = (await import('remark-d2')).default;

  return {
    presets: [
      [
        '@docusaurus/preset-classic',
        {
          docs: {
            remarkPlugins: [[remarkD2, { layout: 'tala' }]],
          },
        },
      ],
    ],
  };
};

export default getConfig();
```

## GitHub Integration

### GitHub Actions: Render on Push

```yaml
# .github/workflows/render-diagrams.yml
name: Render D2 Diagrams

on:
  push:
    paths:
      - 'docs/**/*.d2'
      - 'website/docs/**/*.d2'
  pull_request:
    paths:
      - 'docs/**/*.d2'
      - 'website/docs/**/*.d2'

jobs:
  render:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install D2
        run: curl -fsSL https://d2lang.com/install.sh | sh -s --

      - name: Install TALA
        if: ${{ env.TALA_LICENSE }}
        env:
          TALA_LICENSE: ${{ secrets.TALA_LICENSE }}
        run: |
          echo "Installing TALA..."
          # TALA installation commands

      - name: Render diagrams
        run: |
          find . -name '*.d2' -type f | while read -r d2file; do
            svgfile="${d2file%.d2}.svg"
            echo "Rendering: $d2file"
            d2 fmt "$d2file"
            d2 validate "$d2file"
            d2 --layout=${TALA_LICENSE:+tala}${TALA_LICENSE:-elk} \
               "$d2file" "$svgfile"
          done

      - name: Commit changes
        if: github.event_name == 'push' && github.ref == 'refs/heads/main'
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add -A '*.svg'
          if ! git diff --staged --quiet; then
            git commit -m "chore: render D2 diagrams"
            git push
          fi
```

### GitHub Actions: Validate on PR

```yaml
# .github/workflows/validate-diagrams.yml
name: Validate D2 Diagrams

on:
  pull_request:
    paths:
      - '**/*.d2'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install D2
        run: curl -fsSL https://d2lang.com/install.sh | sh -s --

      - name: Validate diagrams
        run: |
          errors=0
          find . -name '*.d2' -type f | while read -r d2file; do
            echo "Validating: $d2file"
            if ! d2 validate "$d2file"; then
              errors=$((errors + 1))
            fi
          done
          exit $errors
```

### Docusaurus Build with D2

```yaml
# .github/workflows/deploy-docs.yml
name: Deploy Documentation

on:
  push:
    branches: [main]
    paths:
      - 'website/**'
      - 'docs/**'

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install D2
        run: curl -fsSL https://d2lang.com/install.sh | sh -s --

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: website/package-lock.json

      - name: Install dependencies
        run: npm ci
        working-directory: website

      - name: Build website
        run: npm run build
        working-directory: website

      - name: Deploy to GitHub Pages
        uses: peaceiris/actions-gh-pages@v3
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          publish_dir: website/build
```

## Hybrid Approach (Recommended)

Combine pre-rendered SVGs with remark-d2 for the best of both worlds:

### Strategy

1. **Source files**: Store `.d2` files in version control
2. **CI rendering**: Auto-render on push with GitHub Actions
3. **Documentation**: Reference pre-rendered SVGs in MDX
4. **Development**: Use remark-d2 for live preview

### Implementation

```
project/
├── docs/
│   └── architecture/
│       ├── overview.mdx
│       └── diagrams/
│           ├── context.d2      # Source
│           └── context.svg     # Rendered (gitignored or committed)
├── scripts/
│   └── render-diagrams.sh
├── .github/
│   └── workflows/
│       └── render-diagrams.yml
└── website/
    └── docusaurus.config.js
```

### Development Workflow

```bash
# Watch mode for local development
d2 --watch docs/architecture/diagrams/context.d2 &

# Or use remark-d2 with Docusaurus dev server
cd website && npm start
```

### Production Build

```bash
# Render all diagrams
./scripts/render-diagrams.sh

# Build Docusaurus site
cd website && npm run build
```

## GitHub README Integration

### Option 1: Pre-rendered in CI

Store rendered SVGs and reference in README:

```markdown
# Project Name

## Architecture

![System Context](docs/diagrams/context.svg)
```

### Option 2: GitHub Actions Badge-style

Create a workflow that updates a badge image:

```yaml
# .github/workflows/diagram-badge.yml
- name: Update architecture badge
  run: |
    d2 docs/diagrams/context.d2 .github/assets/architecture.svg
    git add .github/assets/architecture.svg
    git commit -m "Update architecture diagram" || true
    git push
```

### Option 3: External Rendering Service

Use a service like Kroki:

```markdown
![Diagram](https://kroki.io/d2/svg/eNpLyUwpSizIUHBJTSxR8MnPS8ksUgQA)
```

**Note:** Kroki URL encodes the D2 source. This is useful for simple diagrams but limited for complex ones.

## Caching Strategies

### Cache D2 Binary

```yaml
- uses: actions/cache@v4
  with:
    path: ~/.local/bin/d2
    key: d2-${{ runner.os }}-${{ hashFiles('.d2-version') }}

- name: Install D2
  if: steps.cache.outputs.cache-hit != 'true'
  run: curl -fsSL https://d2lang.com/install.sh | sh -s --
```

### Cache Rendered Diagrams

```yaml
- uses: actions/cache@v4
  with:
    path: docs/**/diagrams/*.svg
    key: diagrams-${{ hashFiles('docs/**/*.d2') }}
```

## Troubleshooting

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| "d2: command not found" | D2 not in PATH | Add `~/.local/bin` to PATH |
| ESM import error | Docusaurus/Node version | Use dynamic import |
| Blank diagrams | Missing D2 in CI | Install D2 in workflow |
| Different renders | No seed set | Add `tala-seeds` config |
| TALA not available | No license | Use ELK fallback |

### Debug Mode

```yaml
- name: Debug D2
  run: |
    d2 --version
    d2 layout  # List available engines
    d2 --debug diagram.d2 output.svg 2>&1 | head -100
```

## Best Practices

1. **Store source `.d2` files** - Version control the source
2. **Render in CI** - Consistent output across environments
3. **Use deterministic seeds** - Prevent unnecessary diffs
4. **Cache aggressively** - Speed up builds
5. **Validate in PRs** - Catch errors before merge
6. **Use TALA when available** - Best layout quality
7. **Provide ELK fallback** - For contributors without TALA
8. **Document diagram conventions** - In CONTRIBUTING.md
