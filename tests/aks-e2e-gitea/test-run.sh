#!/bin/bash

set -xeuo pipefail

cat kompoxops.yml

./kompoxops version

./kompoxops cluster provision

./kompoxops cluster status

./kompoxops cluster install

./kompoxops cluster status

N=$(./kompoxops disk list -V default | jq length)
if test "$N" = 0; then
        ./kompoxops disk create -V default
fi

./kompoxops app deploy

./kompoxops app status
