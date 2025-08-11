# Development Guide

## Project Structure

```
.
├── cmd/
│   └── kompoxops/           # Main CLI application
│       └── main.go          # Entry point
├── cluster/                 # Kubernetes cluster handling
│   └── providers/           # Kubernetes and cloud service providers
│       ├── aks/             # AKS and Azure provider
│       └── k3s/             # K3s provider
├── docs/                    # Documentation
├── .devcontainer/           # VS Code Dev Container configuration
│   └── devcontainer.json
├── go.mod                   # Go module definition
├── Makefile                 # Build automation
└── README.md
```

## Development Workflow

1. **Setup**: Use VS Code Dev Container for consistent environment
2. **Code**: Edit source files in `cmd/` and `pkg/`
3. **Build**: Use `make build` to compile
4. **Test**: Use `make test` to run tests
5. **Format**: Use `make fmt` and `make vet` for code quality

## Implementation Plan

Based on `docs/Kompox-Spec-Draft.ja.md`, the kompoxops CLI should implement:

### Core Commands

- `kompoxops init` - Create kompoxops.yml template

### Cluster Commands

- `kompoxops cluster info` - Show cluster information
- `kompoxops cluster deploy` - Deploy traefik ingress controller

### App Commands

- `kompoxops app validate` - Validate compose.yml and output K8s manifests
- `kompoxops app deploy` - Deploy compose.yml
- `kompoxops app destroy` - Delete deployment (disk remains)

### Disk Management (handled by providers)

- `kompoxops disk list` - List disk resources
- `kompoxops disk attach` - Replace disk resource
- `kompoxops disk import` - Import disk resource
- `kompoxops disk export` - Export disk resource
- `kompoxops disk delete` - Delete disk resource

### Snapshot Management (handled by providers)

- `kompoxops snapshot list` - List snapshot resources
- `kompoxops snapshot create` - Create snapshot resource
- `kompoxops snapshot restore` - Restore snapshot resource
- `kompoxops snapshot export` - Export snapshot resource
- `kompoxops snapshot delete` - Delete snapshot resource

## Configuration

The tool reads `kompoxops.yml` configuration file with the following structure:

```yaml
version: 1
service:
  name: ops
  domain: ops.kompox.dev
cluster:
  name: my-aks
  auth:
    type: kubectl
    kubeconfig: ~/.kube/config
    context: my-aks
  ingress:
    controller: traefik
    namespace: traefik
  provider: aks
  settings:
    # Provider-specific settings
app:
  name: my-app
  compose: compose.yml
  ingress:
    http_80: www.my-app.kompox.dev
    http_8080: admin.my-app.kompox.dev
  resources:
    cpu: 500m
    memory: 1Gi
  settings:
    # Provider-specific settings
```

## Cloud Provider Support

The tool should support:
- Azure AKS
- Google Cloud GKE
- AWS EKS
- Oracle Cloud OKE
- Self-hosted K3s clusters