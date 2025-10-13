# API Reference: config/crd/ops/v1alpha1

This package provides CRD-style DTOs and loaders for Kompox v1 CLI, implementing the "folder scan → inmem DB" flow based on [K4x-ADR-007].

## Package Structure

```
config/crd/ops/v1alpha1/
├── doc.go              # Package documentation
├── types.go            # CRD DTO definitions (Workspace, Provider, Cluster, App, Box)
├── fqn.go              # FQN utilities (parsing/validation/parent-child relationships)
├── loader.go           # Directory scanner & YAML loader
├── validator.go        # Topological validation logic
├── sink.go             # Immutable Sink (read-only index)
└── *_test.go           # Unit tests (30+ test cases)
```

## Core Type Definitions

### CRD DTOs (types.go)

All Kinds include `metav1.TypeMeta` and `metav1.ObjectMeta`.

```go
type Workspace struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitzero"`
    Spec              WorkspaceSpec `json:"spec,omitzero"`
}

type Provider struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitzero"`
    Spec              ProviderSpec `json:"spec,omitzero"`
}

type Cluster struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitzero"`
    Spec              ClusterSpec `json:"spec,omitzero"`
}

type App struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitzero"`
    Spec              AppSpec `json:"spec,omitzero"`
}

type Box struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitzero"`
    Spec              BoxSpec `json:"spec,omitzero"`
}  // Placeholder (Spec is empty)
```

**Key Fields:**
- `metadata.annotations["ops.kompox.dev/path"]`: Shorthand for parent path (e.g., `ws/prv/cls`)
- `metadata.name`: Resource name (must conform to DNS-1123 label constraints)

### FQN (Fully Qualified Name) (fqn.go)

```go
type FQN string

// Construction
func BuildFQN(kind, parentPath, name string) (FQN, error)

// Validation
func ValidateSegmentCount(fqn FQN, kind string) error
func ValidateSegmentLabels(fqn FQN) error

// Parent-child relationships
func (f FQN) Segments() []string
func (f FQN) ParentFQN() (FQN, error)
func (f FQN) WorkspaceName() string
func (f FQN) ProviderName() string  // Provider and above only
func (f FQN) ClusterName() string   // Cluster and above only
func (f FQN) AppName() string       // App and above only
func (f FQN) BoxName() string       // Box only

// Helper
func ExtractParentPath(kind string, annotations map[string]string) string
```

**FQN Format:**
- Workspace: `ws`
- Provider: `ws/prv`
- Cluster: `ws/prv/cls`
- App: `ws/prv/cls/app`
- Box: `ws/prv/cls/app/box`

## Usage

### 1. Loading from YAML (Loader)

```go
import "github.com/kompox/kompox/config/crd/ops/v1alpha1"

loader := v1alpha1.NewLoader()

// Recursive directory scan (.yml/.yaml)
docs, err := loader.Load("/path/to/config/dir")
if err != nil {
    log.Fatalf("Failed to load: %v", err)
}

// docs: []Document (TypeMeta, ObjectMeta, FQN, Object)
for _, doc := range docs {
    fmt.Printf("Loaded: Kind=%s, FQN=%s\n", doc.Kind, doc.FQN)
}
```

**Loader Behavior:**
- Recursively scans `.yml` / `.yaml` files
- Supports multi-document YAML (`---` separator)
- Only processes documents with `apiVersion: ops.kompox.dev/v1alpha1`
- Automatically builds FQN from `metadata.annotations["ops.kompox.dev/path"]`
- Validates DNS-1123 label constraints

### 2. Validation

```go
// Validate documents (standalone function)
result := v1alpha1.Validate(docs)
if result.HasErrors() {
    for _, err := range result.Errors {
        fmt.Printf("Validation error: %v\n", err)
    }
}

// Access valid documents
for _, doc := range result.ValidDocuments {
    fmt.Printf("Valid: %s %s\n", doc.Kind, doc.FQN)
}
```

**Validation Rules:**
- **Kind/Segment Consistency**: Validates each Kind has correct number of FQN segments
- **Topological Sort**: Processes in order: Workspace → Provider → Cluster → App → Box
- **Parent Existence**: All Kinds except Workspace require parent resources
- **Duplicate FQN Detection**: Detects conflicts within batch
- **DNS-1123 Constraints**: Segments must be lowercase alphanumeric with hyphens, max 63 chars

### 3. Sink API (sink.go)

The `Sink` is an immutable, read-only index for validated CRD documents. It's designed for single-threaded CLI initialization workflows where all resources are loaded once at startup.

**Design Principles:**
- **Immutable**: Once created via `NewSink()`, the Sink cannot be modified
- **Validation on Construction**: Validates documents during construction
- **Read-Only Access**: Get/List methods return defensive copies to prevent external mutations
- **All-or-Nothing**: Only succeeds if all validations pass (no partial loading)

```go
// Load documents from directory
loader := v1alpha1.NewLoader()
loadResult, err := loader.Load("/path/to/config/dir")
if err != nil {
    log.Fatalf("Failed to load documents: %v", err)
}

// Create sink with validation
sink, err := v1alpha1.NewSink(loadResult.Documents)
if err != nil {
    log.Fatalf("Failed to create sink: %v", err)
}

// List resources (returns defensive copies)
workspaces := sink.ListWorkspaces()
providers := sink.ListProviders()
clusters := sink.ListClusters()
apps := sink.ListApps()
boxes := sink.ListBoxes()

// Get individual resources (returns defensive copy)
ws, found := sink.GetWorkspace("ws")
prv, found := sink.GetProvider("ws/prv")
cls, found := sink.GetCluster("ws/prv/cls")
app, found := sink.GetApp("ws/prv/cls/app")
box, found := sink.GetBox("ws/prv/cls/app/box")

