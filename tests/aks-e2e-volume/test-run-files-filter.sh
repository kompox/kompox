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

log "===== Azure Files Volume Filtering Test ====="
log "Testing volume filtering for files backend (vol3 and vol4)"

# Check jq availability
if ! command -v jq >/dev/null 2>&1; then
	log "ERROR: jq is required but not installed"
	exit 1
fi

# Create disks (file shares) in vol3
log ""
log "===== Creating file shares in vol3 ====="
VOL3_DISK1="vol3-s1-${SUFFIX}"
VOL3_DISK2="vol3-s2-${SUFFIX}"

log "Creating file share $VOL3_DISK1 in vol3"
run ./kompoxops disk create -V vol3 -N "$VOL3_DISK1"
CREATED_DISKS+=("vol3|$VOL3_DISK1")

log "Creating file share $VOL3_DISK2 in vol3"
run ./kompoxops disk create -V vol3 -N "$VOL3_DISK2"
CREATED_DISKS+=("vol3|$VOL3_DISK2")

# Create disks (file shares) in vol4
log ""
log "===== Creating file shares in vol4 ====="
VOL4_DISK1="vol4-s1-${SUFFIX}"
VOL4_DISK2="vol4-s2-${SUFFIX}"

log "Creating file share $VOL4_DISK1 in vol4"
run ./kompoxops disk create -V vol4 -N "$VOL4_DISK1"
CREATED_DISKS+=("vol4|$VOL4_DISK1")

log "Creating file share $VOL4_DISK2 in vol4"
run ./kompoxops disk create -V vol4 -N "$VOL4_DISK2"
CREATED_DISKS+=("vol4|$VOL4_DISK2")

# Test vol3 listing
log ""
log "===== Testing vol3 disk list ====="
VOL3_LIST_ALL=$(run ./kompoxops disk list -V vol3)
assert_json_array "$VOL3_LIST_ALL" "vol3 disk list"

# Filter to only test-created disks (prefix: vol3-s)
VOL3_LIST=$(echo "$VOL3_LIST_ALL" | jq '[.[] | select(.name | startswith("vol3-s"))]')
assert_disk_count "$VOL3_LIST" 2 "vol3 filtered disk list"
assert_disk_volume "$VOL3_LIST" "vol3" "vol3 filtered disk list"

# Verify vol3 disks are present
if ! echo "$VOL3_LIST" | jq -e --arg name "$VOL3_DISK1" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL3_DISK1 not found in vol3 list"
	exit 1
fi
log "Verified: $VOL3_DISK1 is in vol3 list"

if ! echo "$VOL3_LIST" | jq -e --arg name "$VOL3_DISK2" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL3_DISK2 not found in vol3 list"
	exit 1
fi
log "Verified: $VOL3_DISK2 is in vol3 list"

# Verify vol4 disks are NOT present in vol3 list
if echo "$VOL3_LIST" | jq -e --arg name "$VOL4_DISK1" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL4_DISK1 from vol4 incorrectly appears in vol3 list"
	exit 1
fi
log "Verified: $VOL4_DISK1 (vol4) is NOT in vol3 list"

if echo "$VOL3_LIST" | jq -e --arg name "$VOL4_DISK2" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL4_DISK2 from vol4 incorrectly appears in vol3 list"
	exit 1
fi
log "Verified: $VOL4_DISK2 (vol4) is NOT in vol3 list"

# Test vol4 listing
log ""
log "===== Testing vol4 disk list ====="
VOL4_LIST_ALL=$(run ./kompoxops disk list -V vol4)
assert_json_array "$VOL4_LIST_ALL" "vol4 disk list"

# Filter to only test-created disks (prefix: vol4-s)
VOL4_LIST=$(echo "$VOL4_LIST_ALL" | jq '[.[] | select(.name | startswith("vol4-s"))]')
assert_disk_count "$VOL4_LIST" 2 "vol4 filtered disk list"
assert_disk_volume "$VOL4_LIST" "vol4" "vol4 filtered disk list"

# Verify vol4 disks are present
if ! echo "$VOL4_LIST" | jq -e --arg name "$VOL4_DISK1" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL4_DISK1 not found in vol4 list"
	exit 1
fi
log "Verified: $VOL4_DISK1 is in vol4 list"

if ! echo "$VOL4_LIST" | jq -e --arg name "$VOL4_DISK2" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL4_DISK2 not found in vol4 list"
	exit 1
fi
log "Verified: $VOL4_DISK2 is in vol4 list"

# Verify vol3 disks are NOT present in vol4 list
if echo "$VOL4_LIST" | jq -e --arg name "$VOL3_DISK1" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL3_DISK1 from vol3 incorrectly appears in vol4 list"
	exit 1
fi
log "Verified: $VOL3_DISK1 (vol3) is NOT in vol4 list"

if echo "$VOL4_LIST" | jq -e --arg name "$VOL3_DISK2" 'map(.name) | index($name) != null' >/dev/null; then
	log "ERROR: Disk $VOL3_DISK2 from vol3 incorrectly appears in vol4 list"
	exit 1
fi
log "Verified: $VOL3_DISK2 (vol3) is NOT in vol4 list"

log ""
log "===== All volume filtering tests passed! ====="
log "Summary:"
log "  - vol3 list shows only vol3 file shares ($VOL3_DISK1, $VOL3_DISK2)"
log "  - vol4 list shows only vol4 file shares ($VOL4_DISK1, $VOL4_DISK2)"
log "  - Cross-volume file share isolation is working correctly"
