BINARY_NAME ?= mw
GOEXE := $(shell go env GOEXE)

SHELL := /bin/bash
RELEASE_DIR ?= ./_release
TARGETS ?= darwin/amd64 darwin/arm64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le windows/amd64

PACKAGE := gitlab.wikimedia.org/repos/releng/cli
VERSION := latest
SEMVER := $(subst v,,$(VERSION))

GO_LIST_GOFILES := '{{range .GoFiles}}{{printf "%s/%s\n" $$.Dir .}}{{end}}{{range .XTestGoFiles}}{{printf "%s/%s\n" $$.Dir .}}{{end}}'
GO_PACKAGES := gitlab.wikimedia.org/repos/releng/cli

include .bingo/Variables.mk

.PHONY: build
build: $(GOVVV) generate
	@echo "Building $(GOFILES) to ./bin"
	go build -v -ldflags "$(shell $(GOVVV) -flags -version $(SEMVER))" -o bin/$(BINARY_NAME)$(GOEXE) ./

# Build first; stdout contains only Bash commands for eval or another terminal.
.PHONY: dev
dev:
	@$(MAKE) --no-print-directory build >&2 || { printf 'false\n'; exit 1; }
	@dev_bin="$$(pwd -P)/bin"; dev_path=$$dev_bin; remaining=$$PATH; \
	while :; do \
		entry=$${remaining%%:*}; \
		if [[ $$entry != "$$dev_bin" ]]; then dev_path+=:$$entry; fi; \
		[[ $$remaining == *:* ]] || break; \
		remaining=$${remaining#*:}; \
	done; \
	printf 'export PATH=%q\nhash -r\n' "$$dev_path"

.PHONY: release
release: $(GOX) $(GOVVV) generate
	$(GOX) -output="$(RELEASE_DIR)/$(SEMVER)/$(BINARY_NAME)_$(VERSION)_{{.OS}}_{{.Arch}}" -osarch='$(TARGETS)' -ldflags '$(shell $(GOVVV) -flags -version $(SEMVER))' $(GO_PACKAGES)
	cp LICENSE "$(RELEASE_DIR)"
	rm -f ./_release/latest/*.sha256
	for f in "$(RELEASE_DIR)"/$(SEMVER)/$(BINARY_NAME)_*; do \
		shasum -a 256 "$${f}" | awk '{print $$1}' > "$${f}.sha256"; \
	done

.PHONY: generate
generate:
	go run tools/code-gen/main.go
	@cd ./mount/dev/ && find . -type f -not -name '*_test.go' | LC_ALL=C sort > files.txt
	go generate $(GO_PACKAGES)

.PHONY: clean
clean:
	go clean $(GO_PACKAGES)
	rm -rf bin || true
	rm -rf _release || true

.PHONY: test
test: $(GOVVV) generate
	go test -ldflags "$(shell $(GOVVV) -flags)" $(GO_PACKAGES)/...
	@echo '<testsuites></testsuites>' > junit.xml

.PHONY: recipe-validate
recipe-validate:
	go test ./internal/mwdd/recipe -run TestParseRecipeExamples

.PHONY: lint
lint: $(GOLANGCI_LINT) generate
	@$(GOLANGCI_LINT) run --timeout 2m --concurrency 1

.PHONY: fix
fix: $(GOLANGCI_LINT) generate
	@$(GOLANGCI_LINT) run --fix --concurrency 1

.PHONY: generate
vet: generate
	go vet -composites=false $(GO_PACKAGES)

.PHONY: duplicates
duplicates: $(DUPL)
	find ./ -name '*.go' -not -path './vendor/*' -not -name '*.gen.go' -not -name '*_test.go' | $(DUPL) -files

.PHONY: git-state
git-state: $(GOX) $(GOVVV)
	$(MAKE) TARGETS=linux/amd64 release
	git diff --quiet -- . ':(exclude).bingo/go.mod' || (git --no-pager diff -- . ':(exclude).bingo/go.mod' && false)

.PHONY: linti
linti:
	go run ./tools/lint/main.go

.PHONY: upgrade
upgrade: $(GO_MOD_UPGRADE)
	$(GO_MOD_UPGRADE)

# Credentials stay in the environment, not command arguments or CI logs.
export DOCS_UPDATE_USER ?= $(user)
export DOCS_UPDATE_PASS ?= $(password)
export DOCS_VERSION ?= $(VERSION)

.PHONY: docs docs-check docs-pull docs-publish docs-import docs-accept
docs:
	go run ./tools/docs-gen

docs-check: docs
	go run ./tools/docs-gen -mode check

docs-pull: docs
	go run ./tools/docs-gen -mode pull

docs-publish: docs
	go run ./tools/docs-gen -mode publish

docs-import:
	go run ./tools/docs-gen -mode import -page "$(page)" -source "$(file)"

docs-accept:
	go run ./tools/docs-gen -mode accept -page "$(page)"
