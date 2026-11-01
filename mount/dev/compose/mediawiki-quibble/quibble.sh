#!/bin/sh
# Use the image's virtualenv, not the system Python. Also covers interactive shells.
exec /opt/quibble-venv/bin/python3 /wm-cli-quibble.py "$@"
