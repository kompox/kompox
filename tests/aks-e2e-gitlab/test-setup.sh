#!/bin/bash

set -euo pipefail

touch "$KUBECONFIG"
chmod 600 "$KUBECONFIG"

mkdir -m 700 -p "$SSH_DIR"
if ! test -r "$SSH_PRIVATE_KEY"; then
    ssh-keygen -N '' -t rsa -f "$SSH_PRIVATE_KEY"
fi
