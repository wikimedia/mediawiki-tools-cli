GOPATH=$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GONAME=mw

SHELL := /bin/bash
RELEASE_DIR ?= ./_release
TARGETS ?= darwin/amd64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le windows/amd64

PACKAGE := gerrit.wikimedia.org/r/mediawiki/tools/cli
VERSION := $(shell cat VERSION)

GO_LIST_GOFILES := '{{range .GoFiles}}{{printf "%s/%s\n" $$.Dir .}}{{end}}{{range .XTestGoFiles}}{{printf "%s/%s\n" $$.Dir .}}{{end}}'
GO_PACKAGES := $(shell go list ./...)

all: get-dev generate get build

get:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get .

get-dev:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get github.com/ahmetb/govvv@v0.3.0
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get bou.ke/staticfiles@v0.0.0-20210106104248-dd04075d4104
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get github.com/mitchellh/gox@v1.0.1

build:
	@echo "Building $(GOFILES) to ./bin"
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -v -ldflags "$(shell ./bin/govvv -flags)" -o bin/mw ./

release:
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) ./bin/gox -output="$(RELEASE_DIR)/mw_$(VERSION)_{{.OS}}_{{.Arch}}" -osarch='$(TARGETS)' -ldflags '$(shell ./bin/govvv -flags)' $(GO_PACKAGES)
	cp LICENSE "$(RELEASE_DIR)"
	for f in "$(RELEASE_DIR)"/mw_*; do \
		shasum -a 256 "$${f}" | awk '{print $$1}' > "$${f}.sha256"; \
	done

install: all
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install -v $(GO_PACKAGES)

generate: internal/mwdd/files/files.go
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go generate $(GO_PACKAGES)

internal/mwdd/files/files.go: static/mwdd/*
	rm -f internal/mwdd/files/files.go || true
	./bin/staticfiles -o internal/mwdd/files/files.go static/mwdd/

clean:
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go clean $(GO_PACKAGES)
	rm -rf bin || true
	rm -rf _release || true
	rm internal/mwdd/files/files.go || true

test: get-dev generate unit lint

unit:
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go test -cover -ldflags "$(shell ./bin/govvv -flags)" $(GO_PACKAGES)

lint:
	@echo > .lint-gofmt.diff
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go list -f $(GO_LIST_GOFILES) $(GO_PACKAGES) | while read f; do \
		gofmt -e -d "$${f}" >> .lint-gofmt.diff; \
	done
	@test -z "$(grep '[^[:blank:]]' .lint-gofmt.diff)" || (echo "gofmt found errors:"; cat .lint-gofmt.diff; exit 1)
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) golint -set_exit_status $(GO_PACKAGES)
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go vet -composites=false $(GO_PACKAGES)


.PHONY: install release
