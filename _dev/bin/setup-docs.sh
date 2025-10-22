#!/usr/bin/env bash
set -euo pipefail

# Setup docs toolchain at repository root (.venv) and install requirements
# Assumptions:
# - Run this script from the repository root (via `make docs-setup`)
# - Docs requirements live at docs/_mkdocs/requirements.txt

REPO_ROOT="${REPO_ROOT:-$(pwd -P)}"
REQ_FILE="${REPO_ROOT}/docs/_mkdocs/requirements.txt"
VENV_DIR="${REPO_ROOT}/.venv"

if [[ ! -f "${REQ_FILE}" ]]; then
  echo "Requirements file not found: ${REQ_FILE}" >&2
  exit 1
fi

if command -v uv >/dev/null 2>&1; then
  echo "Using uv to create venv and install docs deps" >&2
  if [[ ! -d "${VENV_DIR}" ]]; then
    uv venv "${VENV_DIR}"
  fi
  # shellcheck disable=SC1091
  source "${VENV_DIR}/bin/activate"
  uv pip install -r "${REQ_FILE}"
else
  echo "uv not found, falling back to python3 venv + pip" >&2
  if [[ ! -d "${VENV_DIR}" ]]; then
    if ! python3 -m venv "${VENV_DIR}"; then
      echo "python3-venv is required (e.g., apt install python3.12-venv)" >&2
      exit 1
    fi
  fi
  # shellcheck disable=SC1091
  source "${VENV_DIR}/bin/activate"
  pip install -r "${REQ_FILE}"
fi

echo "Docs toolchain ready at ${VENV_DIR}." >&2
