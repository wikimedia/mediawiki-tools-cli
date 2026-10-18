# Contributing to a Wikimedia CLI

Contributions to a Wikimedia CLI are welcome.

## Getting Started

1.  **Find an issue**: Check our [Phabricator board](https://phabricator.wikimedia.org/project/view/5331/) to find something to work on.
2.  **Request branch access**: Full integration CI is enabled for the main GitLab repository. To create branches there, request developer access. [File a ticket on Phabricator](https://phabricator.wikimedia.org/maniphest/task/edit/form/1/?tags=mwcli&title=Request%20access%20to%20mwcli%20gitlab%20project%20for%20%3CUSER%3E) to request access.
3.  **Follow the Development Guide**: See [DEVELOPMENT.md](DEVELOPMENT.md) for instructions on setting up your environment, building, and testing.

## Contribution Workflow

### GitLab and CI

We use [Wikimedia GitLab](https://gitlab.wikimedia.org/repos/releng/cli) for code hosting and CI.

- MR pipelines test the branch merged with its target.
- Full integration CI is enabled for the main repository. Fork pipelines run shared-runner checks; GitHub integration tests are limited to the upstream project.
- See [CI.md](CI.md) for more details on our CI setup.

### Merge Requests

1.  **Create a branch**: Create a branch in the main repository for full integration CI; a draft MR is enough to start it.
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
