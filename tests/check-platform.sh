#!/usr/bin/env bash
# Shared checks called by the existing Linux, macOS and Windows jobs.
set -euo pipefail

bash tests/check-docker-context.sh
python3 tests/check-self-update.py
