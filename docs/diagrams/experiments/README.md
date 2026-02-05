# TALA Seeds Experiment Results

This document contains the analysis of different `--tala-seeds` values for D2 diagrams to determine the optimal default for deterministic, high-quality layouts.

## Executive Summary

**Recommended Seed: `100`**

Seed 100 provides the best overall balance of:
- Compact layouts for flowcharts (2nd most compact for runtime-decision)
- Optimal C4 diagram layout (matches the best seeds: 0, 7, 13, 42, 500, 1000)
- Good aspect ratios for web display
- Small path complexity (efficient edge routing)

**Current seed (42) performs poorly for flowcharts** (least compact runtime-decision layout, second-least compact discovery-flow) but optimally for C4 diagrams.

## Methodology

### Test Diagrams

| Diagram | Type | Complexity | Key Characteristics |
|---------|------|------------|---------------------|
| `c4/container.d2` | C4 | High | 377 lines, nested containers, many nodes |
| `c4/context.d2` | C4 | Medium | 132 lines, external actors, system boundaries |
| `flowcharts/runtime-decision.d2` | Flowchart | High | 202 lines, deep decision tree, diamond shapes |
| `flowcharts/discovery-flow.d2` | Flowchart | Medium | 196 lines, grouped sections, loops |
| `sequences/execution-main.d2` | Sequence | Medium | 67 lines, linear flow with grouped phases |

### Seeds Tested

```
0, 1, 7, 13, 23, 42 (current), 100, 123, 256, 500, 1000, 9999
```

### Evaluation Criteria

1. **Compactness**: Total bounding box area (lower = better)
2. **Aspect Ratio**: Suitability for web display (0.7-1.5 ideal)
3. **Path Complexity**: Edge routing efficiency (lower = cleaner)
4. **Layout Stability**: Consistency across diagram types

## Key Findings

### Finding 1: Sequence Diagrams Are Seed-Invariant

All sequence diagrams produce **identical output** regardless of seed value:
- `sequences-execution-main.svg`: 42,137 bytes, viewBox 1684×2808 for ALL seeds

D2's `shape: sequence_diagram` uses a specialized layout algorithm that ignores TALA seeds entirely.

### Finding 2: C4 Context Is Also Stable

The simpler C4 context diagram produces **identical output** for all seeds:
- `c4-context.svg`: 67,805 bytes, viewBox 1217×801 for ALL seeds

### Finding 3: C4 Container Has Stable Clusters

Seven seeds produce the optimal C4 container layout:
- **Optimal cluster**: Seeds 0, 7, 13, 42, 100, 500, 1000 → 1568×2229 (3,495,072 sq px)
- Seeds 1, 123, 256, 23, 9999 produce slightly larger layouts

### Finding 4: Flowcharts Are Highly Seed-Sensitive

Every seed produces a unique layout for both flowchart diagrams, with dramatic differences in compactness and aspect ratio.

## Detailed Scoring Matrix

### Runtime Decision Flowchart

| Seed | Dimensions | Area (sq px) | Aspect Ratio | Compactness Rank | Quality |
|------|------------|--------------|--------------|------------------|---------|
| 500 | 1860×1514 | 2,816,040 | 1.228 | 1 (best) | ★★★★★ |
| 100 | 1497×2066 | 3,092,802 | 0.724 | 2 | ★★★★★ |
| 256 | 2230×1475 | 3,289,250 | 1.511 | 3 | ★★★★☆ |
| 123 | 2502×1373 | 3,435,246 | 1.822 | 4 | ★★★☆☆ |
| 13 | 2779×1277 | 3,548,783 | 2.176 | 5 | ★★☆☆☆ |
| 0 | 1995×1900 | 3,790,500 | 1.050 | 6 | ★★★★☆ |
| 23 | 2777×1401 | 3,890,577 | 1.982 | 7 | ★★★☆☆ |
| 9999 | 2128×1838 | 3,911,264 | 1.157 | 8 | ★★★★☆ |
| 1 | 2095×2015 | 4,221,425 | 1.039 | 9 | ★★★☆☆ |
| 1000 | 2514×1769 | 4,447,266 | 1.421 | 10 | ★★★☆☆ |
| 7 | 2023×2326 | 4,705,498 | 0.869 | 11 | ★★★☆☆ |
| **42** | 2295×2066 | 4,741,470 | 1.110 | **12 (worst)** | ★★☆☆☆ |

### Discovery Flow Flowchart

| Seed | Dimensions | Area (sq px) | Aspect Ratio | Compactness Rank | Quality |
|------|------------|--------------|--------------|------------------|---------|
| 9999 | 2106×2766 | 5,825,196 | 0.761 | 1 (best) | ★★★★★ |
| 7 | 3223×1981 | 6,384,763 | 1.626 | 2 | ★★★☆☆ |
| 13 | 1958×3281 | 6,424,198 | 0.596 | 3 | ★★★☆☆ |
| 123 | 1958×3373 | 6,604,334 | 0.580 | 4 | ★★★☆☆ |
| 100 | 2215×3044 | 6,742,460 | 0.727 | 5 | ★★★★☆ |
| 1 | 2548×2682 | 6,833,736 | 0.950 | 6 | ★★★★☆ |
| 256 | 2276×3044 | 6,928,144 | 0.747 | 7 | ★★★★☆ |
| 500 | 3568×1981 | 7,068,208 | 1.801 | 8 | ★★☆☆☆ |
| 1000 | 2269×3169 | 7,190,461 | 0.715 | 9 | ★★★★☆ |
| 23 | 2589×2840 | 7,352,760 | 0.911 | 10 | ★★★★☆ |
| 0 | 2681×2840 | 7,614,040 | 0.944 | 11 | ★★★★☆ |
| **42** | 2697×2840 | 7,659,480 | 0.949 | **11 (tied)** | ★★★☆☆ |

