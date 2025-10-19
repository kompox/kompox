#!/bin/bash

set -euo pipefail

# ANSI color codes
BOLD='\033[1m'
REVERSE='\033[7m'
RESET='\033[0m'

# Combined styles for labels
LABEL_TEST='\033[1;7;34m'   # Bold + Reverse + Blue
LABEL_PASS='\033[1;7;32m'   # Bold + Reverse + Green
LABEL_FAIL='\033[1;7;31m'   # Bold + Reverse + Red

log() {
	echo -e "${BOLD}>>>${RESET} $*"
}

describe() {
	log "${BOLD}$*${RESET}"
}

run() {
	EXPECTED=$1
	shift
	log "${LABEL_TEST}TEST${RESET} $*"
	if "$@" ; then
		LASTEXITCODE=0
	else
		LASTEXITCODE=$?
	fi
	if test "$EXPECTED" = "$LASTEXITCODE" ; then
		log "${LABEL_PASS}PASS${RESET} expected exit code $LASTEXITCODE"
	else
		log "${LABEL_FAIL}FAIL${RESET} unexpected exit code $LASTEXITCODE, expected $EXPECTED"
	fi
}

describe "Version check"
run 0 ./kompoxops version

describe "Validate all apps in default kompoxapp.yml"
run 0 ./kompoxops app validate

describe "Validate kom0 app (kompoxapp.yml with RefBase allowing file: references)"
run 0 ./kompoxops app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom0

describe "Validate kom1 app (external KOM with file: reference should fail)"
run 1 ./kompoxops app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom1

describe "Validate non-existent app (should fail)"
run 1 ./kompoxops app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/nonexistent

describe "Validate kom2 app (external KOM with inline compose should succeed)"
run 0 ./kompoxops app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom2

describe "Test Defaults.appId with valid existing app"
run 0 ./kompoxops app validate --kom-app kompoxapp-appid-valid.yml

describe "Test Defaults.appId with non-existent app (should fail during load)"
run 1 ./kompoxops app validate --kom-app kompoxapp-appid-invalid.yml

describe "Test KOMPOX_KOM_APP environment variable with valid file"
run 0 env KOMPOX_KOM_APP=kompoxapp-appid-valid.yml ./kompoxops app validate

describe "Test KOMPOX_KOM_APP with non-existent file (should fail - falls back to legacy mode)"
run 1 env KOMPOX_KOM_APP=nonexistent.yml ./kompoxops app validate

describe "Test --kom-path with external directory"
run 0 ./kompoxops --kom-path kom app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom2

describe "Test --kom-path with non-existent path (should fail)"
run 1 ./kompoxops --kom-path nonexistent-dir app validate

describe "Test KOMPOX_KOM_PATH environment variable"
run 0 env KOMPOX_KOM_PATH=kom ./kompoxops app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom2
