#!/bin/bash
set -euo pipefail
exec /usr/bin/python3 "$EASY_LIMA_HELPER" "$EASY_LIMA_STAGE" "$EASY_LIMA_INPUT"
