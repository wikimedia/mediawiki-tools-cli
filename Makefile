SHELL := /bin/bash
RELEASE_DIR ?= ./_release
TARGETS ?= darwin/amd64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le windows/amd64

PACKAGE := gerrit.wikimedia.org/r/mediawiki/tools/cli

GO_LIST_GOFILES := '{{range .GoFiles}}{{printf "%s/%s\n" $$.Dir .}}{{end}}{{range .XTestGoFiles}}{{printf "%s/%s\n" $$.Dir .}}{{end}}'
GO_PACKAGES := $(shell go list ./...)


GO_LDFLAGS := \
  -X $(PACKAGE)/meta.Version=$(shell cat VERSION) \
  -X $(PACKAGE)/meta.GitCommit=$(shell git rev-parse --short HEAD)

# go build/install commands
#
GO_BUILD := go build -v -ldflags "$(GO_LDFLAGS)" -o bin/mw
GO_INSTALL := go install -v -ldflags "$(GO_LDFLAGS)"

all: code mw-cli

mw-cli:
	$(GO_BUILD) ./cmd/cli

code:
	go generate $(GO_PACKAGES)

clean:
	go clean $(GO_PACKAGES)
	rm -f bin/mw || true
	rm -rf _releases || true

install: all
	$(GO_INSTALL) $(GO_PACKAGES)

release:
	gox -output="$(RELEASE_DIR)/{{.OS}}-{{.Arch}}/mw" -osarch='$(TARGETS)' -ldflags '$(GO_LDFLAGS)' $(GO_PACKAGES)
	cp LICENSE "$(RELEASE_DIR)"
	for f in "$(RELEASE_DIR)"/*/mw; do \
		shasum -a 256 "$${f}" | awk '{print $$1}' > "$${f}.sha256"; \
	done

lint:
	@echo > .lint-gofmt.diff
	@go list -f $(GO_LIST_GOFILES) $(GO_PACKAGES) | while read f; do \
		gofmt -e -d "$${f}" >> .lint-gofmt.diff; \
	done
	@test -z "$(grep '[^[:blank:]]' .lint-gofmt.diff)" || (echo "gofmt found errors:"; cat .lint-gofmt.diff; exit 1)
	golint -set_exit_status $(GO_PACKAGES)
	go vet -composites=false $(GO_PACKAGES)

unit:
	go test -cover -ldflags "$(GO_LDFLAGS)" $(GO_PACKAGES)

test: unit lint

internal/mwdd/files/files.go: static/mwdd/*
	staticfiles -o internal/mwdd/files/files.go static/mwdd/

.PHONY: install release
