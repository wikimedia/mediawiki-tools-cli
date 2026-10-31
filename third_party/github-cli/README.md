# GitHub CLI JSON formatter

This module contains only `pkg/jsoncolor`, copied from GitHub CLI v2.97.0:
https://github.com/cli/cli/tree/v2.97.0/pkg/jsoncolor

The source and upstream tests are retained with their MIT licence. Both mw and
the generated Toolforge CLI use this package. The root module replacement avoids
importing the full GitHub CLI dependency graph for JSON colouring, including
Docker dependencies incompatible with our current Docker SDK.

Only `pkg/jsoncolor` is provided. If another GitHub CLI package is needed, review
this replacement first. To update the formatter, copy its source and tests from
the reviewed upstream version, retain the licence, and run the module tests.
