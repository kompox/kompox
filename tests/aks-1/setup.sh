#!/bin/bash

set -euo pipefail
set -a # auto export
: ${TOP_DIR=$(cd $(dirname $0)/../..; pwd)}
: ${SRC_DIR=$(cd $(dirname $0); pwd)}
: ${TEST_NAME=$(basename $SRC_DIR)}
: ${SERVICE_NAME=$(date +"ops-%Y%m%d-%H%M%S")}
: ${RUN_DIR=$TOP_DIR/tmp/tests/$TEST_NAME/$SERVICE_NAME}
: ${AZURE_SUBSCRIPTION_ID=34809bd3-31b4-4331-9376-49a32a9616f2}
: ${AZURE_LOCATION=eastus}
: ${AZURE_AKS_SYSTEM_VM_SIZE=Standard_B2s}
: ${AZURE_AKS_SYSTEM_VM_DISK_TYPE=Managed}
: ${AZURE_AKS_SYSTEM_VM_DISK_SIZE_GB=32}
: ${AZURE_AKS_SYSTEM_VM_PRIORITY=Regular}
: ${AZURE_AKS_SYSTEM_VM_ZONES="1,2"}
: ${AZURE_AKS_USER_VM_SIZE=Standard_B2s}
: ${AZURE_AKS_USER_VM_DISK_TYPE=Managed}
: ${AZURE_AKS_USER_VM_DISK_SIZE_GB=32}
: ${AZURE_AKS_USER_VM_PRIORITY=Regular}
: ${AZURE_AKS_USER_VM_ZONES="1,2"}
: ${KOMPOXOPS=$TOP_DIR/kompoxops}
set +a

mkdir -p "$RUN_DIR"

ln "$KOMPOXOPS" "$RUN_DIR/kompoxops"
install "$SRC_DIR/run.sh" "$RUN_DIR/run.sh"

ENV_SH=$(cat "$SRC_DIR/env.sh")
for i in env.sh kompoxops.yml; do
   envsubst "$ENV_SH" < "$SRC_DIR/$i" > "$RUN_DIR/$i"
done

echo "$RUN_DIR/run.sh 2>&1 | tee $RUN_DIR/run.log"
