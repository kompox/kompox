#!/bin/bash

set -e

RUN_DIR=$(cd $(dirname $0); pwd)

cd $RUN_DIR

echo "Running in $RUN_DIR"

export KUBECONFIG="$RUN_DIR/kubeconfig"
touch "$KUBECONFIG"
chmod 600 "$KUBECONFIG"

export SSH_DIR="$RUN_DIR/ssh"
export SSH_PRIVATE_KEY="$SSH_DIR/id_rsa"
export SSH_PUBLIC_KEY="$SSH_DIR/id_rsa.pub"
mkdir -m 700 -p "$SSH_DIR"
if ! test -r "$SSH_PRIVATE_KEY"; then
    ssh-keygen -N '' -t rsa -f "$SSH_PRIVATE_KEY"
fi

export KOMPOXOPS_DB_URL="file:$RUN_DIR/kompoxops.yml"

set -x

cat kompoxops.yml

./kompoxops version

./kompoxops cluster status

./kompoxops cluster provision

./kompoxops cluster status

./kompoxops cluster install

./kompoxops cluster status

./kompoxops cluster kubeconfig --merge --set-current

kubectl get ns

./kompoxops app deploy

./kompoxops app status

./kompoxops app logs

./kompoxops box deploy --ssh-pubkey=$SSH_PUBLIC_KEY

./kompoxops box status

IP=$(./kompoxops cluster status | jq -r .ingressGlobalIP)

HOSTS=$(./kompoxops app status | jq -r .ingress_hosts[])

for HOST in $HOSTS; do
	curl -k --resolve "$HOST:443:$IP" https://$HOST
done

./kompoxops cluster logs
