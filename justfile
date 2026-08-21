# Run all checks before committing
verify: fmt build lint test test-system

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

# Alias to the product build recipe so callers can use composable names.
product-build: build

# Build the unshipped support executable used only by system tests.
support-build *args:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p dist
    env CGO_ENABLED=0 go build -trimpath \
        -o dist/ ./cmd/kms-support/... {{args}}

# Build all artifacts required by the containerized system-test suite.
system-artifact-build: build support-build

# Run the containerized system-test suite against a fresh Local KMS instance.
# Leaves Compose resources running for inspection; use system-cleanup to remove them.
system-run: system-artifact-build
    docker compose -f test/system/compose.yaml --project-name kms-import-system up --abort-on-container-exit --exit-code-from bats

# Remove containers, networks, and volumes created by system-run / test-system.
system-cleanup:
    docker compose -f test/system/compose.yaml --project-name kms-import-system down --volumes --remove-orphans

# Run the system-test suite and always clean up Compose resources afterwards.
test-system:
    #!/usr/bin/env bash
    set -euo pipefail
    trap 'just system-cleanup' EXIT
    just system-run
