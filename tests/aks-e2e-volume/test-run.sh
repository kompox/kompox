#!/bin/bash

set -euo pipefail

CREATED_DISKS=()
CREATED_SNAPSHOTS=()
RESTORE_DISK=""
JQ_AVAILABLE=0
RETRY_ATTEMPTS=${RETRY_ATTEMPTS:-10}
RETRY_DELAY_SECONDS=${RETRY_DELAY_SECONDS:-30}

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
		log "Restoring Assigned disk to $RESTORE_DISK"
		run ./kompoxops disk assign -V "$VOLUME" -N "$RESTORE_DISK" >/dev/null 2>&1 || true
	fi
	for SNAP in "${CREATED_SNAPSHOTS[@]}"; do
		log "Cleaning snapshot $SNAP"
		run ./kompoxops snapshot delete -V "$VOLUME" --snap-name "$SNAP" >/dev/null 2>&1 || true
	done
	for DISK in "${CREATED_DISKS[@]}"; do
		if [[ "$DISK" == "$RESTORE_DISK" ]]; then
			continue
		fi
		log "Cleaning disk $DISK"
		run ./kompoxops disk delete -V "$VOLUME" --disk-name "$DISK" >/dev/null 2>&1 || true
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

retry_capture() {
	local __resultvar=$1
	shift
	local attempts=$RETRY_ATTEMPTS
	local delay=$RETRY_DELAY_SECONDS
	local output status
	for ((i=1; i<=attempts; i++)); do
		if output=$("$@"); then
			printf -v "${__resultvar}" '%s' "$output"
			return 0
		fi
		status=$?
		if (( i == attempts )); then
			return $status
		fi
		log "Retrying in ${delay}s (attempt $i/$attempts failed)"
		sleep "$delay"
	done
}

cat kompoxapp.yml

run ./kompoxops version

if command -v jq >/dev/null 2>&1; then
	JQ_AVAILABLE=1
else
	log "jq command is required"
	exit 1
fi

VOLUME=${VOLUME:-vol1}
SUFFIX=$(date +%H%M%S)

DISK_BASE="db-${SUFFIX}"
DISK_FROM_SNAPSHOT="dbsnap-${SUFFIX}"
DISK_TEMP="dbtmp-${SUFFIX}"
SNAP_AUTO="snauto-${SUFFIX}"
SNAP_EXPLICIT="snexp-${SUFFIX}"

INITIAL_DISKS_JSON=$(run ./kompoxops disk list -V "$VOLUME")
assert_json_array "$INITIAL_DISKS_JSON" "disk list"
ORIGINAL_ASSIGNED=$(echo "$INITIAL_DISKS_JSON" | jq -r 'map(select(.assigned == true)) | first | .name // ""')

if [[ -n "$ORIGINAL_ASSIGNED" ]]; then
	RESTORE_DISK=$ORIGINAL_ASSIGNED
	log "Original Assigned disk: $ORIGINAL_ASSIGNED"
else
	RESTORE_DISK=""
	log "No pre-existing Assigned disk"
fi

log "Creating base disk $DISK_BASE"
BASE_DISK_JSON=$(run ./kompoxops disk create -V "$VOLUME" -N "$DISK_BASE")
CREATED_DISKS+=("$DISK_BASE")
assert_field_equals "$BASE_DISK_JSON" '.name' "$DISK_BASE" "disk create name"

log "Assigning $DISK_BASE"
run ./kompoxops disk assign -V "$VOLUME" -N "$DISK_BASE"

POST_ASSIGN_JSON=$(run ./kompoxops disk list -V "$VOLUME")
assert_json_array "$POST_ASSIGN_JSON" "disk list after assign"
ASSIGNED_COUNT=$(echo "$POST_ASSIGN_JSON" | jq '[.[] | select(.assigned == true)] | length')
if (( ASSIGNED_COUNT != 1 )); then
	log "Expected exactly one Assigned disk, got $ASSIGNED_COUNT"
	exit 1
fi

log "Creating snapshot without explicit source -> $SNAP_AUTO"
SNAP_AUTO_JSON=$(run ./kompoxops snapshot create -V "$VOLUME" -N "$SNAP_AUTO")
CREATED_SNAPSHOTS+=("$SNAP_AUTO")
assert_field_equals "$SNAP_AUTO_JSON" '.name' "$SNAP_AUTO" "snapshot create (implicit source) name"

log "Creating snapshot with explicit disk source -> $SNAP_EXPLICIT"
SNAP_EXPLICIT_JSON=$(run ./kompoxops snapshot create -V "$VOLUME" -N "$SNAP_EXPLICIT" -S "disk:$DISK_BASE")
CREATED_SNAPSHOTS+=("$SNAP_EXPLICIT")
assert_field_equals "$SNAP_EXPLICIT_JSON" '.name' "$SNAP_EXPLICIT" "snapshot create (explicit source) name"

