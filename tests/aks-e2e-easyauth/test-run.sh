#!/bin/bash

set -xeuo pipefail

cat kompoxops.yml

./kompoxops version

./kompoxops cluster provision

./kompoxops cluster status

./kompoxops cluster install

./kompoxops cluster status

./kompoxops app deploy --bootstrap-disks --update-dns

./kompoxops app status
