#!/usr/bin/env bash

set -euo pipefail

# Prerequisite: already logged in to Azure and the target subscription is selected
# az account set --subscription "<SUBSCRIPTION_ID>"

providers=(
    Microsoft.Compute
    Microsoft.Network
    Microsoft.ContainerService
    Microsoft.ManagedIdentity
    Microsoft.ContainerRegistry
    Microsoft.KeyVault
    Microsoft.OperationalInsights
    Microsoft.OperationsManagement
    Microsoft.Insights
    Microsoft.Storage
    Microsoft.Portal
)

echo "Registering required resource providers..."

for p in "${providers[@]}"; do
    echo "$p"
    az provider register --namespace "$p" || true
done

echo "Waiting for all providers to be registered..."

while :; do
    pending=0
    for p in "${providers[@]}"; do
        state="$(az provider show --namespace "$p" --query registrationState -o tsv 2>/dev/null || echo NotRegistered)"
        [[ "$state" == "Registered" ]] || pending=$((pending+1))
    done
    (( pending == 0 )) && break
    sleep 5
done

echo "All required providers are registered."