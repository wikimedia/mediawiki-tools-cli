set -euo pipefail
context=$1
workspace=$2
metadata=$3
expected=$HOME/wm/$context
if [[ $workspace != "$expected" || $metadata != "$expected/.wm" || -L $HOME/wm || -L $workspace || -L $metadata ]]; then
    echo 'Refusing to delete a workspace outside its expected ~/wm/CONTEXT location or through a symlink.' >&2
    exit 1
fi
if ! test -e "$workspace"; then exit 0; fi
if test -f "$metadata/context"; then
    if [[ $(cat "$metadata/context") != "$context" ]]; then
        echo 'The workspace belongs to a different context; nothing was deleted.' >&2
        exit 1
    fi
elif ! test -x "$metadata/bin/mw" || ! test -f "$metadata/recipe" || ! test -f "$metadata/port"; then
    echo 'The workspace is not identifiable as a CLI-managed environment; nothing was deleted.' >&2
    exit 1
fi
exec 9>"$metadata/setup.lock"
flock -w 15 9 || { echo 'Remote setup is still running; retry cleanup when it finishes.' >&2; exit 1; }
# Teardown must succeed before removing the checkout and its recovery metadata.
if test -x "$metadata/bin/mw"; then
    "$metadata/bin/mw" --no-interaction dev destroy
    "$metadata/bin/mw" --no-interaction dev vscode stop
fi
rm -rf -- "$workspace"
printf 'Deleted remote workspace: %s\n' "$workspace"
