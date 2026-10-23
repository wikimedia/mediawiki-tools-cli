# Writing command help and guides

Put information about a command's behavior and operation in its help. Use a
guide to walk through a task, combine commands, or explore how features work
together. Readers should be able to understand a command and its consequences
from `--help` without first reading a guide.

## Command help

Use `Short` for a one-line description of the command's purpose. Use `Use`,
`Args`, and flag descriptions for invocation syntax, accepted arguments, and
individual options. Explain relationships between options in `Long` when those
relationships affect behavior.

Include the following in `Long` when relevant:

- What the command does, what it acts on, and its defaults.
- How it works under the hood when that explains observable behavior: execution
  order, requests, subprocesses, containers, or files it reads and changes.
- Required configuration, authentication, and services; how input, output, and
  argument forwarding work.
- Side effects, destructive actions, partial success, failure behavior, and
  recovery guarantees. Explain what happens to existing data and local changes.
- Important limitations and links to related help, the reference, or guides.

Use `Example` for copyable invocations covering the command's useful modes.
Configuration snippets and equivalent underlying commands also belong in help
when they explain how to use this command. A detailed single-command reference
can be long; use paragraphs and headings to make it easy to scan.

For example, `update-code` help should explain fast-forward, stash, and reset
behavior, which repositories it processes, recovery refs, and when Composer
runs. Custom-service help should explain file naming, Compose configuration,
routing, and dashboard discovery. These details remain part of the generated
command reference because that reference is built from command help.

## Guides

Organize a guide around an outcome, such as updating a working wiki, configuring
single sign-on, or adding and testing a custom service. Include prerequisites,
commands in order, example configuration or output, ways to check the result,
and cleanup or recovery steps when needed. Explain why the steps fit together.
A guide can also explore several modes of one command in a worked scenario.

For example, an update workflow can combine `update-code`, Git inspection,
Composer, and database maintenance. A custom-service guide can create a Compose
file, start the service, inspect it, and stop or remove it. The exact behavior
of each command belongs in its help; link there instead of copying its full
reference into the guide. Brief reminders beside relevant steps are useful.

Length alone does not decide where text belongs. A short workflow can be a
guide, and a long explanation of a command's behavior can be help. LINTI's help
and example size suggestions prompt an editorial review; they are not limits
or instructions to move command details out of help.

## Sources and links

Edit command help in its command package, including embedded `.long.md` and
`.example` files. Put user guides in `docs/guides/`, add their publication mapping
to `docs/wiki.json`, and link them from `docs/index.md`. Link help to relevant
guides with `mw help guides <topic>`; mapped guides are bundled offline and can
be opened on the wiki with `--web`. Contributor documentation such as this file
lives outside the published user guides.

Run `make linti` to review command metadata and readability suggestions, and
`make docs` to check guide links and generate the reference and wiki output.
See [Documentation publishing](README.md) for the publishing workflow.
