#!/bin/bash

set -xeuo pipefail

./kompoxops version

./kompoxops cluster status

./kompoxops cluster provision

./kompoxops cluster status

./kompoxops cluster install

./kompoxops cluster status

./kompoxops cluster kubeconfig --merge --set-current

kubectl get ns

./kompoxops app deploy --update-dns

./kompoxops app status

./kompoxops app logs

./kompoxops box deploy --ssh-pubkey=$SSH_PUBLIC_KEY

./kompoxops box status

IP=$(./kompoxops cluster status | jq -r .ingressGlobalIP)

HOSTS=$(./kompoxops app status | jq -r .ingress_hosts[])

for HOST in $HOSTS; do
	curl -k --resolve "$HOST:443:$IP" https://$HOST?env=true
done

./kompoxops secret env set -S app -f app-env-override.yml

for HOST in $HOSTS; do
	curl -k --resolve "$HOST:443:$IP" https://$HOST?env=true
done

./kompoxops cluster logs
