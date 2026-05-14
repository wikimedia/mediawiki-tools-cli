# Contributing to mwcli

Thank you for your interest in contributing to `mwcli`! We welcome contributions from everyone.

## Getting Started

1.  **Find an issue**: Check our [Phabricator board](https://phabricator.wikimedia.org/project/view/5331/) to find something to work on.
2.  **Request Access**: If you want to contribute code, you'll need developer access to the GitLab repository. [File a ticket on Phabricator](https://phabricator.wikimedia.org/maniphest/task/edit/form/1/?tags=mwcli&title=Request%20access%20to%20mwcli%20gitlab%20project%20for%20%3CUSER%3E) to request access.
3.  **Follow the Development Guide**: See [DEVELOPMENT.md](DEVELOPMENT.md) for instructions on setting up your environment, building, and testing.

## Contribution Workflow

### GitLab and CI

We use [Wikimedia GitLab](https://gitlab.wikimedia.org/repos/releng/cli) for code hosting and CI.

- **CI Constraints**: CI only runs for branches created within the main repository. It will **not** run for Merge Requests from forks.
- Once you have developer access, please create a branch in the main repository for your changes to ensure CI runs.
- See [CI.md](CI.md) for more details on our CI setup.

### Merge Requests

1.  **Create a branch**: Ensure you create the branch within the main repository (not a fork) so that CI will run.
2.  **Commit your changes**: Follow [standard Git commit conventions](https://www.conventionalcommits.org/).
3.  **Push and open MR**: Push your branch and open a Merge Request (MR) on GitLab.
4.  **Verify CI**: Ensure all CI checks pass.
5.  Wait for a maintainer to review your MR.

## Coding Standards

- Follow standard Go idioms and formatting (`go fmt`).
- Write tests for new features and bug fixes.
- Keep documentation up to date.
- Use the `internal/cmdgloss` package for user-facing output to maintain consistency.

## Support

- IRC: `#mediawiki` on [Libera.Chat](https://libera.chat/)
- Phabricator: [#mwcli](https://phabricator.wikimedia.org/project/view/5331/)