SNAP_LIST_JSON=$(run ./kompoxops snapshot list -V "$VOLUME")
assert_json_array "$SNAP_LIST_JSON" "snapshot list"
for EXPECT in "$SNAP_AUTO" "$SNAP_EXPLICIT"; do
	if ! echo "$SNAP_LIST_JSON" | jq -e --arg name "$EXPECT" 'map(.name) | index($name) != null' >/dev/null; then
		log "Snapshot $EXPECT not listed"
		exit 1
	fi
done

log "Creating disk from snapshot $SNAP_AUTO -> $DISK_FROM_SNAPSHOT"
if ! retry_capture DISK_FROM_SNAP_JSON run ./kompoxops disk create -V "$VOLUME" -N "$DISK_FROM_SNAPSHOT" -S "snapshot:$SNAP_AUTO"; then
	log "Failed to create disk from snapshot $SNAP_AUTO after ${RETRY_ATTEMPTS} attempts"
	exit 1
fi
CREATED_DISKS+=("$DISK_FROM_SNAPSHOT")
assert_field_equals "$DISK_FROM_SNAP_JSON" '.name' "$DISK_FROM_SNAPSHOT" "disk create from snapshot name"

log "Assigning $DISK_FROM_SNAPSHOT to verify reassignment semantics"
run ./kompoxops disk assign -V "$VOLUME" -N "$DISK_FROM_SNAPSHOT"

REASSIGN_JSON=$(run ./kompoxops disk list -V "$VOLUME")
assert_json_array "$REASSIGN_JSON" "disk list after reassign"
CURRENT_ASSIGNED=$(echo "$REASSIGN_JSON" | jq -r 'map(select(.assigned == true)) | first | .name // ""')
if [[ "$CURRENT_ASSIGNED" != "$DISK_FROM_SNAPSHOT" ]]; then
	log "Expected Assigned disk to be $DISK_FROM_SNAPSHOT but got $CURRENT_ASSIGNED"
	exit 1
fi

log "Validating --disk-name alias rejects assigned deletions"
expect_failure run ./kompoxops disk delete -V "$VOLUME" --disk-name "$DISK_FROM_SNAPSHOT"

if [[ -n "$ORIGINAL_ASSIGNED" ]]; then
	log "Reassigning to original disk $ORIGINAL_ASSIGNED"
	run ./kompoxops disk assign -V "$VOLUME" -N "$ORIGINAL_ASSIGNED"
elif disk_exists "$DISK_BASE"; then
	log "Reassigning to base disk $DISK_BASE"
	run ./kompoxops disk assign -V "$VOLUME" -N "$DISK_BASE"
	RESTORE_DISK=$DISK_BASE
else
	log "No disk available for reassignment; keeping $DISK_FROM_SNAPSHOT assigned"
	RESTORE_DISK=$DISK_FROM_SNAPSHOT
fi

log "Deleting unassigned disk $DISK_FROM_SNAPSHOT"
run ./kompoxops disk delete -V "$VOLUME" -N "$DISK_FROM_SNAPSHOT"

mapfile -t CREATED_DISKS < <(printf '%s\n' "${CREATED_DISKS[@]}" | grep -v "^$DISK_FROM_SNAPSHOT$" || true)

log "Creating temporary disk to exercise --source without snapshot prefix"
if ! retry_capture DISK_TEMP_JSON run ./kompoxops disk create -V "$VOLUME" -N "$DISK_TEMP" -S "disk:$DISK_BASE"; then
	log "Failed to create disk from disk source $DISK_BASE after ${RETRY_ATTEMPTS} attempts"
	exit 1
fi
CREATED_DISKS+=("$DISK_TEMP")
assert_field_equals "$DISK_TEMP_JSON" '.name' "$DISK_TEMP" "disk create with disk: source"

log "Deleting temporary disk $DISK_TEMP"
run ./kompoxops disk delete -V "$VOLUME" -N "$DISK_TEMP"
mapfile -t CREATED_DISKS < <(printf '%s\n' "${CREATED_DISKS[@]}" | grep -v "^$DISK_TEMP$" || true)

log "Deleting snapshots using --snap-name"
for SNAP in "$SNAP_EXPLICIT" "$SNAP_AUTO"; do
	run ./kompoxops snapshot delete -V "$VOLUME" --snap-name "$SNAP"
	mapfile -t CREATED_SNAPSHOTS < <(printf '%s\n' "${CREATED_SNAPSHOTS[@]}" | grep -v "^$SNAP$" || true)
done

log "Verifying invalid disk name is rejected"
expect_failure run ./kompoxops disk create -V "$VOLUME" -N "INVALID_NAME"

log "Disk/Snapshot CLI E2E checks completed"
