#!/usr/bin/env bash
set -euo pipefail

if [[ -z ${TOOL:-} ]]; then
    echo "Use TOOL=your-tool $0" >&2
    exit 1
fi
export TOOL

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cd "$script_dir/../../.."

# The tape types mw, so build and select that executable explicitly.
make build BINARY_NAME=mw
export PATH="$PWD/bin:$PATH"

existing=$(bin/mw tools kubernetes get pod cli-guide-hello --ignore-not-found -o name --tool "$TOOL")
if [[ -n $existing ]]; then
    echo 'cli-guide-hello already exists; choose another tool or finish that demo first.' >&2
    exit 1
fi

mkdir -p _recordings
awk '/^cat > cli-guide-hello.yaml / { copying=1; next } copying && /^YAML$/ { exit } copying { print }' "$script_dir/kubernetes-pod-helloworld.md" > _recordings/cli-guide-hello.yaml
test -s _recordings/cli-guide-hello.yaml

"${MISE:-mise}" exec vhs@0.12.1 ttyd@1.7.7 -- vhs "$script_dir/kubernetes-pod-helloworld.tape"
