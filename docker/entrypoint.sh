#!/bin/sh

set -e

test $# -eq 0 && set -- /usr/sbin/sshd -D -e

exec /usr/bin/tini -g -- "$@"
