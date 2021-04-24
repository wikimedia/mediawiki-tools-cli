# MediaWiki CLI (Addshore Dev Fork)

This is a fork of a repository actually maintained on the Wikimedia Gerrit server.

To get setup I recommend:

```sh
mkdir ~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli
cd ~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli

git clone ssh://addshore@gerrit.wikimedia.org:29418/mediawiki/tools/cli .
git remote add github git@github.com:addshore/mwcli.git
git fetch github

git checkout -b dev --track github/dev
git checkout -b gerrit --track github/gerrit
```

## Branches

- `master`, Kept in sync with the upstream master branch that is maintained on Wikimedia Gerrit
- `gerrit`, Kept in sync with the chain of commits to review on Gerrit
- `dev`, Where Addshore does development

## Getting changes from dev to gerrit branch

Development should happen on `dev`.

Changes can then be pulled from this branches HEAD, or any point along the branch, and new commits can be applied to gerrit.

```sh
git diff gerrit dev -- . ':!.github' ':!README-ADDSHORE.md' | git apply

git diff gerrit e95cba358c0d0dd02b725ac68fdc988efb969164 -- . ':!.github' ':!README-ADDSHORE.md' | git apply
```

Commits can then be made and submitted to gerrit.

## Getting changes from master to dev

```sh
git checkout dev
git pull origin master
git push github dev
```

## Updating the gerrit branch

If things have changed in the patches on gerrit, then you may need to update the gerrit branch of this fork.

For example:

```sh
git checkout gerrit
git fetch "ssh://addshore@gerrit.wikimedia.org:29418/mediawiki/tools/cli" refs/changes/40/682240/3
git reset --hard FETCH_HEAD
git push github gerrit --force-with-lease
```
