#!/usr/bin/env python3
# PLACEHOLDER: Replace this file with the actual phab.py obtained from:
#   https://gitlab.wikimedia.org/jiji/phab
#
# The mwcli build embeds this file at compile time (via //go:embed).
# To update it, replace this file and rebuild the binary.
import sys

print(
    "This is a placeholder for the phab CLI tool.\n"
    "The actual implementation must be obtained from:\n"
    "  https://gitlab.wikimedia.org/jiji/phab\n"
    "\n"
    "Replace commands/phabricator/phab.py with phab.py from that repository\n"
    "and rebuild mwcli to embed the real script.",
    file=sys.stderr,
)
sys.exit(1)
