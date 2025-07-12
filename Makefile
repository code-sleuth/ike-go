PKG := github.com/code-sleuth/ike-go
SERVICE := ike-go
VERSION := 0.0.1-alpha1
RELEASE_CYCLE := alpha
GO_FILES=$(shell find . -type f -name '*.go')
LINT_TOOL=$(shell go env GOPATH)/bin/golangci-lint
CPU_INFO := $(shell \
	if [ "$(shell uname)" = "Darwin" ]; then \
		sysctl -n machdep.cpu.brand_string; \
	else \
		cat /proc/cpuinfo | grep "model name" | head -n 1 | cut -d ":" -f 2 | sed 's/^[ \t]*//'; \
	fi)
_GIT_DESCRIPTION_OR_TAG := $(subst v${VERSION}-,,$(shell git describe --tag --dirty --always --abbrev=9))
GITHASH := $(subst v${VERSION},$(shell git rev-parse --short=9 HEAD),${_GIT_DESCRIPTION_OR_TAG})
ifeq (${GITHASH},dirty)
GITHASH := $(shell git rev-parse --short=9 HEAD)
endif

ifeq "$(shell uname -p)" "arm"
	BUILD_ARCH=arm64
endif
ifeq "$(shell uname -p)" "x86_64"
	BUILD_ARCH=amd64
endif
ifeq "$(shell uname -s)" "Darwin"
	BUILD_HOST=darwin
endif
ifeq "$(shell uname -s)" "Linux"
	BUILD_HOST=linux
endif

.PHONY: build build-linux build-mac build-windows migrate run clean deps fmt lint test version help

# Default target
help:
	@echo "Available targets:"
	@echo "  build        - Build the ike-go binary"
	@echo "  build-linux  - Build the ike-go binary for Linux"
	@echo "  build-mac    - Build the ike-go binary for macOS"
	@echo "  build-windows - Build the ike-go binary for Windows"
	@echo "  migrate      - Run database migrations"
	@echo "  run          - Build and run the CLI"
	@echo "  clean        - Remove built binary"
	@echo "  deps         - Install/update dependencies"
	@echo "  fmt          - Format go files"
	@echo "  lint         - Lint files"
	@echo "  test         - Run all tests with coverage"
	@echo "  version      - Show version and build information"
	@echo "  help         - Show this help message"

version:
	@echo "Package     : ${PKG}"
	@echo "Version     : ${VERSION}"
	@echo "Git Hash    : ${GITHASH}"
	@printf "GOOS        : "; go env GOOS
	@printf "GOARCH      : "; go env GOARCH
	@echo   "CPU         : ${CPU_INFO}"
	@echo

# Build the binary for current platform
build: version fmt lint
	@echo "\n==> Building ike-go using $(shell go version) for current platform...\n"
	go build -a -tags ike-go -o bin/ike-go .

# Build the binary for linux
build-linux: version fmt lint
	@echo "\n==> Building ike-go using $(shell go version) for target linux/$(BUILD_ARCH)...\n"
	CGO_ENABLED=0 GOOS=linux GOARCH=$(BUILD_ARCH) go build -a -tags ike-go -o bin/ike-go .

# Build the binary for mac
build-mac: version fmt lint
	@echo "\n==> Building ike-go using $(shell go version) for target darwin/$(BUILD_ARCH)...\n"
	CGO_ENABLED=0 GOOS=darwin GOARCH=$(BUILD_ARCH) go build -a -tags ike-go -o bin/ike-go .

# Build the binary for windows
build-windows: version fmt lint
	@echo "\n==> Building ike-go using $(shell go version) for target windows/$(BUILD_ARCH)...\n"
	CGO_ENABLED=0 GOOS=windows GOARCH=$(BUILD_ARCH) go build -a -tags ike-go -o bin/ike-go.exe .

# Run database migrations
migrate: build
	./bin/ike-go migrate

# Build and run the CLI (shows help by default)
run:
	./bin/ike-go

# Clean up built binary
clean:
	rm -rf ./bin/

# Install dependencies
deps:
	go mod tidy
	go mod download

fmt:
	@echo "==> Running gofmt, goimports & golines"
	@gofmt -w -s $(GO_FILES)
	@goimports -w $(GO_FILES)
	@golines -m 120 -w $(GO_FILES)
	@echo "🎉 go fmt check passed 🏁"

lint:
	@echo "==> Running lint"
	$(LINT_TOOL) --version
	$(LINT_TOOL) run --allow-parallel-runners --config=.golangci.yaml ./...
	@echo "🎉 Lint check passed 🏁"

test:
	@echo "==> Run tests"
	@go test -p 1 -coverprofile=coverage.out `go list ./...`

lint-tool:
	@echo "Installing the Lint tool"
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.2
	@echo "🎉 Lint Tool installed 🏁"