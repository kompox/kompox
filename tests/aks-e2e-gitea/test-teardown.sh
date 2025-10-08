#!/bin/bash

set -euo pipefail

./kompoxops dns destroy

./kompoxops cluster deprovision
