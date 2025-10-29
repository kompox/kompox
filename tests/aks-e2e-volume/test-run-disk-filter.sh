#!/bin/bash

set -euo pipefail

SUFFIX=$(date +%H%M%S)

log() {
	printf '[%s] %s\n' "$(date --iso-8601=seconds)" "$*" >&2
}

run() {
	local q
	printf -v q '%q ' "$@"
	log "+ ${q% }"
	"$@"
}

assert_json_array() {
	local json=$1 desc=$2
	echo "$json" | jq -e 'type == "array"' >/dev/null
	log "$desc returned JSON array (${#json} bytes)"
}

assert_disk_count() {
	local json=$1 expected=$2 desc=$3
	local actual
	actual=$(echo "$json" | jq -r 'length')
	if [[ "$actual" != "$expected" ]]; then
		log "Assertion failed for $desc: expected $expected disks but got $actual"
		echo "$json" | jq '.'
		exit 1
	fi
	log "$desc: found $actual disk(s) as expected"
}

assert_disk_volume() {
	local json=$1 expected_volume=$2 desc=$3
	local volumes
	volumes=$(echo "$json" | jq -r '.[].volumeName' | sort -u)
	local count
	count=$(echo "$volumes" | wc -l)
	if [[ "$count" != "1" ]] || [[ "$volumes" != "$expected_volume" ]]; then
		log "Assertion failed for $desc: expected all disks to belong to volume '$expected_volume'"
		log "Found volumes: $volumes"
		echo "$json" | jq '.'
		exit 1
	fi
	log "$desc: all disks belong to volume '$expected_volume'"
}

WORKDIR=$(mktemp -d)
trap 'cleanup' EXIT

CREATED_DISKS=()

cleanup() {
	set +e
	for entry in "${CREATED_DISKS[@]}"; do
		IFS='|' read -r vol disk <<<"$entry"
		log "Cleaning disk $disk from volume $vol"
		run ./kompoxops disk delete -V "$vol" --disk-name "$disk" >/dev/null 2>&1 || true
	done
	rm -rf "$WORKDIR"
}

log "===== Azure Disk Volume Filtering Test ====="
log "Testing volume filtering for disk backend (vol1 and vol2)"

# Check jq availability
if ! command -v jq >/dev/null 2>&1; then
	log "ERROR: jq is required but not installed"
	exit 1
fi

# Create disks in vol1
log ""
log "===== Creating disks in vol1 ====="
VOL1_DISK1="vol1-d1-${SUFFIX}"
VOL1_DISK2="vol1-d2-${SUFFIX}"

log "Creating disk $VOL1_DISK1 in vol1"
run ./kompoxops disk create -V vol1 -N "$VOL1_DISK1"
CREATED_DISKS+=("vol1|$VOL1_DISK1")

log "Creating disk $VOL1_DISK2 in vol1"
run ./kompoxops disk create -V vol1 -N "$VOL1_DISK2"
CREATED_DISKS+=("vol1|$VOL1_DISK2")

# Create disks in vol2
log ""
log "===== Creating disks in vol2 ====="
VOL2_DISK1="vol2-d1-${SUFFIX}"
VOL2_DISK2="vol2-d2-${SUFFIX}"

log "Creating disk $VOL2_DISK1 in vol2"
run ./kompoxops disk create -V vol2 -N "$VOL2_DISK1"
CREATED_DISKS+=("vol2|$VOL2_DISK1")

log "Creating disk $VOL2_DISK2 in vol2"
run ./kompoxops disk create -V vol2 -N "$VOL2_DISK2"
CREATED_DISKS+=("vol2|$VOL2_DISK2")

# Test vol1 listing
log ""
log "===== Testing vol1 disk list ====="
VOL1_LIST_ALL=$(run ./kompoxops disk list -V vol1)
assert_json_array "$VOL1_LIST_ALL" "vol1 disk list"

# Filter to only test-created disks (prefix: vol1-d)
VOL1_LIST=$(echo "$VOL1_LIST_ALL" | jq '[.[] | select(.name | startswith("vol1-d"))]')
assert_disk_count "$VOL1_LIST" 2 "vol1 filtered disk list"
assert_disk_volume "$VOL1_LIST" "vol1" "vol1 filtered disk list"

# Verify vol1 disks are present
if ! echo "$VOL1_LIST" | jq -e --arg name "$VOL1_DISK1" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL1_DISK1 not found in vol1 list"
	exit 1
fi
log "Verified: $VOL1_DISK1 is in vol1 list"

if ! echo "$VOL1_LIST" | jq -e --arg name "$VOL1_DISK2" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL1_DISK2 not found in vol1 list"
	exit 1
fi
log "Verified: $VOL1_DISK2 is in vol1 list"

# Verify vol2 disks are NOT present in vol1 list
if echo "$VOL1_LIST" | jq -e --arg name "$VOL2_DISK1" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL2_DISK1 from vol2 incorrectly appears in vol1 list"
	exit 1
fi
log "Verified: $VOL2_DISK1 (vol2) is NOT in vol1 list"

if echo "$VOL1_LIST" | jq -e --arg name "$VOL2_DISK2" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL2_DISK2 from vol2 incorrectly appears in vol1 list"
	exit 1
fi
log "Verified: $VOL2_DISK2 (vol2) is NOT in vol1 list"

# Test vol2 listing
log ""
log "===== Testing vol2 disk list ====="
VOL2_LIST_ALL=$(run ./kompoxops disk list -V vol2)
assert_json_array "$VOL2_LIST_ALL" "vol2 disk list"

# Filter to only test-created disks (prefix: vol2-d)
VOL2_LIST=$(echo "$VOL2_LIST_ALL" | jq '[.[] | select(.name | startswith("vol2-d"))]')
assert_disk_count "$VOL2_LIST" 2 "vol2 filtered disk list"
assert_disk_volume "$VOL2_LIST" "vol2" "vol2 filtered disk list"

# Verify vol2 disks are present
if ! echo "$VOL2_LIST" | jq -e --arg name "$VOL2_DISK1" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL2_DISK1 not found in vol2 list"
	exit 1
fi
log "Verified: $VOL2_DISK1 is in vol2 list"

if ! echo "$VOL2_LIST" | jq -e --arg name "$VOL2_DISK2" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL2_DISK2 not found in vol2 list"
	exit 1
fi
log "Verified: $VOL2_DISK2 is in vol2 list"

# Verify vol1 disks are NOT present in vol2 list
if echo "$VOL2_LIST" | jq -e --arg name "$VOL1_DISK1" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL1_DISK1 from vol1 incorrectly appears in vol2 list"
	exit 1
fi
log "Verified: $VOL1_DISK1 (vol1) is NOT in vol2 list"

if echo "$VOL2_LIST" | jq -e --arg name "$VOL1_DISK2" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL1_DISK2 from vol1 incorrectly appears in vol2 list"
	exit 1
fi
log "Verified: $VOL1_DISK2 (vol1) is NOT in vol2 list"

log ""
log "===== All volume filtering tests passed! ====="
log "Summary:"
log "  - vol1 list shows only vol1 disks ($VOL1_DISK1, $VOL1_DISK2)"
log "  - vol2 list shows only vol2 disks ($VOL2_DISK1, $VOL2_DISK2)"
log "  - Cross-volume disk isolation is working correctly"
