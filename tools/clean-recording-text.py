#!/usr/bin/env python3
"""Remove recording harness setup from VHS text exports, preserving CLI output."""

import argparse
import re
from pathlib import Path


def clean_text(text):
    lines = []
    hidden_setup = False
    for line in text.splitlines(keepends=True):
        # VHS text exports can include setup frames even inside Hide/Show.
        # Keep their output format, but discard the shell harness and its wraps.
        if (line.startswith('> mw() {') and 'MWCLI_RECORD_' in line) or line.startswith(
            ('> export CONTEXT="$MWCLI_RECORD_CONTEXT"',
             '> cd "$(mw dev mediawiki where)"',
             '> cd "$(mw dev --context "$MWCLI_RECORD_CONTEXT" mediawiki where)"')
        ):
            hidden_setup = True
            continue
        if hidden_setup:
            if line.startswith('> ') or re.fullmatch(r'[─\s]+', line):
                hidden_setup = False
            else:
                continue
        line = line.replace('mw dev --context "$MWCLI_RECORD_CONTEXT" ', 'mw dev ')
        recording_password = '-e MYSQL_PWD="$MWCLI_RECORD_DB_PASSWORD" ' in line
        line = line.replace('-e MYSQL_PWD="$MWCLI_RECORD_DB_PASSWORD" ', '')
        # Older dump recordings use a recording-only variable for the standard
        # local development password; make the displayed commands self-contained.
        if recording_password and line.startswith('> mw dev ') and 'mariadb' in line and '-uroot' in line:
            line = re.sub(r'-uroot(?! -ptoor)', '-uroot -ptoor', line)
        lines.append(line)
    return ''.join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('files', nargs='+', type=Path)
    args = parser.parse_args()
    for path in args.files:
        original = path.read_text()
        cleaned = clean_text(original)
        if cleaned != original:
            path.write_text(cleaned)


if __name__ == '__main__':
    main()
