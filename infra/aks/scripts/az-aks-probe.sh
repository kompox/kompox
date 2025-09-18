#!/bin/bash

# This script creates multiple AKS clusters in different regions to probe for availability and capacity.
# It waits for all clusters to reach a terminal state (Succeeded, Failed, or Canceled) and then lists any activity log errors that occurred during the process.
#
# Prerequisites:
# - Azure CLI installed and logged in
# - Sufficient permissions to create resource groups and AKS clusters
# - Quota for the specified VM sizes in the target regions
#
# Examples:
#   VM_SIZE=Standard_D2ds_v6 DISK_SIZE=110 az-aks-probe.sh
#   LOCATIONS="eastus eastus2" ZONE_PROBE_MODE=each VM_SIZE=Standard_D2alds_v6 az-aks-probe.sh
#   LOCATIONS="japaneast southeastasia" ZONES="1 2" VM_SIZE=Standard_B2s DISK_TYPE=Managed az-aks-probe.sh

: ${RESOURCE_GROUP=rg-aks-probe-$(date +%Y%m%d-%H%M%S)}
: ${LOCATIONS=eastus eastus2 westus2 westus3 northcentralus japaneast southeastasia}
: ${NAME=aks-probe}
: ${ZONES=1 2 3}
: ${ZONE_PROBE_MODE=all} # "all" or "each"
: ${VM_SIZE=Standard_D2ds_v4}
: ${DISK_TYPE=Ephemeral}
: ${DISK_SIZE=64}
: ${EXTRA_ARGS=--network-plugin azure --network-plugin-mode overlay --enable-aad --enable-azure-rbac --enable-managed-identity --enable-oidc-issuer --enable-workload-identity -a azure-keyvault-secrets-provider}
: ${POLL_INTERVAL=30}

BOLD=$(printf '\033[1m')
NORM=$(printf '\033[0m')

timestamp() {
    echo -n "$BOLD$(date +%Y-%m-%dT%H:%M:%S)$NORM "
}

log() {
    timestamp
    echo "$*"
}

run() {
    timestamp
    (set -x && "$@")
}

set -euo pipefail

case "$ZONE_PROBE_MODE" in
    all)  log "Probing all zones [$ZONES] in each location" ;;
    each) log "Probing each zone [$ZONES] separately in each location" ;;
    *)    log "ZONE_PROBE_MODE is $ZONE_PROBE_MODE, must be 'all' or 'each'"; exit 1 ;;
esac

log "Creating resource group $RESOURCE_GROUP"
run az group create -n "$RESOURCE_GROUP" -l eastus -o none

trap "log $'Terminated. Command to clean up:\n  az group delete -n $RESOURCE_GROUP --yes --no-wait'" EXIT

for LOCATION in $LOCATIONS; do
    case "$ZONE_PROBE_MODE" in
        all)  set -- "$ZONES" ;;
        each) set -- $ZONES ;;
    esac
    for Z; do    
        N="$NAME-$LOCATION-$(echo $Z | tr -d ' ')"
        log "Creating AKS cluster $N"
        status=0
        run az aks create \
            --resource-group "$RESOURCE_GROUP" \
            --location "$LOCATION" \
            --name "$N" \
            --zones $Z \
            --node-vm-size "$VM_SIZE" \
            --node-osdisk-type "$DISK_TYPE" \
            --node-osdisk-size "$DISK_SIZE" \
            --node-count 1 \
            --generate-ssh-keys \
            $EXTRA_ARGS \
            --tags "ZONES=$ZONES" "VM_SIZE=$VM_SIZE" "DISK_TYPE=$DISK_TYPE" "DISK_SIZE=$DISK_SIZE" "EXTRA_ARGS=$EXTRA_ARGS" \
            --no-wait || status=$?
        log "Created AKS cluster $N: status=$status"
    done
done

log "Waiting for all AKS clusters in resource group $RESOURCE_GROUP to reach terminal state..."

while true; do
    unknown=0
    running=0
    terminated=0
    states=$(az aks list --resource-group "$RESOURCE_GROUP" --query '[].provisioningState' -o tsv)
    for state in $states; do
        case "$state" in
            Creating|Updating|Deleting) running=$((running+1)) ;;
            Canceled|Failed|Succeeded) terminated=$((terminated+1)) ;;
            *) unknown=$((unknown+1)) ;;
        esac
    done
    log "Current states: running=$running unknown=$unknown terminated=$terminated"
    if test $running -eq 0; then
        break
    fi
    sleep $POLL_INTERVAL
done

run az aks list --resource-group "$RESOURCE_GROUP" --query "[].{Name:name, Location:location, Zones:join(',',agentPoolProfiles[0].availabilityZones), State:provisioningState}" -o table

log "Activity log errors occurred during this run (logs may be delayed by a few minutes):"

run az monitor activity-log list \
    --resource-group "$RESOURCE_GROUP" \
    --status Failed \
    -o yamlc \
    --query '[].{
        time: eventTimestamp,
        operation: operationName.value,
        resource: resourceId,
        status: status.localizedValue,
        subStatus: subStatus.localizedValue,
        message: properties.statusMessage
    }'
