.PHONY: build test cmd bicep docker release release-snapshot release-check tidy diff-staged-changes test-integration-1

# Run full tests
test:
	go test ./...

# Run full build check
build:
	go build ./...
	go build ./cmd/kompoxops

# Build kompoxops CLI executable
cmd:
	go build ./cmd/kompoxops

# Run go mod tidy
tidy:
	go mod tidy

# Build multi-platform binaries using GoReleaser (requires git tag)
release:
	goreleaser release --clean

# Build multi-platform binaries in snapshot mode (no git tag required)
release-snapshot:
	goreleaser build --snapshot --clean

# Check GoReleaser configuration
release-check:
	goreleaser check

# Show staged changes in Git
diff-staged-changes:
	git status
	git diff --cached

# Build adapters/drivers/provider/aks/main.json to embed in AKS driver
# You need it when you make changes in infra/aks
bicep:
	az bicep build -f infra/aks/infra/main.bicep --outdir adapters/drivers/provider/aks

# Build Docker image
docker:
	docker build -f docker/Dockerfile -t kompoxops .

# Run tests in tests/aks-1
test-aks-1:
	$(MAKE) cmd
	RUN_SH=$$($(CURDIR)/tests/aks-1/setup.sh) && echo "$$RUN_SH" && eval "$$RUN_SH"
