#!/usr/bin/env bash
set -euo pipefail

# setup-test.sh
# Usage: _dev/bin/setup-test.sh <tests/subdir>
# Example: _dev/bin/setup-test.sh ./tests/aks-e2e-basic
#
# Copies the given test directory under _tmp/tests/<name>-N
# where <name> is the basename of the provided directory and N is an
# incrementing number starting from 1, choosing the first non-existing target.

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <tests/subdir>" >&2
  exit 2
fi

SRC_DIR="$1"
# Normalize path (but we assume repo root CWD as per project conventions)
SRC_DIR="${SRC_DIR%/}"

if [[ ! -d "$SRC_DIR" ]]; then
  echo "Source directory not found: $SRC_DIR" >&2
  exit 1
fi

NAME="$(basename "$SRC_DIR")"
DEST_BASE="_tmp/tests/${NAME}"

mkdir -p "_tmp/tests"

# Find next available destination: <base>, <base>-1, <base>-2, ...
DEST_DIR="$DEST_BASE"
if [[ -d "$DEST_DIR" ]]; then
  i=1
  while [[ -d "${DEST_BASE}-${i}" ]]; do
    i=$((i+1))
  done
  DEST_DIR="${DEST_BASE}-${i}"
fi

cp -r "$SRC_DIR" "$DEST_DIR"

echo "$DEST_DIR"
