GOPATH=$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GONAME=mw

SHELL := /bin/bash
RELEASE_DIR ?= ./_release
TARGETS ?= darwin/amd64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le windows/amd64

PACKAGE := gerrit.wikimedia.org/r/mediawiki/tools/cli
VERSION := latest
SEMVER := $(subst v,,$(VERSION))

GO_LIST_GOFILES := '{{range .GoFiles}}{{printf "%s/%s\n" $$.Dir .}}{{end}}{{range .XTestGoFiles}}{{printf "%s/%s\n" $$.Dir .}}{{end}}'
GO_PACKAGES := $(shell go list ./...)

all: get-dev generate get build

get:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get .

# Go 1.16 onwards includes a change that makes it possible to install a binary without affecting go.mod.
# https://stackoverflow.com/a/65734439/4746236
# TODO use this when we are running on Go 1.16 or later
get-dev: get-dev-govvv get-dev-gox get-dev-staticfiles get-dev-lint get-dev-staticcheck
get-dev-govvv:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get github.com/ahmetb/govvv@v0.3.0
get-dev-gox:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get github.com/mitchellh/gox@v1.0.1
get-dev-staticfiles:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get bou.ke/staticfiles@v0.0.0-20210106104248-dd04075d4104
get-dev-lint:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get golang.org/x/lint/golint
get-dev-staticcheck:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get honnef.co/go/tools/cmd/staticcheck

build: get-dev-govvv
	@echo "Building $(GOFILES) to ./bin"
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -v -ldflags "$(shell ./bin/govvv -flags -version $(SEMVER))" -o bin/mw ./

release: get-dev-govvv
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) ./bin/gox -output="$(RELEASE_DIR)/$(SEMVER)/mw_$(VERSION)_{{.OS}}_{{.Arch}}" -osarch='$(TARGETS)' -ldflags '$(shell ./bin/govvv -flags -version $(SEMVER))' $(GO_PACKAGES)
	cp LICENSE "$(RELEASE_DIR)"
	for f in "$(RELEASE_DIR)"/$(SEMVER)/mw_*; do \
		shasum -a 256 "$${f}" | awk '{print $$1}' > "$${f}.sha256"; \
	done

install: all
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install -v $(GO_PACKAGES)

generate: generate-staticfiles
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go generate $(GO_PACKAGES)

generate-staticfiles: get-dev-staticfiles
	rm -f internal/mwdd/files/files.go || true
	./bin/staticfiles -o internal/mwdd/files/files.go static/mwdd/
	echo "//lint:file-ignore ST1005 It's generated code"|cat - internal/mwdd/files/files.go > internal/mwdd/files/files.go.tmp && mv internal/mwdd/files/files.go.tmp internal/mwdd/files/files.go
	echo "//Package files ..."|cat - internal/mwdd/files/files.go > internal/mwdd/files/files.go.tmp && mv internal/mwdd/files/files.go.tmp internal/mwdd/files/files.go

clean:
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go clean $(GO_PACKAGES)
	rm -rf bin || true
	rm -rf _release || true
	rm internal/mwdd/files/files.go || true

test: get-dev-govvv generate
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go test -covermode=count -coverprofile "coverage.txt" -ldflags "$(shell ./bin/govvv -flags)" $(GO_PACKAGES)

lint: get-dev-lint generate
	@echo > .lint-gofmt.diff
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go list -f $(GO_LIST_GOFILES) $(GO_PACKAGES) | while read f; do \
		gofmt -e -d "$${f}" >> .lint-gofmt.diff; \
	done
	@test -z "$(grep '[^[:blank:]]' .lint-gofmt.diff)" || (echo "gofmt found errors:"; cat .lint-gofmt.diff; exit 1)
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) ./bin/golint -set_exit_status $(GO_PACKAGES)

vet: generate
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go vet -composites=false $(GO_PACKAGES)

staticcheck: get-dev-staticcheck generate
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) ./bin/staticcheck -version
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) ./bin/staticcheck -- ./...

.PHONY: all get build release install generate generate-staticfiles clean test lint vet staticcheck
