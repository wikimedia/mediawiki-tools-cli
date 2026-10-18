#!/usr/bin/env bash
# Shared checks called by the existing Linux, macOS and Windows jobs.
set -euo pipefail

go test ./pkg/docker -count=1
python3 tests/check-self-update.py
