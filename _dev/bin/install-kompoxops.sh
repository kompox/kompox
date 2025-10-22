#!/usr/bin/env bash
set -euo pipefail

# Install kompoxops into a user-writable bin directory that is already in PATH.
# Preference: $HOME/.local/bin (XDG) -> $HOME/bin
# If the binary is not built yet, build it first.

BIN="kompoxops"
SRC="${BIN}"

if [[ ! -f "${SRC}" ]]; then
  echo "Binary '${SRC}' not found. Building via 'go build ./cmd/kompoxops'..." >&2
  go build ./cmd/kompoxops
fi

HLOCAL="${HOME}/.local/bin"
HBIN="${HOME}/bin"
TARGET=""

# Normalize PATH lookup by surrounding with colons
PATH_COLON=":${PATH}:"

if grep -q ":${HLOCAL}:" <<<"${PATH_COLON}"; then
  TARGET="${HLOCAL}"
elif grep -q ":${HBIN}:" <<<"${PATH_COLON}"; then
  TARGET="${HBIN}"
else
  echo "Neither '${HLOCAL}' nor '${HBIN}' is in PATH. Skipping install." >&2
  exit 0
fi

echo "Installing ${BIN} to '${TARGET}'" >&2
mkdir -p "${TARGET}"
install -m 0755 "${SRC}" "${TARGET}/${BIN}"

if command -v "${BIN}" >/dev/null 2>&1; then
  echo "'${BIN}' is now available: $(command -v "${BIN}")" >&2
fi
