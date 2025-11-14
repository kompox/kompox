#!/bin/bash

set -xeuo pipefail

cat kompoxapp.yml

./kompoxops version

./kompoxops cluster provision

./kompoxops cluster status

./kompoxops cluster install

./kompoxops cluster status

./kompoxops app validate

if ./kompoxops app deploy ; then
	echo "ERROR: app deploy succeeded without bootstrap but should fail" >&2
	exit 1
fi

./kompoxops app deploy --bootstrap-disks --update-dns

./kompoxops app validate

./kompoxops app status
