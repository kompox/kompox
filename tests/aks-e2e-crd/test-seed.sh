#!/bin/bash

set -euo pipefail

DIR=$(cd $(dirname $0); pwd)

cat <<EOF
KOMPOX_CRD_PATH=$DIR/crd
KUBECONFIG=$DIR/kubeconfig
SSH_DIR=$DIR/ssh
SSH_PRIVATE_KEY=$DIR/ssh/id_rsa
SSH_PUBLIC_KEY=$DIR/ssh/id_rsa.pub
EOF
