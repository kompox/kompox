#!/bin/bash

set -euo pipefail

rm -rf -- "${KUBECONFIG-}" "${SSH_DIR-}"
