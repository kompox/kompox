---
id: K4x-ADR-004
title: Cluster ingress endpoint DNS auto-update
status: accepted
date: 2025-09-30
supersedes: []
supersededBy: []
---

## Context

- In Kompox, ingress hostnames are defined in `kompoxops.yml` via `app.ingress.rules[].hosts` and become accessible once the app is deployed to a cluster with an ingress controller. Historically, users needed to manually configure their DNS zone to map these hostnames to the cluster ingress endpoint IP address.
- This manual step is error-prone, time-consuming, and complicates automation. Some providers support first-class DNS services that we can leverage to reduce operational toil.
- We want to introduce an optional capability to apply DNS records for ingress endpoints automatically, while keeping provider-specific DNS integration encapsulated in each provider driver.
- Requirements and constraints:
  - Optional feature (opt-in via configuration or usage path). Existing workflows remain valid.
  - Provider-agnostic API surface in the domain layer; provider-specific resolution and write logic in drivers.
  - Idempotent and best-effort semantics by default. DNS write failures should not break core workflows.
  - Allow warnings/logging when updates fail; only escalate to errors when explicitly requested.
  - DNS records must reflect actual deployed state (ingress hosts with LoadBalancer IPs from deployed Kubernetes Ingress resources) rather than configuration intent alone. This ensures DNS management operates on real operational state.

## Decision

Introduce a generic DNS apply capability centered around a record-set abstraction and wire it through the domain port and provider drivers. Implementation details (types, method signatures, flags) are documented in `_dev/tasks`。

- Domain model
  - Add a provider-agnostic record-set model and type identifiers (e.g., A/AAAA/CNAME/TXT/MX/NS/SRV/CAA) to represent the desired DNS state per FQDN and type.

- Domain port
  - Expose `DNSApply` to apply a single record set per call with options for zone hint, strict mode, and dry-run.
  - Semantics: idempotent and best-effort by default; provider write failures do not surface as errors unless strict mode is enabled.

- Provider drivers
  - Provide a corresponding DNS apply operation and encapsulate provider-specific behaviors including zone discovery/selection, ingress endpoint resolution, and record formatting.

- Scope for ingress automation
  - Drivers may implement helper logic to discover the cluster ingress public address (e.g., LoadBalancer IPs from Kubernetes Ingress resources) and upsert `A/AAAA` records for the FQDNs obtained from deployed apps.
  - DNS management targets actual deployed ingress hosts retrieved via `kube.Client.IngressHosts()` which returns FQDN and LoadBalancer IP pairs from Kubernetes Ingress resources. This ensures DNS reflects real operational state rather than configuration values.
  - The exact way manageable zones are chosen depends on driver configuration (e.g., zone hints via `--zone` flag, provider credentials, and `cluster.settings`).

- Use case layer
  - Introduce `usecase/dns` to orchestrate DNS operations. The use case:
    - Accepts AppID as the primary input to identify the target application.
    - Uses `kube.Client.IngressHosts(ctx, namespace, labelSelector)` to retrieve FQDN and LoadBalancer IP pairs from actual deployed Kubernetes Ingress resources.
    - Constructs DNS record sets (A records) inline with the retrieved IPs.
    - Calls `ClusterPort.DNSApply` for each FQDN with appropriate options.

- CLI
  - Add `kompoxops dns deploy` and `kompoxops dns destroy` commands with `--app` flag to specify target application.
  - `app deploy/destroy` may optionally invoke the corresponding DNS operation via `--update-dns` flag, passing AppID directly to the DNS use case.
  - Common flags: `--zone` (zone hint), `--strict` (error on write failures), `--dry-run` (show planned changes without applying).
  - Rationale: aligns with deploy/destroy verbs; DNS deploy covers create/update, DNS destroy covers removal of records deterministically associated to the app (no GC of orphans).

### Operation timing and cleanup policy

DNS records are applied and removed during explicit lifecycle operations: via `kompoxops dns deploy/destroy` or, when `--update-dns` is provided, as part of `kompoxops app deploy/destroy`. The system does not perform background garbage collection; only records deterministically associated with the app are touched, and orphaned or otherwise untrackable records (e.g., after manual `kompoxops.yml` edits) are left unchanged.

## Alternatives Considered

1) Ensure-style API ("DNSEnsure")
    - Pros: conveys intent to achieve a state.
    - Cons: implies strong error semantics; our default must tolerate provider write failures without failing workflows. Rejected.

2) Centralized DNS client in core (no driver delegation)
    - Pros: uniform logic in one place.
    - Cons: would embed provider-specific complexity into core, increase credentials surface, and reduce extensibility. Rejected.

3) Batch apply for multiple record sets in one call
    - Pros: fewer round-trips; bulk operations.
    - Cons: increases API surface and complexity now; can be added later without breaking the single-record API. Deferred.

4) CLI-only automation (scripts) instead of API
    - Pros: minimal code changes.
    - Cons: lacks domain/driver encapsulation and is harder to validate and evolve. Rejected.

## Consequences

- Pros
  - Reduces manual DNS steps for ingress hostnames; improves out-of-the-box automation.
  - Keeps provider-specific logic and credentials localized to drivers.
  - Idempotent best-effort semantics minimize impact on existing flows and resilience during intermittent provider issues.

- Cons/Constraints
  - Exact DNS behavior depends on provider capabilities and configuration; behavior is not fully uniform across drivers.
  - Additional logging and observability are needed to make best-effort outcomes visible to operators.
  - Deletion-by-empty-values is simple but requires clear documentation to avoid surprises.

## Rollout

1) Domain and interface additions
  - Add `DNSRecordType`, `DNSRecordSet`.
  - Extend `ClusterPort` with `DNSApply` and consolidate `ClusterDNSApplyOptions` in `cluster_port.go`.
  - Add `ClusterDNSApply` to provider driver interface.

2) Driver stubs
  - AKS and K3s drivers implement no-op placeholders that log and return nil.

3) Provider implementations
  - Implement DNS updates per provider (e.g., Azure DNS for AKS) with support for `ZoneHint`, `Strict`, and `DryRun`.

4) Use case and CLI wiring
  - Add `usecase/dns` and wire `kompoxops dns deploy/destroy` to it; allow `kompoxops app deploy/destroy --update-dns` to call the same use case flows.

## References

- Tasks
  - [2025-09-30-cluster-dns] - Initial design and planning task
  - [2025-10-01-cluster-dns] - Current implementation task with detailed specifications

[2025-09-30-cluster-dns]: ../../_dev/tasks/2025-09-30-cluster-dns.ja.md
[2025-10-01-cluster-dns]: ../../_dev/tasks/2025-10-01-cluster-dns.ja.md