// Statistics
total := sink.Count()
```

**Sink Features:**
- **Single-threaded Design**: Optimized for CLI startup (no locking overhead)
- **All-or-nothing Loading**: Only succeeds if all validations pass
- **Defensive Copying**: Get/List methods return copies to guarantee immutability
- **FQN Primary Key**: Each resource is uniquely identified by its FQN

### 4. Incremental Loading from Multiple Sources

For loading from multiple files or directories (e.g., multiple command-line arguments):

```go
loader := v1alpha1.NewLoader()
var allDocuments []Document

// Load from multiple sources
sources := []string{"/path/to/config1", "/path/to/config2.yaml"}
for _, source := range sources {
    result, err := loader.Load(source)
    if err != nil {
        log.Printf("Warning: failed to load %s: %v", source, err)
        continue
    }
    allDocuments = append(allDocuments, result.Documents...)
}

// Validate all documents together
sink, err := v1alpha1.NewSink(allDocuments)
if err != nil {
    // Validation errors include Path (file path) and Index (document position)
    // Example error: provider "ws1/prv1" validation error: parent "ws1" does not exist from /path/to/config2.yaml (document 1)
    log.Fatalf("Validation failed: %v", err)
}
```

**Document Tracking:**
- `Document.Path`: The file path from which the document was loaded
- `Document.Index`: 1-based position within the source file (useful for multi-document YAML files)
- `ValidationError.Path` and `ValidationError.Index`: Include source location in error messages for easy debugging

### 5. End-to-End Example

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/kompox/kompox/config/crd/ops/v1alpha1"
)

func main() {
    // Load documents from directory
    loader := v1alpha1.NewLoader()
    loadResult, err := loader.Load("/path/to/kompox/config")
    if err != nil {
        log.Fatalf("Failed to load: %v", err)
    }
    
    // Create sink with validation
    sink, err := v1alpha1.NewSink(loadResult.Documents)
    if err != nil {
        log.Fatalf("Failed to create sink: %v", err)
    }
    
    // Verify results
    fmt.Printf("Loaded %d resources:\n", sink.Count())
    fmt.Printf("  Workspaces: %d\n", len(sink.ListWorkspaces()))
    fmt.Printf("  Providers:  %d\n", len(sink.ListProviders()))
    fmt.Printf("  Clusters:   %d\n", len(sink.ListClusters()))
    fmt.Printf("  Apps:       %d\n", len(sink.ListApps()))
    fmt.Printf("  Boxes:      %d\n", len(sink.ListBoxes()))
    
    // Access specific resources
    if ws, ok := sink.GetWorkspace("myworkspace"); ok {
        fmt.Printf("Found workspace: %s\n", ws.Name)
    }
}
```

## YAML Format Examples

```yaml
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: myworkspace
spec:
  displayName: "My Workspace"

---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: azprovider
  annotations:
    ops.kompox.dev/path: "myworkspace"  # Parent Workspace
spec:
  type: azure
  subscriptionID: "12345678-1234-1234-1234-123456789abc"

---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: devcluster
  annotations:
    ops.kompox.dev/path: "myworkspace/azprovider"  # Parent Provider
spec:
  resourceGroup: "rg-dev"
  location: "japaneast"
  nodeCount: 3

---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: webapp
  annotations:
    ops.kompox.dev/path: "myworkspace/azprovider/devcluster"  # Parent Cluster
spec:
  composePath: "./docker-compose.yml"
  namespace: "default"
```

## Error Handling

```go
// ValidationError: contains multiple validation errors
type ValidationError struct {
    Errors []error
}

func (e *ValidationError) Error() string {
    // Concatenates all errors with newlines
}
```

**Common Error Causes:**
- Parent resource does not exist
- Duplicate FQN
- DNS-1123 constraint violation (uppercase/symbols/name too long)
- Segment count mismatch with Kind
- YAML parsing error

## Test Coverage

- `fqn_test.go`: FQN parsing, validation, parent-child relationships (15+ tests)
- `loader_test.go`: Directory scanning, YAML decoding (10+ tests)
- `validator_test.go`: Topological validation, parent resolution (8+ tests)
- `sink_test.go`: Staging, commit, CRUD operations (9+ tests)

All tests can be run with `make test`.

## API Stability

This package implements the `ops.kompox.dev/v1alpha1` API group and version:
- **API Group**: `ops.kompox.dev`
- **Version**: `v1alpha1`
- **Stability**: Alpha (subject to breaking changes)

## Dependencies

- `k8s.io/apimachinery`: For TypeMeta, ObjectMeta, and YAML decoding
- Standard library: `os`, `path/filepath`, `strings`, `fmt`, `sync`

## Future Work

- **RDB Implementation**: Migrate from in-memory to persistent storage (FQN UNIQUE + UUID PK)
- **CLI Subcommands**: Implement `import`, `plan`, `app`, `box` commands
- **Operator/CRD**: Deploy to actual Kubernetes clusters
- **Box Specification**: Detailed implementation per ADR-008

## References

- [K4x-ADR-007]: CRD-style configuration
- [Kompox-CRD.ja.md]: CRD specification (Japanese)
- [2025-10-13-crd.ja.md]: Implementation task tracking

[K4x-ADR-007]: ../../../../design/adr/K4x-ADR-007.md
[Kompox-CRD.ja.md]: ../../../../design/v1/Kompox-CRD.ja.md
[2025-10-13-crd.ja.md]: ../../../../_dev/tasks/2025-10-13-crd.ja.md
