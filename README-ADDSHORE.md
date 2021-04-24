# MediaWiki CLI (Addshore Dev Fork)

## Branches

- master, Kept in sync with the upstream master branch that is maintained on Wikimedia Gerrit
- gerrit, Kept in sync with the chain of commits to review on Gerrit
- dev, Where Addshore does development

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