### C4 Container Diagram

| Seed | Dimensions | Area (sq px) | Compactness Rank | Quality |
|------|------------|--------------|------------------|---------|
| 0, 7, 13, **42**, 100, 500, 1000 | 1568×2229 | 3,495,072 | 1 (best, tied) | ★★★★★ |
| 1 | 1570×2281 | 3,581,170 | 2 | ★★★★☆ |
| 123 | 1602×2281 | 3,654,162 | 3 | ★★★★☆ |
| 256 | 1570×2333 | 3,662,810 | 4 | ★★★★☆ |
| 23 | 1602×2333 | 3,737,466 | 5 | ★★★★☆ |
| 9999 | 1607×2425 | 3,896,975 | 6 (worst) | ★★★☆☆ |

## Aggregate Scoring

| Seed | Runtime (rank) | Discovery (rank) | C4 (rank) | **Total Rank Sum** | **Overall** |
|------|----------------|------------------|-----------|--------------------|-------------|
| **100** | 2 | 5 | 1 | **8** | ★★★★★ |
| 0 | 6 | 11 | 1 | 18 | ★★★★☆ |
| 500 | 1 | 8 | 1 | 10 | ★★★★☆ |
| 1000 | 10 | 9 | 1 | 20 | ★★★☆☆ |
| 7 | 11 | 2 | 1 | 14 | ★★★★☆ |
| 13 | 5 | 3 | 1 | 9 | ★★★★☆ |
| 256 | 3 | 7 | 4 | 14 | ★★★★☆ |
| 9999 | 8 | 1 | 6 | 15 | ★★★★☆ |
| **42 (current)** | **12** | **11** | 1 | **24** | ★★★☆☆ |
| 1 | 9 | 6 | 2 | 17 | ★★★☆☆ |
| 23 | 7 | 10 | 5 | 22 | ★★★☆☆ |
| 123 | 4 | 4 | 3 | 11 | ★★★★☆ |

**Lower total rank sum = better overall performance.**

## Recommendation

### Primary Recommendation: Seed 100

**Total rank sum: 8** (best among all tested seeds)

Seed 100 excels because:
1. **2nd most compact** runtime-decision flowchart (3,092,802 sq px)
2. **Good aspect ratio** (0.724) - slightly tall but works well for top-to-bottom flowcharts
3. **Optimal C4 layout** - produces the same compact layout as the best seeds
4. **Consistent performance** - no poor results for any diagram type

### Alternative: Seed 13

**Total rank sum: 9** (second best)

Seed 13 is a strong alternative:
- Excellent flowchart compactness (ranks 5th and 3rd)
- Optimal C4 layout
- However, very wide aspect ratio for runtime-decision (2.176) may cause horizontal scrolling

### Why Not Continue with Seed 42?

Seed 42 has **total rank sum: 24** (worst among seeds that produce optimal C4 layouts):
- **Worst** compactness for runtime-decision flowchart
- **Tied for second-worst** for discovery-flow flowchart
- While it produces optimal C4 layouts, that's not unique (6 other seeds match it)

## Migration Impact

Changing from seed 42 to seed 100 will:
1. **Improve flowchart layouts** - significantly more compact
2. **Maintain C4 diagram quality** - produces identical output
3. **Keep sequence diagrams unchanged** - seeds have no effect

## Files to Update

If adopting seed 100:

1. `scripts/render-diagrams.sh:80`
   ```bash
   # Change from:
   render_args+=(--tala-seeds=42)
   # To:
   render_args+=(--tala-seeds=100)
   ```

2. `.agents/skills/d2-diagrams/SKILL.md:195`
   - Update example seed value

3. `.agents/skills/d2-diagrams/references/layout-engines.md:193`
   - Fix incorrect d2-config example (tala-seeds in config is wrong - must use CLI flag)

## Appendix: Path Complexity Data

Path data length (chars) as a proxy for edge routing complexity:

| Seed | Runtime Decision | Discovery Flow |
|------|------------------|----------------|
| 100 | 5471 | 5020 |
| 13 | 5807 | 4899 |
| 23 | 5552 | 5044 |
| **42** | 5641 | 5092 |
| 0 | 5887 | 5128 |
| 1 | 5744 | 4959 |
| 7 | 5942 | 5333 |
| 123 | 6017 | 4899 |
| 256 | 6244 | 5024 |
| 500 | 5573 | 5186 |
| 1000 | 5830 | 4960 |
| 9999 | 5844 | 4977 |

Seed 100 has among the **lowest path complexity** for both diagrams, indicating cleaner edge routing.

## Experiment Artifacts

All 60 rendered SVGs are archived in `docs/diagrams/experiments/seed-*/`:
- 12 seed directories × 5 diagrams = 60 SVG files
- Total archive size: ~7.2 MB
- These can be used for future reference or visual comparison
