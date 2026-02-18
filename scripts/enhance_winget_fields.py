#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
#
# Inject missing WinGet manifest fields into a GoReleaser-generated installer
# manifest. Inserts Commands, MinimumOSVersion, and Platform at the correct
# positions while preserving the existing formatting.
#
# Usage: python3 scripts/enhance_winget_fields.py <manifest.yaml>
#
# This script is idempotent — running it on an already-enhanced manifest
# produces no changes.

import sys


def enhance(filepath):
    with open(filepath) as f:
        lines = f.readlines()

    result = []
    has_min_os = any(l.startswith("MinimumOSVersion:") for l in lines)
    has_platform = any(l.startswith("Platform:") for l in lines)
    has_commands = any(l.startswith("Commands:") for l in lines)

    for line in lines:
        # Insert MinimumOSVersion and Platform after PackageVersion
        if line.startswith("PackageVersion:") and (not has_min_os or not has_platform):
            result.append(line)
            if not has_min_os:
                result.append("MinimumOSVersion: 10.0.17763.0\n")
            if not has_platform:
                result.append("Platform:\n")
                result.append("  - Windows.Desktop\n")
            continue
        # Insert Commands before Installers
        if line.startswith("Installers:") and not has_commands:
            result.append("Commands:\n")
            result.append("  - invowk\n")
            result.append(line)
            continue
        result.append(line)

    with open(filepath, "w") as f:
        f.writelines(result)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: enhance_winget_fields.py <manifest.yaml>", file=sys.stderr)
        sys.exit(1)
    enhance(sys.argv[1])
