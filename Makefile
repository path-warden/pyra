.PHONY: build build-all test lint clean demo-bundle

VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
BINARY := memphis

# The default binary is pure Go (no cgo) so it cross-compiles to every target
# below with plain `go build`. Code intelligence uses a pure-Go tree-sitter
# runtime specifically to preserve this. Keep CGO off on these targets.
export CGO_ENABLED ?= 0

# Code intelligence embeds only the grammars for the languages memphis has query
# sets for, via gotreesitter's grammar_subset build tags. This keeps the binary
# lean (a plain `go build` embeds all ~206 grammars, ~2x larger). Keep this list
# in sync with grammarLoaders in internal/codeintel/registry.go.
CODEINTEL_TAGS := grammar_subset \
	grammar_subset_go grammar_subset_python grammar_subset_javascript \
	grammar_subset_typescript grammar_subset_tsx grammar_subset_java \
	grammar_subset_rust
TAGS := $(CODEINTEL_TAGS)

# Build for current platform
build:
	go build -tags '$(TAGS)' $(LDFLAGS) -o $(BINARY) ./cmd/memphis

# Build for all platforms (proves the single self-contained, cgo-free binary)
build-all: clean
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags '$(TAGS)' $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/memphis
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags '$(TAGS)' $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/memphis
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -tags '$(TAGS)' $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 ./cmd/memphis
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -tags '$(TAGS)' $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/memphis
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags '$(TAGS)' $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./cmd/memphis

# Run tests
test:
	go test -v ./...

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/

# Run the demo
demo: build
	./$(BINARY) demo
