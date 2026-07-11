#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Check that a release note poetic opening was not reused."""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
import unicodedata
from pathlib import Path


REQUIRED_SECTIONS = (
    "Poetic Opening",
    "What's Changed",
    "Manual Actions Needed",
    "Warnings and Deprecations",
    "Bug Fixes",
)
HEADING_RE = re.compile(r"^##\s+(.+?)\s*$", re.MULTILINE)
SIGNATURE_RE = re.compile(r"^-----BEGIN (?:PGP|SSH) SIGNATURE-----$", re.MULTILINE)


def run(args: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(args, check=False, text=True, capture_output=True)


def required_sections(markdown: str) -> dict[str, str]:
    headings = list(HEADING_RE.finditer(markdown))
    required_lookup = {name.casefold(): name for name in REQUIRED_SECTIONS}
    found: dict[str, list[tuple[int, str]]] = {name: [] for name in REQUIRED_SECTIONS}
    for index, heading in enumerate(headings):
        canonical = required_lookup.get(heading.group(1).strip().casefold())
        if not canonical:
            continue
        end = headings[index + 1].start() if index + 1 < len(headings) else len(markdown)
        found[canonical].append((heading.start(), markdown[heading.end() : end].strip()))

    errors = []
    for name in REQUIRED_SECTIONS:
        entries = found[name]
        if not entries:
            errors.append(f"missing required section: ## {name}")
        elif len(entries) > 1:
            errors.append(f"duplicate required section: ## {name}")
        elif not entries[0][1]:
            errors.append(f"empty required section: ## {name}")
    positions = [found[name][0][0] for name in REQUIRED_SECTIONS if len(found[name]) == 1]
    if len(positions) == len(REQUIRED_SECTIONS) and positions != sorted(positions):
        errors.append("required sections are out of order")
    if errors:
        raise ValueError("; ".join(errors))
    return {name: found[name][0][1] for name in REQUIRED_SECTIONS}


def extract_poetic_opening(markdown: str) -> str:
    headings = list(HEADING_RE.finditer(markdown))
    section = ""
    for index, heading in enumerate(headings):
        if heading.group(1).strip().casefold() != "poetic opening":
            continue
        end = headings[index + 1].start() if index + 1 < len(headings) else len(markdown)
        section = markdown[heading.end() : end].strip()
        break
    if not section:
        return ""
    quoted_lines = []
    for line in section.splitlines():
        stripped = line.strip()
        if stripped.startswith(">"):
            quoted_lines.append(stripped.lstrip(">").strip())
    return "\n".join(quoted_lines).strip() or section


def normalize(text: str) -> str:
    text = re.sub(r"<!--.*?-->", " ", text, flags=re.DOTALL)
    text = unicodedata.normalize("NFKD", text)
    text = "".join(ch for ch in text if not unicodedata.combining(ch))
    text = text.casefold()
    text = re.sub(r"[^a-z0-9]+", " ", text)
    return re.sub(r"\s+", " ", text).strip()


def tag_message(tag: str) -> str | None:
    exists = run(["git", "cat-file", "-e", f"{tag}^{{tag}}"])
    if exists.returncode != 0:
        return None
    result = run(["git", "cat-file", "tag", tag])
    if result.returncode != 0:
        return None
    _, _, body = result.stdout.partition("\n\n")
    body = SIGNATURE_RE.split(body, maxsplit=1)[0]
    return body.strip()


def local_tag_openings() -> list[tuple[str, str]]:
    result = run(["git", "tag", "--list", "v[0-9]*"])
    if result.returncode != 0:
        return []
    openings = []
    for tag in result.stdout.splitlines():
        body = tag_message(tag)
        if not body:
            continue
        opening = extract_poetic_opening(body)
        if opening:
            openings.append((f"tag:{tag}", opening))
    return openings


def github_release_openings(repo: str, limit: int) -> list[tuple[str, str]]:
    if not shutil.which("gh"):
        print("warning: gh not found; skipped GitHub Release body check", file=sys.stderr)
        return []
    result = run(
        ["gh", "release", "list", "--repo", repo, "--limit", str(limit), "--json", "tagName"]
    )
    if result.returncode != 0:
        print("warning: gh release list failed; skipped GitHub Release body check", file=sys.stderr)
        return []
    try:
        releases = json.loads(result.stdout)
    except json.JSONDecodeError:
        print("warning: could not parse gh release list output", file=sys.stderr)
        return []
    openings = []
    for release in releases:
        tag = release.get("tagName")
        if not tag:
            continue
        view = run(["gh", "release", "view", tag, "--repo", repo, "--json", "body"])
        if view.returncode != 0:
            continue
        try:
            body = json.loads(view.stdout).get("body", "")
        except json.JSONDecodeError:
            continue
        opening = extract_poetic_opening(body)
        if opening:
            openings.append((f"github:{tag}", opening))
    return openings


def is_duplicate(candidate: str, previous: str) -> bool:
    candidate_norm = normalize(candidate)
    previous_norm = normalize(previous)
    if len(candidate_norm) < 40 or len(previous_norm) < 40:
        return False
    return (
        candidate_norm == previous_norm
        or candidate_norm in previous_norm
        or previous_norm in candidate_norm
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("release_notes", type=Path, help="Markdown release notes file to check")
    parser.add_argument(
        "--repo", default="invowk/invowk", help="GitHub repo for release body checks"
    )
    parser.add_argument("--limit", type=int, default=200, help="GitHub releases to inspect")
    parser.add_argument("--skip-gh", action="store_true", help="Skip GitHub Release body checks")
    args = parser.parse_args()

    try:
        candidate_body = args.release_notes.read_text(encoding="utf-8")
    except OSError as exc:
        print(f"error: read release notes: {exc}", file=sys.stderr)
        return 2

    try:
        required_sections(candidate_body)
    except ValueError as exc:
        print(f"error: invalid release-note structure: {exc}", file=sys.stderr)
        return 2

    candidate = extract_poetic_opening(candidate_body)
    if not candidate:
        print("error: no Poetic Opening content found", file=sys.stderr)
        return 2
    openings = local_tag_openings()
    if not args.skip_gh:
        openings.extend(github_release_openings(args.repo, args.limit))

    for source, previous in openings:
        if is_duplicate(candidate, previous):
            print(f"error: poetic opening appears to duplicate {source}", file=sys.stderr)
            return 1

    print(f"OK: poetic opening is unique across {len(openings)} checked prior release notes")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
