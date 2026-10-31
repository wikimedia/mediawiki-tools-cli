#!/usr/bin/env bash
# Publish an update generated from the latest default-branch checkout.
set -euo pipefail

if git diff --quiet && git diff --cached --quiet; then
    echo 'Skipped image update: no image changes remain on the default branch'
    exit 0
fi

bot_email=project16_bot@noreply.gitlab.wikimedia.org
push_remote=${1:-origin}
base=$(git rev-parse HEAD)
branch=$(printf '%s\n' "$DESCRIPTION" | sed -r 's/ [^:]+:/ to /g')
branch=${branch// /_}
branch=${branch//:/_}
git check-ref-format --branch "$branch" >/dev/null
ref=refs/heads/$branch

skip() {
    printf 'Skipped %s: %s\n' "$branch" "$1"
    if [[ -n ${CI_PROJECT_URL:-} ]]; then
        printf 'Existing merge requests: %s/-/merge_requests?scope=all&state=opened&source_branch=%s\n' "$CI_PROJECT_URL" "$branch"
    fi
    exit 0
}

# An empty expected SHA also protects creation: a concurrently created branch
# must not be overwritten. Fetch the existing branch before checking ownership.
expected=
remote_head=$(git ls-remote --heads origin "$ref")
if [[ -n $remote_head ]]; then
    git fetch origin "$ref"
    expected=$(git rev-parse FETCH_HEAD)
    # Both identities matter: preserve human amendments and cherry-picks too.
    identities=$(git log --format='%ae %ce' "$expected" --not "$base")
    while IFS= read -r identity; do
        if [[ -n $identity && $identity != "$bot_email $bot_email" ]]; then
            skip 'branch contains commits not authored and committed by the image-update bot'
        fi
    done <<< "$identities"
fi

git checkout -B "$branch" "$base"
git -c user.email="$bot_email" -c user.name=image-update-ci commit -a -m "$DESCRIPTION"
if git push --force-with-lease="$ref:$expected" \
    -o merge_request.title="$DESCRIPTION" \
    -o merge_request.label=dependencies -o merge_request.label=docker \
    -o merge_request.create "$push_remote" "HEAD:$ref"; then
    exit 0
else
    status=$?
    # Only a changed remote tip is an expected skip. Authentication, network,
    # and server errors must still fail the job.
    remote_head=$(git ls-remote --heads origin "$ref")
    actual=${remote_head%%$'\t'*}
    if [[ $actual != "$expected" ]]; then
        skip 'remote branch changed during this run; newer work was left untouched'
    fi
    exit "$status"
fi
