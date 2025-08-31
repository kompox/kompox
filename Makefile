.PHONY: build test cmd bicep

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

# Show staged changes in Git
diff-staged-changes:
	git status
	git diff --cached

# Build adapters/drivers/provider/aks/main.json to embed in AKS driver
# You need it when you make changes in infra/aks
bicep:
	az bicep build -f infra/aks/infra/main.bicep --outdir adapters/drivers/provider/aks
