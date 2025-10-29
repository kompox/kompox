#!/bin/bash

set -euo pipefail

VOLUME="vol3"
SUFFIX=$(date +%H%M%S)
DISK1="fdb1-${SUFFIX}"
DISK2="fdb2-${SUFFIX}"

CREATED_DISKS=()
JQ_AVAILABLE=0
RESTORE_DISK=""

log() {
	printf '[%s] %s\n' "$(date --iso-8601=seconds)" "$*" >&2
}

run() {
	local q
	printf -v q '%q ' "$@"
	log "+ ${q% }"
	"$@"
}

WORKDIR=$(mktemp -d)
trap 'cleanup' EXIT

cleanup() {
	set +e
	if (( JQ_AVAILABLE == 1 )) && [[ -n "${RESTORE_DISK:-}" ]] && disk_exists "$RESTORE_DISK"; then
		log "Restoring Assigned share to $RESTORE_DISK"
		run ./kompoxops disk assign -V "$VOLUME" -N "$RESTORE_DISK" >/dev/null 2>&1 || true
	fi
	for DISK in "${CREATED_DISKS[@]}"; do
		if [[ "$DISK" == "$RESTORE_DISK" ]]; then
			continue
		fi
		log "Cleaning Azure Files share $DISK"
		run ./kompoxops disk delete -V "$VOLUME" -N "$DISK" >/dev/null 2>&1 || true
	done
	rm -rf "$WORKDIR"
}

disk_exists() {
	local name=$1
	run ./kompoxops disk list -V "$VOLUME" | jq -e --arg name "$name" 'map(.name) | index($name) != null' >/dev/null
}

assert_json_array() {
	local json=$1 desc=$2
	echo "$json" | jq -e 'type == "array"' >/dev/null
	log "$desc returned JSON array (${#json} bytes)"
}

assert_field_equals() {
	local json=$1 jq_filter=$2 expected=$3 desc=$4
	local actual
	actual=$(echo "$json" | jq -r "$jq_filter")
	if [[ "$actual" != "$expected" ]]; then
		log "Assertion failed for $desc: expected '$expected' but got '$actual'"
		exit 1
	fi
	log "$desc = $actual"
}

expect_failure() {
	if "$@"; then
		log "Expected failure but command succeeded: $*"
		exit 1
	fi
	log "Command failed as expected: $*"
}

cat kompoxapp.yml

run ./kompoxops version

if command -v jq >/dev/null 2>&1; then
	JQ_AVAILABLE=1
else
	log "jq command is required"
	exit 1
fi

log "========== Azure Files (vol3) E2E Tests =========="

log "Listing initial Azure Files shares for $VOLUME"
INITIAL_JSON=$(run ./kompoxops disk list -V "$VOLUME")
assert_json_array "$INITIAL_JSON" "Azure Files disk list"
ORIGINAL_ASSIGNED=$(echo "$INITIAL_JSON" | jq -r 'map(select(.assigned == true)) | first | .name // ""')

log "===== Azure Files Operations Test ====="
log "Testing files backend operations for volume $VOLUME"
log ""

if [[ -n "$ORIGINAL_ASSIGNED" ]]; then
	RESTORE_DISK=$ORIGINAL_ASSIGNED
	log "Original Assigned share: $ORIGINAL_ASSIGNED"
else
	RESTORE_DISK=""
	log "No pre-existing Assigned share"
fi

