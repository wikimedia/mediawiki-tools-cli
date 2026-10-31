# Development

- Read nearby code and follow existing patterns before adding new abstractions.
- Reuse shared helpers for profiles, credentials, output, browser launching and Cobra commands.

# Verification

- Check relevant tests and the basic CI checks: `make test`, `make lint`, `make linti` and `make vet`.
- Build for the current platform during normal development. Check other OS/architectures only when the change warrants it; leave the release matrix to CI.

# Documentation

- Do not create unnecessary documentation or Markdown files.
- Prefer updating relevant existing documentation, don't automatically expand the scope.
- Document only useful, lasting information about the code and its behavior.
- Never put worklogs, implementation progress, session notes or accounts of what we are currently doing into repository documentation.