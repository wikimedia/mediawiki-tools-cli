#!/usr/bin/env bash
# Offline end-to-end checks for mw dev mediawiki update-code.
set -euo pipefail
MWCLI_TEST_BINARY=$(realpath "${1:-./bin/mw}")
MWCLI_UPDATE_TEST_DIR=$(mktemp -d)
trap 'rm -rf "$MWCLI_UPDATE_TEST_DIR"' EXIT
unset GITLAB_CI CI_PROJECT_DIR GIT_DIR GIT_WORK_TREE
export XDG_CONFIG_HOME="$MWCLI_UPDATE_TEST_DIR/config"
export MWCLI_CONTEXT_TEST=1
export GIT_CONFIG_GLOBAL="$MWCLI_UPDATE_TEST_DIR/no-config"
export GIT_CONFIG_NOSYSTEM=1
export MWCLI_TEST_DOCKER_MARKER="$MWCLI_UPDATE_TEST_DIR/docker-called"
CODE="$MWCLI_UPDATE_TEST_DIR/core"
mkdir -p "$CODE" "$XDG_CONFIG_HOME/mwcli/mwdd/default" "$MWCLI_UPDATE_TEST_DIR/bin"
repo_git() { git -C "$CODE" -c user.name=Fixture -c user.email=fixture@example.test "$@"; }
repo_git init -b main >/dev/null
printf '[gerrit]\nproject=mediawiki/core\n' > "$CODE/.gitreview"
repo_git add .gitreview
repo_git commit -m fixture >/dev/null
repo_git remote add origin "$MWCLI_UPDATE_TEST_DIR/absent-remote.git"
repo_git update-ref refs/remotes/origin/main HEAD
repo_git config branch.main.remote origin
repo_git config branch.main.merge refs/heads/main
printf '{"telemetry":"no","timer_last_update_checked":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$XDG_CONFIG_HOME/mwcli/config.json"
printf 'MEDIAWIKI_VOLUMES_CODE=%s\n' "$CODE" > "$XDG_CONFIG_HOME/mwcli/mwdd/default/.env"
for command in docker docker-compose; do
    cat > "$MWCLI_UPDATE_TEST_DIR/bin/$command" <<'DOCKER'
#!/bin/sh
touch "$MWCLI_TEST_DOCKER_MARKER"
exit 99
DOCKER
    chmod +x "$MWCLI_UPDATE_TEST_DIR/bin/$command"
done
export PATH="$MWCLI_UPDATE_TEST_DIR/bin:$PATH"
run_update() {
    if ! "$MWCLI_TEST_BINARY" dev mediawiki update-code --context=default --no-interaction "$@" > "$MWCLI_UPDATE_TEST_DIR/output" 2>&1; then
        cat "$MWCLI_UPDATE_TEST_DIR/output"
        return 1
    fi
    test ! -e "$MWCLI_TEST_DOCKER_MARKER"
}
for mode in ff-only stash mop; do
    run_update --mode="$mode" --dry-run
    run_update --mode="$mode" --dry-run --composer
    echo "PASS: Docker-free $mode previews, including Composer preview"
done
if "$MWCLI_TEST_BINARY" dev mediawiki update-code --context=default --no-interaction > "$MWCLI_UPDATE_TEST_DIR/output" 2>&1; then
    echo 'FAIL: update unexpectedly fetched a missing remote'
    exit 1
fi
grep -q absent-remote "$MWCLI_UPDATE_TEST_DIR/output"
test ! -e "$MWCLI_TEST_DOCKER_MARKER"
echo 'PASS: Git-only execution reaches fetch with Docker unavailable'

# Exercise actual updates using a local bare remote.
REMOTE="$MWCLI_UPDATE_TEST_DIR/remote.git"
SEED="$MWCLI_UPDATE_TEST_DIR/seed"
git clone --bare "$CODE" "$REMOTE" >/dev/null 2>&1
repo_git remote set-url origin "$REMOTE"
git clone "$REMOTE" "$SEED" >/dev/null 2>&1
seed_git() { git -C "$SEED" -c user.name=Fixture -c user.email=fixture@example.test "$@"; }
advance_remote() {
    printf 'remote update\n' > "$SEED/$1"
    seed_git add "$1"
    seed_git commit -m "$1" >/dev/null
    seed_git push >/dev/null 2>&1
}
advance_remote first-remote-update
run_update --mode=ff-only
test -f "$CODE/first-remote-update"
echo 'PASS: CLI fast-forward update'
printf 'staged\n' > "$CODE/local changes"
repo_git add 'local changes'
printf 'unstaged\n' > "$CODE/local changes"
printf 'local data\n' > "$CODE/untracked file"
advance_remote second-remote-update
run_update --mode=stash
test -f "$CODE/second-remote-update"
test "$(cat "$CODE/local changes")" = unstaged
test "$(repo_git show ':local changes')" = staged
test "$(cat "$CODE/untracked file")" = 'local data'
echo 'PASS: CLI stash preserves staged, unstaged and untracked changes'
repo_git checkout -b feature >/dev/null 2>&1
run_update --mode=mop
test "$(repo_git symbolic-ref --short HEAD)" = main
test -z "$(repo_git status --porcelain)"
test ! -e "$CODE/local changes"
test ! -e "$CODE/untracked file"
test -n "$(repo_git stash list)"
echo 'PASS: CLI mop switches to main and leaves changes saved'
