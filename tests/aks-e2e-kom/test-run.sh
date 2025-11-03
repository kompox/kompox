#!/bin/bash

set -euo pipefail

# Test Rules:
# - All tests assume .kompox/kom does not exist before each test
# - If a test creates .kompox/kom, it must remove it at the end
# - This script removes .kompox/kom at the beginning to ensure clean state

# KOM Path Priority (5 levels, from highest to lowest):
# Level 1: --kom-path flag
# Level 2: KOMPOX_KOM_PATH environment variable
# Level 3: Defaults.spec.komPath in kompoxapp.yml
# Level 4: komPath in .kompox/config.yml
# Level 5: Default path $KOMPOX_DIR/kom (ignored if missing)

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

# Ensure .kompox/kom does not exist at the start
rm -rf .kompox/kom .kompox-*/kom

describe "Starting E2E test for kompox kom app validate"

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

describe "Test Level 1: --kom-path flag"
run 0 ./kompoxops --kom-path kom --kom-app kompoxapp-nokompath.yml app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom-nokompath2

describe "Test Level 1: --kom-path with non-existent path (should fail)"
run 1 ./kompoxops --kom-path nonexistent-dir --kom-app kompoxapp-nokompath.yml app validate

describe "Test Level 2: KOMPOX_KOM_PATH environment variable"
run 0 env KOMPOX_KOM_PATH=kom ./kompoxops --kom-app kompoxapp-nokompath.yml app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom-nokompath2

describe "Test Level 4: komPath in .kompox-kompath/config.yml"
# .kompox-kompath/config.yml has komPath: [kom] which is relative to $KOMPOX_DIR
# So it looks for .kompox-kompath/kom, we need to copy kom/ there
cp -r kom .kompox-kompath/kom
run 0 ./kompoxops --kompox-dir .kompox-kompath --kom-app kompoxapp-nokompath.yml app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom-nokompath2
rm -rf .kompox-kompath/kom

describe "Test Level 4: komPath in .kompox-kompath/config.yml without KOM documents (should fail)"
# .kompox-kompath/config.yml has komPath: [kom] but .kompox-kompath/kom/ doesn't exist
# This tests that Level 4 (config.yml komPath) requires the path to exist
# Use kompoxapp-full.yml which has Workspace/Provider/Cluster/App, so admin app list can work properly
run 1 ./kompoxops --kompox-dir .kompox-kompath --kom-app kompoxapp-full.yml admin app list

describe "Test Level 5: default path \$KOMPOX_DIR/kom (ignored when missing)"
# .kompox/config.yml has no komPath, so Level 5 default path .kompox/kom is used
# Unlike Level 4, Level 5 default path is ignored if it doesn't exist, so this succeeds
# Use kompoxapp-full.yml which has Workspace/Provider/Cluster/App, so admin app list can work properly
run 0 ./kompoxops --kom-app kompoxapp-full.yml admin app list

describe "Test --kompox-root flag"
run 0 ./kompoxops --kompox-root . app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom2

describe "Test --kompox-dir flag"
run 0 ./kompoxops --kompox-dir .kompox app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom2

describe "Test -C (chdir) flag"
run 0 ./kompoxops -C . app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom2

describe "Test KOMPOX_ROOT environment variable"
run 0 env KOMPOX_ROOT=$(pwd) ./kompoxops app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom2

describe "Test KOMPOX_DIR environment variable"
run 0 env KOMPOX_DIR=$(pwd)/.kompox ./kompoxops app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom2

describe "Test priority: --kom-path overrides KOMPOX_KOM_PATH"
run 0 env KOMPOX_KOM_PATH=nonexistent ./kompoxops --kom-path kom --kom-app kompoxapp-nokompath.yml app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom-nokompath2

describe "Test priority: KOMPOX_KOM_PATH overrides config.yml"
run 0 env KOMPOX_KOM_PATH=kom ./kompoxops --kom-app kompoxapp-nokompath.yml app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom-nokompath2

describe "Test error message clarity with non-existent --kom-path"
run 1 ./kompoxops --kom-path /nonexistent/path app validate

describe "Test error message clarity with non-existent KOMPOX_KOM_PATH"
run 1 env KOMPOX_KOM_PATH=/nonexistent/path ./kompoxops app validate

describe "Test \$KOMPOX_ROOT variable expansion in paths"
run 0 ./kompoxops --kom-path '$KOMPOX_ROOT/kom' app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom2

describe "Test \$KOMPOX_DIR variable expansion with non-existent path"
run 1 ./kompoxops --kom-path '$KOMPOX_DIR/kom' app validate

describe "Test \$KOMPOX_DIR variable expansion with created directory"
cp -r kom .kompox/kom
run 0 ./kompoxops --kom-path '$KOMPOX_DIR/kom' app validate -A /ws/${SERVICE_NAME}/prv/aks1/cls/cluster1/app/kom2
rm -rf .kompox/kom
