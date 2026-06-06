# Run all checks before committing
verify: fmt build lint test

# Format all Go source files
fmt:
    gofmt -w .

# Run all tests
test *args:
    go test ./... {{args}}

# Build the binary, stamping a dev version derived from git
build *args:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p dist
    version="0.0.0-dev.$(git rev-parse --short HEAD)"
    if [ -n "$(git status --porcelain)" ]; then
        version="${version}.dirty"
    fi
    env CGO_ENABLED=0 go build -trimpath \
        -ldflags "-X github.com/chinmina/kms-import/internal/buildinfo.Version=${version}" \
        -o dist/ ./cmd/kms-import/... {{args}}

# Run linter
lint:
    golangci-lint run ./...
