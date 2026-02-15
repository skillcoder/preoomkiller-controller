# Build configuration
BINARY_NAME := "preoomkiller-controller"
CMD_PATH := "./cmd/" + BINARY_NAME
BUILD_DIR := "./bin"

# Default recipe
default: build

# Build the application
build:
    go build -o {{BUILD_DIR}}/{{BINARY_NAME}} {{CMD_PATH}}

# Run the application
run: build
    {{BUILD_DIR}}/{{BINARY_NAME}}

# Test with debug output and custom parameters
# Usage: just test-debug TIMEOUT=30s PKG=./internal/logic TESTS=TestParse
test-debug TIMEOUT="30s" GOFLAGS="" PKG="./..." TESTS="":
    #!/usr/bin/env bash
    ARGS="-timeout {{TIMEOUT}}"
    [ -n "{{GOFLAGS}}" ] && ARGS="$ARGS {{GOFLAGS}}"
    ARGS="$ARGS {{PKG}}"
    [ -n "{{TESTS}}" ] && ARGS="$ARGS -run {{TESTS}}"
    go test $ARGS

# Regular test
test:
    gotestsum

# Test with verbose output
test-verbose:
    go test -v ./...

# Test with coverage
test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
    rm -rf {{BUILD_DIR}}
    rm -f coverage.out coverage.html

# Download dependencies
deps:
    go mod download
    go mod tidy

# Tidy go.mod and go.sum
tidy:
    go mod tidy

# Format code
fmt:
    go fmt ./...

# Run code generation
generate: # mocks-generate
    go generate ./...

# Generate mocks using mockery
mocks-generate:
    mockery

# Lint code
lint:
    golangci-lint run

# Container commands

# Setup buildx builder (run once)
setup-buildx:
	docker buildx create --name multiarch --use || docker buildx use multiarch
	docker buildx inspect --bootstrap

# Build multi-arch Docker image (arm64 and amd64)
# Usage: just container IMAGE="preoomkiller-controller" TAG="latest"
container IMAGE="preoomkiller-controller" TAG="latest":
	docker buildx build --platform linux/arm64,linux/amd64 \
		-t {{IMAGE}}:{{TAG}} \
		--file Dockerfile \
		.

# Install tools
install-tools:
    brew install mockery
    brew install golangci-lint

fc: generate tidy build test fmt lint
