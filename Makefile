.PHONY: build test cmd bicep docker release release-snapshot release-check tidy diff-staged-changes test-integration-1 gen-index
# Run full tests
test:
	go test ./...

# Run full build check
build:
	go build ./...
	go build ./cmd/kompoxops
	bash _dev/bin/install-kompoxops.sh

# Build kompoxops CLI executable
cmd:
	go build ./cmd/kompoxops
	bash _dev/bin/install-kompoxops.sh

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

# Run integration/e2e tests in _tmp/tests/<test-name>-N directory
TEST_DIR := tests/aks-e2e-basic
.PHONY: test-integration test-e2e e2e
test-integration test-e2e e2e:
	bash _dev/bin/setup-test.sh $(TEST_DIR)

# Generate indexes for design and maintainer tasks
gen-index:
	go run ./design/gen -design-dir design

.PHONY: docs-setup docs-serve docs-build docs-deploy-edge docs-deploy-version

MKDOCS := $(CURDIR)/.venv/bin/mkdocs
MIKE   := $(CURDIR)/.venv/bin/mike

docs-setup:
	bash _dev/bin/setup-docs.sh

docs-serve:
	$(MKDOCS) serve -a 0.0.0.0:8000

docs-build:
	$(MKDOCS) build

docs-deploy-edge:
	$(MIKE) deploy --push --update-aliases edge

# usage: make docs-deploy-version VERSION=0.3
docs-deploy-version:
	@[ -n "$$VERSION" ] || (echo "VERSION is required. e.g., make docs-deploy-version VERSION=0.3" && exit 1)
	$(MIKE) deploy --push --update-aliases v$(VERSION) latest
	$(MIKE) set-default --push latest

.PHONY: docs-clean-venv
docs-clean-venv:
	rm -rf .venv
