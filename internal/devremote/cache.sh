set -euo pipefail
mode=$1
shift
root=$HOME/.cache/a-wikimedia-cli
if ! test -d "$root"; then exit 0; fi
if [[ -L $root || -L $root/bin || -L $root/code-server ]]; then
    echo 'Refusing cache cleanup through a symlink.' >&2
    exit 1
fi
exec 9>"$root/remote-cache.lock"
flock 9
shopt -s nullglob
metadata=("$HOME"/wm/*/.wm "$HOME"/wm/*/.mw)
executables=()
for process in /proc/[0-9]*/exe; do
    executable=$(readlink "$process" 2>/dev/null) || continue
    executables+=("$executable")
done
used() {
    local candidate=$1 item binary
    for item in "${metadata[@]}"; do
        for binary in "$item/binary" "$item/editor-cache" "$item/bin/mw"; do
            if test -f "$binary" && grep -Fq -- "${candidate##*/}" "$binary"; then return 0; fi
        done
        # Old editor installations have no reference manifest: retain them conservatively.
        if [[ $candidate == "$root/code-server/"* ]] && test -f "$item/editor-config.yaml" && ! test -f "$item/editor-cache"; then return 0; fi
    done
    for binary in "${executables[@]}"; do
        if [[ $binary == "$candidate" || $binary == "$candidate/"* ]]; then return 0; fi
    done
    return 1
}
for candidate in "$root"/bin/mw-* "$root"/code-server/*; do
    if [[ -L $candidate ]]; then continue; fi
    name=${candidate##*/}
    if [[ $candidate == "$root/bin/"* ]]; then
        [[ $name =~ ^mw-[a-f0-9]{64}$ ]] && test -f "$candidate" || continue
    else
        [[ $name =~ ^[0-9]+\.[0-9]+\.[0-9]+-(amd64|arm64)$ ]] && test -d "$candidate" || continue
    fi
    if used "$candidate"; then continue; fi
    if [[ $mode == clean ]]; then
        approved=no
        for item in "$@"; do if [[ $item == "$candidate" ]]; then approved=yes; break; fi; done
        [[ $approved == yes ]] || continue
        rm -rf -- "$candidate"
    fi
    printf '%s\0' "$candidate"
done
