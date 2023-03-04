# dotgitreview

This package provides a simple wrapper around `.gitreview` files.

`.gitreview` is a file that is used by the [Gerrit Code Review](https://www.gerritcodereview.com/) system to configure the git repository. It is used to configure the remote repository and the branch to push to.

The `.gitreview` file is located in the root of the repository and is used by the `git review` command.

## Example

```
[gerrit]
host=review.example.com
port=29418
project=example.git
defaultbranch=master
```

## References

* [Gerrit Code Review](https://www.gerritcodereview.com/)
* [git-review](https://pypi.org/project/git-review/)
