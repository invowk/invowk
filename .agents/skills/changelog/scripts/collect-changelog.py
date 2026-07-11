#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Collect deterministic commit-derived changelog inputs as JSON."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path


def git(*args: str, check: bool = True) -> str:
    result = subprocess.run(
        ["git", *args], check=False, text=True, capture_output=True
    )
    if check and result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or "git command failed")
    return result.stdout.rstrip("\n")


def latest_semver_tag() -> str | None:
    tags = git("tag", "--list", "v[0-9]*", "--sort=-v:refname").splitlines()
    return tags[0] if tags else None


def github_slug() -> str | None:
    origin = git("remote", "get-url", "origin", check=False).strip()
    prefixes = ("git@github.com:", "https://github.com/", "ssh://git@github.com/")
    for prefix in prefixes:
        if origin.startswith(prefix):
            slug = origin.removeprefix(prefix).removesuffix(".git")
            return slug if slug.count("/") == 1 else None
    return None


def parse_commits(log_range: str) -> list[dict[str, str]]:
    output = git(
        "log",
        "--no-merges",
        "--format=%H%x00%s%x00%b%x00%aN%x1e",
        log_range,
    )
    commits = []
    for record in output.split("\x1e"):
        record = record.strip("\n")
        if not record:
            continue
        fields = record.split("\x00", 3)
        if len(fields) != 4:
            raise RuntimeError("unexpected git log record")
        commit_hash, subject, body, author = fields
        commits.append(
            {"hash": commit_hash, "subject": subject, "body": body, "author": author}
        )
    return commits


def collect(target: str) -> dict[str, object]:
    git("rev-parse", "--verify", target)
    latest_tag = latest_semver_tag()
    if latest_tag:
        log_range = f"{latest_tag}..{target}"
        diff_range = log_range
    else:
        empty_tree = git("hash-object", "-t", "tree", "/dev/null")
        log_range = target
        diff_range = f"{empty_tree}..{target}"

    commits = parse_commits(log_range)
    slug = github_slug()
    compare_url = (
        f"https://github.com/{slug}/compare/{latest_tag}...{target}"
        if latest_tag and slug
        else None
    )
    return {
        "target": target,
        "latest_tag": latest_tag,
        "log_range": log_range,
        "diff_range": diff_range,
        "compare_url": compare_url,
        "shortstat": git("diff", "--shortstat", diff_range).strip(),
        "contributors": sorted({commit["author"] for commit in commits}),
        "commits": commits,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", default="HEAD", help="Git ref to summarize")
    parser.add_argument("--output", type=Path, help="Write JSON to this file")
    args = parser.parse_args()
    try:
        rendered = json.dumps(collect(args.target), indent=2, sort_keys=True) + "\n"
    except RuntimeError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    else:
        sys.stdout.write(rendered)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
