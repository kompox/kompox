.PHONY: build test cmd bicep docker release release-snapshot release-check tidy diff-staged-changes test-integration-1 gen-design-index
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

# Run integration/e2e tests in _tmp/tests/<test-name> directory
TEST_DIR := tests/aks-e2e-basic
TEST_NAME := $(notdir $(patsubst %/,%,$(TEST_DIR)))
TEST_RUN_DIR := $(shell d=_tmp/tests/$(TEST_NAME); i=''; while test -d $$d$$i; do i=$$(($$i-1)); done; echo $$d$$i)
.PHONY: test-integration test-e2e e2e
test-integration test-e2e e2e:
	@echo "Testing $(TEST_DIR) in directory: $(TEST_RUN_DIR)" >&2
	mkdir -p _tmp/tests
	cp -r $(TEST_DIR) $(TEST_RUN_DIR)
test-integration-run test-e2e-run e2e-run: test-integration
	$(MAKE) -C $(TEST_RUN_DIR) all | tee -a $(TEST_RUN_DIR)/test.log

# Generate design indexes (en/ja)
gen-design-index:
	go run ./design/gen -design-dir design -lang en
	go run ./design/gen -design-dir design -lang ja
