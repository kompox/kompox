.PHONY: build test cmd bicep docker release release-snapshot release-check tidy diff-staged-changes test-integration-1 gen-index git-hooks-setup
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

# Show staged changes that are not yet committed and suggest next commit message file
git-diff-cached:
	@if ! git --no-pager diff --cached --exit-code; then \
		echo; \
		last=$$(ls _tmp/git-commit/[0-9]*.txt 2>/dev/null | sort -V | tail -1 | sed 's/.*\///' | sed 's/.txt//' | sed 's/^0*//'); \
		if [ -n "$$last" ]; then \
			next=$$(printf "%04d.txt" $$(($$last + 1))); \
		else \
			next="0001.txt"; \
		fi; \
		echo "Next commit message file: _tmp/git-commit/$$next"; \
	else \
		echo "No staged changes to commit."; \
	fi

# Open the last commit message file in editor for editing and commit
git-commit-with-editor:
	git -c core.editor='code --wait' commit -v -e -F $(lastword $(sort $(wildcard _tmp/git-commit/[0-9]*.txt)))

# Show the last commit details and current git status
git-show:
	git --no-pager show -1 --name-status --pretty=fuller && git status

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

# Configure git to use repository-managed hooks
git-hooks-setup:
	mkdir -p .githooks
	chmod +x .githooks/*
	git config core.hooksPath .githooks
	@echo "Configured core.hooksPath=.githooks"

.PHONY: docs-setup docs-serve docs-build docs-deploy-edge docs-deploy-version

docs-setup:
	uv sync

docs-serve:
	uv run mkdocs serve --livereload -a 0.0.0.0:8000

docs-build:
	uv run mkdocs build

docs-deploy-edge:
	uv run mike deploy --push --update-aliases edge

# usage: make docs-deploy-version VERSION=0.3
docs-deploy-version:
	@[ -n "$$VERSION" ] || (echo "VERSION is required. e.g., make docs-deploy-version VERSION=0.3" && exit 1)
	uv run mike deploy --push --update-aliases v$(VERSION) latest
	uv run mike set-default --push latest

.PHONY: docs-clean-venv
docs-clean-venv:
	rm -rf .venv
