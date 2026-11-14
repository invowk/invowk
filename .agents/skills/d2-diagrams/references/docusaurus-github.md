# Docusaurus and GitHub Integration

How Invowk diagrams reach the website and GitHub. Invowk does not use the `remark-d2` plugin,
Kroki, or any other build-time or external D2 renderer: diagrams are committed, pre-rendered SVGs
served by the website's `Diagram` component. Do not introduce a build-time or CI renderer.

## Invowk GitHub Integration

Invowk uses committed, pre-rendered SVGs for Docusaurus and repository docs.
Render locally with `make render-diagrams`, commit both source `.d2` and stamped
SVG outputs, and let `.github/workflows/validate-diagrams.yml` validate syntax,
readability, renders, and manifests in CI.

Do not add CI render-on-push or auto-commit workflows for this repo. The website
serves committed assets through Docusaurus static directories and the `Diagram`
component, so CI should prove the committed outputs are current rather than
mutating the branch.

- Sources: `docs/diagrams/<dir>/<name>.d2`.
- Renders: `docs/diagrams/rendered/<dir>/<name>.svg`, each stamped with its source path,
  SHA-256, layout and seed (`scripts/render-diagrams.sh`).
- Website: `website/src/components/Diagram/index.tsx` maps diagram ids to
  `/diagrams/rendered/...` SVGs in `svgPaths`. A new website diagram needs an entry there
  (versioning impact: the `docs` skill, "Versioning Pitfalls").
- Architecture prose in `docs/architecture/` references the rendered SVGs directly.

### Development Workflow

```bash
make render-diagrams
make check-diagram-renders
make check-diagram-readability
```

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| "d2: command not found" | D2 not in PATH | Add `~/.local/bin` to PATH |
| Different renders | Hand-rolled render command without the repository's seed | Render with `make render-diagrams`, which applies `--tala-seeds=100` when supported |
| TALA not available | No TALA plugin or license | Install TALA for committed renders; `./scripts/render-diagrams.sh --allow-elk` is a local preview only and its SVGs must not be committed |
| `check-diagram-renders` reports stale or orphaned SVGs | Source changed or was removed without re-rendering | Run `make render-diagrams`, review the diff, commit source and SVG together |

### Debug Mode

```bash
d2 --version
d2 layout  # List available engines
d2 --debug diagram.d2 "$(mktemp -d)/output.svg" 2>&1 | head -100
```

## Best Practices

1. **Store source `.d2` files** - Version control the source
2. **Commit stamped renders made locally** - `make render-diagrams`, never a CI render or auto-commit
3. **Use deterministic seeds** - Let `make render-diagrams` apply the repository seed (100) to prevent unnecessary diffs
4. **Validate in PRs** - `validate-diagrams.yml` checks syntax, readability and render manifests
5. **Use TALA for committed renders** - ELK is a local preview only
6. **Document diagram conventions** - In the `d2-diagrams` skill
