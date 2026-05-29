#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Shared helpers for release notes validation and tag-message extraction.
# Intended to be sourced by release scripts and GitHub Actions shell steps.

release_notes_error() {
    printf 'ERROR: %s\n' "$*" >&2
}

validate_release_notes_file() {
    local release_notes_file="${1:-}"
    local lower_path

    if [[ -z "$release_notes_file" ]]; then
        release_notes_error "RELEASE_NOTES_FILE=<path> is required."
        return 1
    fi

    if [[ ! -e "$release_notes_file" ]]; then
        release_notes_error "Release notes file does not exist: $release_notes_file"
        return 1
    fi

    if [[ ! -f "$release_notes_file" ]]; then
        release_notes_error "Release notes input must be a regular markdown file: $release_notes_file"
        return 1
    fi

    lower_path="$(printf '%s' "$release_notes_file" | tr '[:upper:]' '[:lower:]')"
    case "$lower_path" in
        *.md|*.markdown) ;;
        *)
            release_notes_error "Release notes input must be a markdown file (.md or .markdown): $release_notes_file"
            return 1
            ;;
    esac

    if [[ ! -s "$release_notes_file" ]]; then
        release_notes_error "Release notes file cannot be empty: $release_notes_file"
        return 1
    fi
}

extract_release_notes_from_tag() {
    local tag_name="${1:-}"
    local output_file="${2:-}"

    if [[ -z "$tag_name" ]]; then
        release_notes_error "Release tag name is required."
        return 1
    fi

    if [[ -z "$output_file" ]]; then
        release_notes_error "Release notes output file is required."
        return 1
    fi

    if ! git cat-file -e "${tag_name}^{tag}" 2>/dev/null; then
        release_notes_error "Release tag '$tag_name' must be an annotated tag containing release notes."
        return 1
    fi

    git cat-file tag "$tag_name" \
        | sed '1,/^$/d' \
        | awk '/^-----BEGIN (PGP|SSH) SIGNATURE-----$/ { exit } { print }' \
        > "$output_file"

    if [[ ! -s "$output_file" ]]; then
        release_notes_error "Release tag '$tag_name' does not contain release notes."
        return 1
    fi
}
