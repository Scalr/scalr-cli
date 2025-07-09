# Get version from git
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || git describe --tags --abbrev=0 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS = -s -w -X main.versionCLI=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildDate=$(BUILD_DATE)

# Default target
.PHONY: build
build:
	@echo "Building scalr-cli $(VERSION)"
	@echo "Git commit: $(GIT_COMMIT)"
	@echo "Build date: $(BUILD_DATE)"
	go build -ldflags "$(LDFLAGS)" -o scalr .

.PHONY: install
install: build
	mv scalr /usr/local/bin/

.PHONY: clean
clean:
	rm -f scalr scalr-test
	rm -rf bin/

.PHONY: test
test:
	go test ./...

.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Build date: $(BUILD_DATE)"

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build    - Build the binary with version information"
	@echo "  install  - Build and install to /usr/local/bin"
	@echo "  clean    - Clean build artifacts"
	@echo "  test     - Run tests"
	@echo "  version  - Show version information"
	@echo "  help     - Show this help" 