#!/bin/bash
set -euo pipefail
# The runner owns receipts and frozen inputs; package transactions are implemented
# in its bounded Python helper to avoid parsing package data in shell.
exec /usr/bin/python3 "$EASY_LIMA_HELPER" "$EASY_LIMA_STAGE" "$EASY_LIMA_INPUT"
