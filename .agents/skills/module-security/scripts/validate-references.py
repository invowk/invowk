#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Validate module-security documentation against the live audit inventory."""

from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
SKILL_DIR = ROOT / ".agents/skills/module-security"
SCANNER = ROOT / "internal/audit/scanner.go"


def default_checkers() -> list[str]:
    source = SCANNER.read_text(encoding="utf-8")
    match = re.search(
        r"func DefaultCheckers\(\) \[\]Checker \{(?P<body>.*?)\n\}",
        source,
        flags=re.DOTALL,
    )
    if not match:
        raise ValueError("cannot locate DefaultCheckers()")
    return re.findall(r"New([A-Za-z0-9]+Checker)\s*\(", match.group("body"))


def main() -> int:
    errors: list[str] = []
    try:
        checkers = default_checkers()
    except (OSError, ValueError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    skill = (SKILL_DIR / "SKILL.md").read_text(encoding="utf-8")
    catalog = (SKILL_DIR / "references/check-catalog.md").read_text(encoding="utf-8")
    implementation = (
        SKILL_DIR / "references/implementation-review.md"
    ).read_text(encoding="utf-8")

    if not checkers:
        errors.append("DefaultCheckers() returned no discoverable constructors")
    checker_sources = sorted((ROOT / "internal/audit").glob("checks_*.go"))
    for checker in checkers:
        constructor = f"func New{checker}"
        sources = [
            path for path in checker_sources if constructor in path.read_text(encoding="utf-8")
        ]
        if len(sources) != 1:
            errors.append(
                f"expected one source defining New{checker}, found {len(sources)}"
            )
        if checker not in skill:
            errors.append(f"SKILL.md does not name default checker {checker}")
        if checker not in catalog:
            errors.append(f"check-catalog.md does not name default checker {checker}")

    markdown = "\n".join(
        path.read_text(encoding="utf-8")
        for path in [SKILL_DIR / "SKILL.md", *sorted((SKILL_DIR / "references").glob("*.md"))]
    )
    for referenced in sorted(
        set(re.findall(r"`((?:internal/audit|cmd/invowk)/[A-Za-z0-9_./-]+\.go)`", markdown))
    ):
        if not (ROOT / referenced).is_file():
            errors.append(f"documented source file does not exist: {referenced}")

    volatile_patterns = {
        "source line count": r"`[^`]+\.go`,?\s+\d+\s+lines?",
        "test inventory count": r"(?:Unit|CLI) Tests? \(\d+ (?:files?|txtar files?)",
        "source line range": r"`(?:internal/audit|cmd/invowk)/[^`]+\.go:\d+(?:-\d+)?`",
    }
    for label, pattern in volatile_patterns.items():
        if re.search(pattern, implementation, flags=re.IGNORECASE):
            errors.append(f"implementation-review.md contains stale-prone {label}")

    if errors:
        for error in errors:
            print(f"error: {error}", file=sys.stderr)
        return 1
    print(f"OK: documented {len(checkers)} default audit checkers and live source references")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
