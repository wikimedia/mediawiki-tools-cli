#!/usr/bin/env bash
# Keep release failures intact while making runner storage problems visible.
set -euo pipefail

disk_report() {
    echo "Release build disk usage ($1):" >&2
    df -h . "${GOTMPDIR:-${TMPDIR:-/tmp}}" >&2 || true
    echo "Release build inode usage ($1):" >&2
    df -i . "${GOTMPDIR:-${TMPDIR:-/tmp}}" >&2 || true
}

disk_report before
if make release "$@"; then
    exit 0
else
    status=$?
    echo "Release build failed (exit $status). Check free space and inodes below if Go reported 'no space left on device'." >&2
    disk_report failed
    for directory in "${GOCACHE:-}" "${GOMODCACHE:-}" "${RELEASE_DIR:-_release}"; do
        if [[ -n "$directory" && -d "$directory" ]]; then
            du -sh -- "$directory" >&2 || true
        fi
    done
    echo "Platform concurrency defaults to 4; lower RELEASE_PARALLEL if storage pressure persists. Shared runner capacity and cache retention may also need attention." >&2
    exit "$status"
fi
