#!/usr/bin/env bash
set -euo pipefail

# Fetch the target project explicitly: origin can be a fork with a different main.
git fetch --no-tags -- "${CI_MERGE_REQUEST_PROJECT_URL}.git" "$CI_MERGE_REQUEST_TARGET_BRANCH_NAME"
target_commit=$(git rev-parse FETCH_HEAD)

if ! git merge-base --is-ancestor "$target_commit" "$CI_COMMIT_SHA"; then
  echo "The MR branch must include the latest $CI_MERGE_REQUEST_TARGET_BRANCH_NAME before CI runs."
  echo "Rebase or merge the target branch into your source branch, then push to rerun the MR pipeline."
  echo "Use Rebase in GitLab, or fetch the target branch and run git rebase locally."
  exit 1
fi
