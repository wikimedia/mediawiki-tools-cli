set -euo pipefail
umask 077
workspace=$1
binary=$2
context=$3
recipe=$4
port=$5
source=${6:-$workspace/mediawiki}
bridge=${7:-}
root=${8:-$workspace}
git_bridge=${9:-no}
git_remote=${10:-}
mkdir -p "$workspace/bin" "$workspace/config"
exec 9>"$workspace/setup.lock"
flock -n 9 || { echo 'Another setup is running for this context.' >&2; exit 1; }
if test -f "$workspace/port" && test "$(cat "$workspace/port")" != "$port"; then
    echo 'This context already uses another port; reconnect using its saved port.' >&2
    exit 1
fi
if test -f "$workspace/recipe" && test "$(cat "$workspace/recipe")" != "$recipe"; then
    echo 'This context already uses another recipe; select another --context.' >&2
    exit 1
fi
printf '%s\n' "$port" > "$workspace/port"
printf '%s\n' "$recipe" > "$workspace/recipe"
printf '%s\n' "$context" > "$workspace/context"
printf '%s\n' "$binary" > "$workspace/binary"

# Keep the remote configuration separate from any existing CLI installation.
launcher=$(mktemp "$workspace/bin/launcher.XXXXXX")
trap 'rm -f "$launcher"' EXIT
printf '#!/bin/bash\nexport XDG_CONFIG_HOME=%q\nexport CONTEXT=%q\nexport MW_REMOTE_BRIDGE=%q\nexport MW_REMOTE_WORKSPACE=%q\nexport MW_REMOTE_METADATA=%q\nexport MW_REMOTE_SOURCE=%q\nexec %q "$@"\n' \
    "$workspace/config" "$context" "$bridge" "$root" "$workspace" "$source" "$binary" > "$launcher"
chmod 700 "$launcher"
mv "$launcher" "$workspace/bin/mw"
mw="$workspace/bin/mw"

# Container users must be able to read generated bind mounts and checkouts.
# The enclosing workspace/config directories remain private to the SSH user.
umask 022
"$mw" --no-interaction dev env set PORT "$port"
"$mw" --no-interaction dev env set TRAEFIK_BIND_ADDR 127.0.0.1
"$mw" --no-interaction dev env set MEDIAWIKI_VOLUMES_CODE "$source"
if [[ $git_bridge == yes ]]; then
    real_git=$(command -v git)
    printf '#!/bin/bash\nexport MW_REMOTE_BRIDGE=%q\nargs=("$@")\nwhile (($#)); do\n case "$1" in\n -C|-c|--git-dir|--work-tree) shift 2;;\n -*) shift;;\n fetch|push|pull|clone) exec %q __remote-git "${args[@]}";;\n *) break;;\n esac\ndone\nexec %q "${args[@]}"\n' "$bridge" "$binary" "$real_git" > "$workspace/bin/git"
    chmod 700 "$workspace/bin/git"
    if test -d "$source/.git"; then
        "$real_git" -C "$source" remote set-url origin "$git_remote"
    fi
else
    rm -f "$workspace/bin/git"
fi
if [[ $recipe == none ]]; then
    mkdir -p "$source"
    touch "$workspace/ready"
    exit 0
fi
if ! test -f "$workspace/ready"; then
    "$mw" --no-interaction dev recipe "$recipe"
    "$mw" --no-interaction dev start --remember-running
    touch "$workspace/ready"
else
    "$mw" --no-interaction dev start --remember-running
fi

# Retain the active executable without accumulating previous development builds.
for previous in "$workspace"/bin/mw-*; do
    if [[ $previous != "$binary" && ${previous##*/} =~ ^mw-[a-f0-9]{64}$ ]]; then
        rm -f -- "$previous"
    fi
done
