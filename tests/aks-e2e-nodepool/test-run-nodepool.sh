#!/bin/bash

set -euo pipefail

SUFFIX=$(date +%H%M%S)
TEST_RUN_ID="np-${SUFFIX}"
NODEPOOL_NAME="np${SUFFIX}"
CREATE_SPEC=""
UPDATE_SPEC=""
CREATED=0

log() {
	printf '[%s] %s\n' "$(date --iso-8601=seconds)" "$*" >&2
}

run() {
	local q
	printf -v q '%q ' "$@"
	log "+ ${q% }"
	"$@"
}

capture_json() {
	local __resultvar=$1
	shift
	local output
	output=$(run "$@" | tee /dev/stderr)
	printf -v "$__resultvar" '%s' "$output"
}

assert_json_array() {
	local json=$1 desc=$2
	echo "$json" | jq -e 'type == "array"' >/dev/null
	log "$desc returned JSON array (${#json} bytes)"
}

assert_name_equals() {
	local json=$1 expected=$2 desc=$3
	local actual
	actual=$(echo "$json" | jq -r '.Name // .name // ""')
	if [[ "$actual" != "$expected" ]]; then
		log "Assertion failed for $desc: expected '$expected' but got '$actual'"
		echo "$json" | jq '.' >&2 || true
		exit 1
	fi
	log "$desc name = $actual"
}

assert_pool_list_contains() {
	local json=$1 expected=$2 desc=$3
	if ! echo "$json" | jq -e --arg name "$expected" 'map(select((.Name // .name // "") == $name)) | length > 0' >/dev/null; then
		log "Assertion failed for $desc: pool '$expected' not found"
		echo "$json" | jq '.' >&2 || true
		exit 1
	fi
	log "$desc contains pool '$expected'"
}

assert_pool_list_not_contains() {
	local json=$1 expected=$2 desc=$3
	if echo "$json" | jq -e --arg name "$expected" 'map(select((.Name // .name // "") == $name)) | length > 0' >/dev/null; then
		log "Assertion failed for $desc: pool '$expected' still exists"
		echo "$json" | jq '.' >&2 || true
		exit 1
	fi
	log "$desc does not contain pool '$expected'"
}

WORKDIR=$(mktemp -d)
trap 'cleanup' EXIT

cleanup() {
	set +e
	if (( CREATED == 1 )); then
		log "Cleanup: deleting nodepool $NODEPOOL_NAME"
		run ./kompoxops cluster nodepool delete --name "$NODEPOOL_NAME" >/dev/null 2>&1 || true
	fi
	rm -rf "$WORKDIR"
}

if ! command -v jq >/dev/null 2>&1; then
	log "jq command is required"
	exit 1
fi

cat kompoxapp.yml

run ./kompoxops version

log "===== Provisioning cluster for nodepool E2E ====="
run ./kompoxops cluster provision
run ./kompoxops cluster status

CREATE_SPEC="$WORKDIR/nodepool-create.yml"
cat > "$CREATE_SPEC" <<EOF
name: ${NODEPOOL_NAME}
mode: user
instanceType: ${NODEPOOL_INSTANCE_TYPE:-Standard_D2ds_v4}
osDiskType: ${NODEPOOL_OS_DISK_TYPE:-Ephemeral}
osDiskSizeGiB: ${NODEPOOL_OS_DISK_SIZE_GIB:-64}
priority: regular
zones:
  - "${NODEPOOL_ZONE:-1}"
labels:
  e2e.kompox.dev/scenario: nodepool-cli
  e2e.kompox.dev/run: ${TEST_RUN_ID}
autoscaling:
  enabled: true
  min: ${NODEPOOL_CREATE_MIN:-1}
  max: ${NODEPOOL_CREATE_MAX:-1}
EOF

UPDATE_SPEC="$WORKDIR/nodepool-update.json"
cat > "$UPDATE_SPEC" <<EOF
{
  "name": "${NODEPOOL_NAME}",
  "labels": {
    "e2e.kompox.dev/scenario": "nodepool-cli",
    "e2e.kompox.dev/run": "${TEST_RUN_ID}",
    "e2e.kompox.dev/phase": "updated"
  },
  "autoscaling": {
    "enabled": true,
    "min": ${NODEPOOL_CREATE_MIN:-1},
    "max": ${NODEPOOL_UPDATE_MAX:-2}
  }
}
EOF

log "===== Initial list ====="
capture_json INITIAL_LIST ./kompoxops cluster nodepool list
assert_json_array "$INITIAL_LIST" "nodepool list"

log "===== Create nodepool (YAML) ====="
CREATE_OUT=$(run ./kompoxops cluster nodepool create --file "$CREATE_SPEC")
CREATED=1
assert_name_equals "$CREATE_OUT" "$NODEPOOL_NAME" "nodepool create"

capture_json AFTER_CREATE_LIST ./kompoxops cluster nodepool list
assert_json_array "$AFTER_CREATE_LIST" "nodepool list after create"
assert_pool_list_contains "$AFTER_CREATE_LIST" "$NODEPOOL_NAME" "nodepool list after create"

log "===== Update nodepool (JSON) ====="
UPDATE_OUT=$(run ./kompoxops cluster nodepool update --file "$UPDATE_SPEC")
assert_name_equals "$UPDATE_OUT" "$NODEPOOL_NAME" "nodepool update"

capture_json AFTER_UPDATE_LIST ./kompoxops cluster nodepool list
assert_json_array "$AFTER_UPDATE_LIST" "nodepool list after update"
assert_pool_list_contains "$AFTER_UPDATE_LIST" "$NODEPOOL_NAME" "nodepool list after update"

log "===== Delete nodepool ====="
run ./kompoxops cluster nodepool delete --name "$NODEPOOL_NAME"
CREATED=0

capture_json AFTER_DELETE_LIST ./kompoxops cluster nodepool list
assert_json_array "$AFTER_DELETE_LIST" "nodepool list after delete"
assert_pool_list_not_contains "$AFTER_DELETE_LIST" "$NODEPOOL_NAME" "nodepool list after delete"

log "===== NodePool CLI E2E passed ====="
