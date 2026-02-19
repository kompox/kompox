#!/bin/bash

set -xeuo pipefail

NODEPOOL_NAME=""
NODEPOOL_CREATED=0
NODEPOOL_TMPDIR=""

cleanup_nodepool() {
	set +e
	if [[ "$NODEPOOL_CREATED" -eq 1 && -n "$NODEPOOL_NAME" ]]; then
		./kompoxops cluster nodepool delete --name "$NODEPOOL_NAME" >/dev/null 2>&1 || true
	fi
	if [[ -n "$NODEPOOL_TMPDIR" ]]; then
		rm -rf "$NODEPOOL_TMPDIR"
	fi
}

trap cleanup_nodepool EXIT

cat kompoxapp.yml

./kompoxops version

./kompoxops cluster status

./kompoxops cluster provision

./kompoxops cluster status

./kompoxops cluster install

./kompoxops cluster status

NODEPOOL_SUFFIX=$(date +%H%M%S)
NODEPOOL_NAME="np${NODEPOOL_SUFFIX}"
NODEPOOL_TMPDIR=$(mktemp -d)
NODEPOOL_CREATE_SPEC="$NODEPOOL_TMPDIR/nodepool-create.yml"
NODEPOOL_UPDATE_SPEC="$NODEPOOL_TMPDIR/nodepool-update.json"

cat > "$NODEPOOL_CREATE_SPEC" <<EOF
name: ${NODEPOOL_NAME}
mode: user
instanceType: ${NODEPOOL_INSTANCE_TYPE:-$AZURE_AKS_USER_VM_SIZE}
osDiskType: ${NODEPOOL_OS_DISK_TYPE:-$AZURE_AKS_USER_VM_DISK_TYPE}
osDiskSizeGiB: ${NODEPOOL_OS_DISK_SIZE_GIB:-$AZURE_AKS_USER_VM_DISK_SIZE_GB}
priority: regular
zones:
  - "${NODEPOOL_ZONE:-1}"
labels:
  e2e.kompox.dev/suite: aks-e2e-basic
autoscaling:
  enabled: true
  min: ${NODEPOOL_CREATE_MIN:-1}
  max: ${NODEPOOL_CREATE_MAX:-1}
EOF

cat > "$NODEPOOL_UPDATE_SPEC" <<EOF
{
  "name": "${NODEPOOL_NAME}",
  "labels": {
    "e2e.kompox.dev/suite": "aks-e2e-basic",
    "e2e.kompox.dev/phase": "updated"
  },
  "autoscaling": {
    "enabled": true,
    "min": ${NODEPOOL_CREATE_MIN:-1},
    "max": ${NODEPOOL_UPDATE_MAX:-2}
  }
}
EOF

CREATE_OUT=$(./kompoxops cluster nodepool create --file "$NODEPOOL_CREATE_SPEC")
echo "$CREATE_OUT" | jq -e --arg name "$NODEPOOL_NAME" '(.Name // .name) == $name' >/dev/null
NODEPOOL_CREATED=1

./kompoxops cluster nodepool list | jq -e --arg name "$NODEPOOL_NAME" 'map(select((.Name // .name // "") == $name)) | length > 0' >/dev/null

UPDATE_OUT=$(./kompoxops cluster nodepool update --file "$NODEPOOL_UPDATE_SPEC")
echo "$UPDATE_OUT" | jq -e --arg name "$NODEPOOL_NAME" '(.Name // .name) == $name' >/dev/null

./kompoxops cluster nodepool list | jq -e --arg name "$NODEPOOL_NAME" 'map(select((.Name // .name // "") == $name)) | length > 0' >/dev/null

./kompoxops cluster nodepool delete --name "$NODEPOOL_NAME"
NODEPOOL_CREATED=0

./kompoxops cluster nodepool list | jq -e --arg name "$NODEPOOL_NAME" 'map(select((.Name // .name // "") == $name)) | length == 0' >/dev/null

./kompoxops cluster kubeconfig --merge --set-current

kubectl get ns

./kompoxops app deploy --update-dns

./kompoxops app status

./kompoxops app logs -c app1

./kompoxops app logs -c app2

./kompoxops app logs -c app3

./kompoxops app logs -c app4

./kompoxops box deploy --ssh-pubkey=$SSH_PUBLIC_KEY

./kompoxops box status

IP=$(./kompoxops cluster status | jq -r .ingressGlobalIP)

HOSTS=$(./kompoxops app status | jq -r .ingress_hosts[])

for HOST in $HOSTS; do
	curl -k --resolve "$HOST:443:$IP" https://$HOST?env=true
done

./kompoxops secret env set -S app -f app-env-override.yml

for HOST in $HOSTS; do
	curl -k --resolve "$HOST:443:$IP" https://$HOST?env=true
done

./kompoxops cluster logs
