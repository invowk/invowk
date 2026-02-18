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
# produces no changes. When only some fields are missing, new fields are
# inserted after PackageVersion regardless of existing field positions.

import sys


def enhance(filepath):
    try:
        with open(filepath) as f:
            lines = f.readlines()
    except (OSError, IOError) as e:
        print(f"ERROR: Cannot read manifest file: {filepath}", file=sys.stderr)
        print(f"  {e}", file=sys.stderr)
        sys.exit(1)

    if not any(l.startswith("ManifestType: installer") for l in lines):
        print(
            f"ERROR: File does not appear to be a WinGet installer manifest: {filepath}",
            file=sys.stderr,
        )
        sys.exit(1)

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

    # Verify all required fields are present in the output.
    output_text = "".join(result)
    for field in ("MinimumOSVersion:", "Platform:", "Commands:"):
        if field not in output_text:
            print(
                f"ERROR: Failed to inject '{field}' — anchor line not found in manifest.",
                file=sys.stderr,
            )
            print(
                "  This likely means GoReleaser's manifest format has changed.",
                file=sys.stderr,
            )
            sys.exit(1)

    try:
        with open(filepath, "w") as f:
            f.writelines(result)
    except (OSError, IOError) as e:
        print(f"ERROR: Cannot write manifest file: {filepath}", file=sys.stderr)
        print(f"  {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: enhance_winget_fields.py <manifest.yaml>", file=sys.stderr)
        sys.exit(1)
    enhance(sys.argv[1])
