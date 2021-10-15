GONAME=mw

SHELL := /bin/bash
RELEASE_DIR ?= ./_release
TARGETS ?= darwin/amd64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le windows/amd64

PACKAGE := gitlab.wikimedia.org/releng/cli
VERSION := latest
SEMVER := $(subst v,,$(VERSION))

GO_LIST_GOFILES := '{{range .GoFiles}}{{printf "%s/%s\n" $$.Dir .}}{{end}}{{range .XTestGoFiles}}{{printf "%s/%s\n" $$.Dir .}}{{end}}'
GO_PACKAGES := $(shell go list ./...)

include .bingo/Variables.mk

.PHONY: build
build: $(GOVVV) generate
	@echo "Building $(GOFILES) to ./bin"
	go build -v -ldflags "$(shell $(GOVVV) -flags -version $(SEMVER))" -o bin/mw ./

.PHONY: release
release: $(GOX) $(GOVVV) generate
	$(GOX) -output="$(RELEASE_DIR)/$(SEMVER)/mw_$(VERSION)_{{.OS}}_{{.Arch}}" -osarch='$(TARGETS)' -ldflags '$(shell $(GOVVV) -flags -version $(SEMVER))' $(GO_PACKAGES)
	cp LICENSE "$(RELEASE_DIR)"
	rm -f ./_release/latest/*.sha256
	for f in "$(RELEASE_DIR)"/$(SEMVER)/mw_*; do \
		shasum -a 256 "$${f}" | awk '{print $$1}' > "$${f}.sha256"; \
	done

.PHONY: generate
generate: generate-staticfiles
	go generate $(GO_PACKAGES)

.PHONY: generate-staticfiles
generate-staticfiles: $(STATICFILES)
	rm -f internal/mwdd/files/files.go || true
	$(STATICFILES) -o internal/mwdd/files/files.go static/mwdd/
	echo "//lint:file-ignore ST1005 It's generated code"|cat - internal/mwdd/files/files.go > internal/mwdd/files/files.go.tmp && mv internal/mwdd/files/files.go.tmp internal/mwdd/files/files.go
	echo "//Package files ..."|cat - internal/mwdd/files/files.go > internal/mwdd/files/files.go.tmp && mv internal/mwdd/files/files.go.tmp internal/mwdd/files/files.go

.PHONY: clean
clean:
	go clean $(GO_PACKAGES)
	rm -rf bin || true
	rm -rf _release || true

.PHONY: test
test: $(GOVVV) generate
	go test -covermode=count -coverprofile "coverage.txt" -ldflags "$(shell $(GOVVV) -flags)" $(GO_PACKAGES)

.PHONY: lint
lint: $(GOLINT) generate
	@echo > .lint-gofmt.diff
	go list -f $(GO_LIST_GOFILES) $(GO_PACKAGES) | while read f; do \
		gofmt -e -d "$${f}" >> .lint-gofmt.diff; \
	done
	@test -z "$(grep '[^[:blank:]]' .lint-gofmt.diff)" || (echo "gofmt found errors:"; cat .lint-gofmt.diff; exit 1)
	@$(GOLINT) -set_exit_status $(GO_PACKAGES)

.PHONY: generate
vet: generate
	go vet -composites=false $(GO_PACKAGES)

.PHONY: staticcheck
staticcheck: $(STATICCHECK) generate
	$(STATICCHECK) -version
	$(STATICCHECK) -- ./...
