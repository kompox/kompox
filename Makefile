.PHONY: build clean test fmt vet run

# Build the kompoxops binary
build:
	go build -o bin/kompoxops ./cmd/kompoxops

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Run the application (with help by default)
run: build
	./bin/kompoxops help

# Install dependencies
deps:
	go mod tidy
	go mod download

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o bin/kompoxops-linux-amd64 ./cmd/kompoxops
	GOOS=darwin GOARCH=amd64 go build -o bin/kompoxops-darwin-amd64 ./cmd/kompoxops
	GOOS=windows GOARCH=amd64 go build -o bin/kompoxops-windows-amd64.exe ./cmd/kompoxops