# 20260215b-networkpolicy-empty-peer fixture

## Description

This directory is a minimal fixture for reproducing the issue tracked by task [20260215b-networkpolicy-empty-peer].

It is used with `kompoxops app validate` to confirm that generated `NetworkPolicy.spec.ingress[].from` currently contains `- {}`.

Reproduction command:

`kompoxops -C ./tests/fixtures/20260215b-networkpolicy-empty-peer app validate --out-manifest -`

## References

- [20260215b-networkpolicy-empty-peer]

[20260215b-networkpolicy-empty-peer]: ../../../design/tasks/2026/02/15/20260215b-networkpolicy-empty-peer.ja.md