log ""
log "===== Creating and assigning share ====="
log "Creating Azure Files share $DISK1"
DISK1_JSON=$(run ./kompoxops disk create -V "$VOLUME" -N "$DISK1")
CREATED_DISKS+=("$DISK1")
assert_field_equals "$DISK1_JSON" '.name' "$DISK1" "Azure Files disk create name"
HANDLE=$(echo "$DISK1_JSON" | jq -r '.handle // ""')
# Azure Files CSI driver uses format: {rg}#{account}#{share}#####{subscription}
if [[ ! "$HANDLE" =~ ^[^#]+#[^#]+#[^#]+####[a-f0-9-]+$ ]]; then
	log "Expected Azure Files handle to be CSI volumeHandle format, got: $HANDLE"
	exit 1
fi
log "Azure Files handle: $HANDLE"

log "Assigning Azure Files share $DISK1"
run ./kompoxops disk assign -V "$VOLUME" -N "$DISK1"

ASSIGN_JSON=$(run ./kompoxops disk list -V "$VOLUME")
assert_json_array "$ASSIGN_JSON" "Azure Files disk list after assign"
ASSIGNED=$(echo "$ASSIGN_JSON" | jq -r 'map(select(.assigned == true)) | first | .name // ""')
if [[ "$ASSIGNED" != "$DISK1" ]]; then
	log "Expected Assigned share to be $DISK1 but got $ASSIGNED"
	exit 1
fi
ASSIGNED_COUNT=$(echo "$ASSIGN_JSON" | jq '[.[] | select(.assigned == true)] | length')
if (( ASSIGNED_COUNT != 1 )); then
	log "Expected exactly one Assigned share, got $ASSIGNED_COUNT"
		exit 1
fi

log ""
log "===== Testing reassignment ====="
log "Creating second share $DISK2"
DISK2_JSON=$(run ./kompoxops disk create -V "$VOLUME" -N "$DISK2")
CREATED_DISKS+=("$DISK2")
assert_field_equals "$DISK2_JSON" '.name' "$DISK2" "Azure Files disk create name"

log "Reassigning to $DISK2"
run ./kompoxops disk assign -V "$VOLUME" -N "$DISK2"

REASSIGN_JSON=$(run ./kompoxops disk list -V "$VOLUME")
assert_json_array "$REASSIGN_JSON" "Azure Files disk list after reassign"
ASSIGNED_AFTER=$(echo "$REASSIGN_JSON" | jq -r 'map(select(.assigned == true)) | first | .name // ""')
if [[ "$ASSIGNED_AFTER" != "$DISK2" ]]; then
	log "Expected Assigned share to be $DISK2 but got $ASSIGNED_AFTER"
	exit 1
fi

log ""
log "===== Testing unsupported operations ====="
log "Verifying snapshot operations are not supported for Azure Files"
expect_failure run ./kompoxops snapshot list -V "$VOLUME"
expect_failure run ./kompoxops snapshot create -V "$VOLUME" -N "invalid-snap"
expect_failure run ./kompoxops snapshot delete -V "$VOLUME" --snap-name "invalid-snap"

log "Verifying source parameter is rejected for Azure Files"
expect_failure run ./kompoxops disk create -V "$VOLUME" -N "invalid-disk" -S "disk:$DISK1"
expect_failure run ./kompoxops disk create -V "$VOLUME" -N "invalid-disk" -S "snapshot:invalid-snap"

log "Verifying deletion of assigned share is rejected"
expect_failure run ./kompoxops disk delete -V "$VOLUME" -N "$DISK2"
expect_failure run ./kompoxops disk delete -V "$VOLUME" --disk-name "$DISK2"

log ""
log "===== Testing deletion ====="
log "Deleting unassigned Azure Files share $DISK1"
run ./kompoxops disk delete -V "$VOLUME" -N "$DISK1"
mapfile -t CREATED_DISKS < <(printf '%s\n' "${CREATED_DISKS[@]}" | grep -v "^$DISK1$" || true)

log "Verifying deleted share is no longer listed"
AFTER_DELETE_JSON=$(run ./kompoxops disk list -V "$VOLUME")
if echo "$AFTER_DELETE_JSON" | jq -e --arg name "$DISK1" 'map(.name) | index($name) != null' >/dev/null; then
	log "Deleted share $DISK1 still appears in list"
	exit 1
fi

log ""
log "===== Testing error conditions ====="
log "Verifying invalid share name is rejected"
expect_failure run ./kompoxops disk create -V "$VOLUME" -N "INVALID_NAME"

log "Verifying disk assign with non-existent share name fails"
expect_failure run ./kompoxops disk assign -V "$VOLUME" -N "nonexistent-share-name"

log ""
log "===== Cleaning up ====="
if [[ -n "$ORIGINAL_ASSIGNED" ]]; then
	log "Reassigning to original share $ORIGINAL_ASSIGNED"
	run ./kompoxops disk assign -V "$VOLUME" -N "$ORIGINAL_ASSIGNED"
	if [[ "$DISK2" != "$ORIGINAL_ASSIGNED" ]]; then
		log "Deleting test share $DISK2"
		run ./kompoxops disk delete -V "$VOLUME" -N "$DISK2"
		mapfile -t CREATED_DISKS < <(printf '%s\n' "${CREATED_DISKS[@]}" | grep -v "^$DISK2$" || true)
	fi
else
	log "Keeping $DISK2 assigned (no original share to restore)"
	RESTORE_DISK=$DISK2
fi

log ""
log "===== All files operations tests passed! ====="
