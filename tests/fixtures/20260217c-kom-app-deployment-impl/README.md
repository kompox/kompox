# 20260217c-kom-app-deployment-impl fixture

## Description

This directory provides fixtures for task [20260217c-kom-app-deployment-impl], based on plan [2026ab-k8s-node-pool-support].

In this phase, KOM deployment fields (`pool/zone/pools/zones`) are aligned with KubeConverter scheduling output (`nodeSelector` / `nodeAffinity`) so NodePool-aware deployment behavior can be validated end-to-end.

Shared resources are placed under `.kompox/kom/`:

- `workspace.yml` (`kind: Workspace`)
- `provider.yml` (`kind: Provider`)
- `cluster.yml` (`kind: Cluster`)

Each `app-valid-*.yml` / `app-invalid-*.yml` file contains only `kind: App` so that patterns can be switched with `--kom-app`.

It is used with `kompoxops app validate` to confirm that:

- `spec.deployment.pools` and `spec.deployment.zones` are accepted.
- generated manifest includes `nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution` with `In` expressions for:
  - `kompox.dev/node-pool`
  - `kompox.dev/node-zone`

Reproduction command:

```bash
kompoxops -C ./tests/fixtures/20260217c-kom-app-deployment-impl app validate --kom-app ./app-valid-pools-zones.yml --out-manifest -
```

## Additional patterns (`--kom-app`)

You can switch fixture patterns by passing `--kom-app`.

- Valid:
  - `app-valid-pools-zones.yml` (`pools + zones`)
  - `app-valid-default.yml` (no deployment settings)
  - `app-valid-pool-zone.yml` (`pool + zone`)
  - `app-valid-pools-only.yml` (`pools` only)
- Invalid (expected validation error):
  - `app-invalid-pool-pools.yml` (`pool + pools`)
  - `app-invalid-zone-zones.yml` (`zone + zones`)
  - `app-invalid-selectors.yml` (`selectors` is reserved)

Examples:

```bash
kompoxops -C ./tests/fixtures/20260217c-kom-app-deployment-impl app validate --kom-app ./app-valid-pool-zone.yml --out-manifest -
```

```bash
kompoxops -C ./tests/fixtures/20260217c-kom-app-deployment-impl app validate --kom-app ./app-invalid-selectors.yml --out-manifest -
```

Bulk trial:

```bash
for f in ./tests/fixtures/20260217c-kom-app-deployment-impl/app-*.yml; do b=$(basename "$f"); echo "=== $b ==="; kompoxops -C ./tests/fixtures/20260217c-kom-app-deployment-impl app validate --kom-app "./$b" --out-manifest - >/dev/null && echo OK || echo NG; done
```

## References

- [20260217c-kom-app-deployment-impl]
- [2026ab-k8s-node-pool-support]

[20260217c-kom-app-deployment-impl]: ../../../design/tasks/2026/02/17/20260217c-kom-app-deployment-impl.ja.md
[2026ab-k8s-node-pool-support]: ../../../design/plans/2026/2026ab-k8s-node-pool-support.ja.md
