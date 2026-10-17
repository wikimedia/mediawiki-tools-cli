#!/usr/bin/env bash
set -euo pipefail

# Fetch from the target project even when the source branch lives in a fork.
git fetch --no-tags -- "${CI_MERGE_REQUEST_PROJECT_URL}.git" "$CI_MERGE_REQUEST_TARGET_BRANCH_NAME"
target_commit=$(git rev-parse FETCH_HEAD)
git checkout --detach "$CI_COMMIT_SHA"
echo "Merging target $target_commit into MR source $CI_COMMIT_SHA"

if ! git -c user.name='CI merge' -c user.email='ci@localhost' \
    merge --no-ff --no-edit "$target_commit"; then
  echo "Cannot merge the target branch. Resolve the conflicts in your MR branch and push again."
  exit 1
fi

echo "Testing merged commit $(git rev-parse HEAD)"
# Include history so consumers work with shallow checkouts too. Checking out the
# bundle preserves tracked deletions and a clean Git index for git-state checks.
git bundle create .ci-merge.bundle HEAD
