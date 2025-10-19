#!/bin/bash

set -euo pipefail

./kompoxops dns destroy || true

./kompoxops cluster deprovision
