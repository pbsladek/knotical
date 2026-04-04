.PHONY: help fmt test test-cover build run verify clean

GO ?= go
GOFMT ?= gofmt
PKGS ?= ./...
CMD ?=
DIST_DIR ?= dist
BINARY ?= $(DIST_DIR)/knotical

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make fmt         Format Go source files' \
		'  make test        Run the test suite' \
		'  make test-cover  Run tests with coverage' \
		'  make build       Build knotical into dist/' \
		'  make run CMD=... Run knotical with a prompt or flags' \
		'  make verify      Format, test, and build' \
		'  make clean       Remove generated build artifacts'

fmt:
	$(GOFMT) -w ./cmd ./internal

test:
	$(GO) test $(PKGS)

test-cover:
	$(GO) test -cover $(PKGS)

build:
	mkdir -p $(DIST_DIR)
	$(GO) build -o $(BINARY) ./cmd/knotical

run:
	$(GO) run ./cmd/knotical $(CMD)

verify: fmt test build

clean:
	rm -rf $(DIST_DIR)
