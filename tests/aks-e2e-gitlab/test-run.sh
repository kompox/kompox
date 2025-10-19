#!/bin/bash

set -xeuo pipefail

cat kompoxapp.yml

./kompoxops version

./kompoxops cluster provision

./kompoxops cluster status

./kompoxops cluster install

./kompoxops cluster status

./kompoxops app deploy --bootstrap-disks --update-dns

./kompoxops app status
