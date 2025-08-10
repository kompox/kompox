# kompoxops

Kompox cloud resource deployment and operations tool.

## Development

This project is set up for development with VS Code Dev Containers, providing a consistent development environment with all necessary tools.

### Prerequisites

- Docker
- VS Code with Dev Containers extension

### Getting Started

1. Clone this repository
2. Open in VS Code
3. When prompted, click "Reopen in Container" or use Command Palette: "Dev Containers: Reopen in Container"
4. The dev container will automatically set up the Go environment and install dependencies

### Building

```bash
# Build the binary
make build

# Run the application
make run

# Clean build artifacts
make clean

# Run tests
make test

# Format and vet code
make fmt
make vet
```

### Manual Setup (without Dev Container)

If you prefer to set up the development environment manually:

1. Install Go 1.23 or later
2. Install kubectl
3. Install Docker
4. Clone the repository
5. Run `go mod tidy` to install dependencies
6. Build with `make build`

## Usage

```bash
# Show help
./bin/kompoxops help

# Show version
./bin/kompoxops version
```

See `docs/Kompox-Spec-Draft.ja.md` for the complete specification.