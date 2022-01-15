GONAME=mw

SHELL := /bin/bash
RELEASE_DIR ?= ./_release
TARGETS ?= darwin/amd64 darwin/arm64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le windows/amd64

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
generate:
	@cd ./internal/mwdd/files/embed/ && find . -type f | LC_ALL=C sort > files.txt
	go generate $(GO_PACKAGES)

.PHONY: clean
clean:
	go clean $(GO_PACKAGES)
	rm -rf bin || true
	rm -rf _release || true

.PHONY: test
test: $(GOVVV) generate
	go test -covermode=count -coverprofile "coverage.txt" -ldflags "$(shell $(GOVVV) -flags)" $(GO_PACKAGES)

.PHONY: lint
lint: $(GOLANGCI_LINT) generate
	@$(GOLANGCI_LINT) run -E revive -E gci -E gofmt -E gofumpt -E goimports -E whitespace

.PHONY: fix
fix: $(GOLANGCI_LINT) generate
	@$(GOLANGCI_LINT) run --fix -E revive -E gci -E gofmt -E gofumpt -E goimports -E whitespace

.PHONY: generate
vet: generate
	go vet -composites=false $(GO_PACKAGES)

.PHONY: staticcheck
staticcheck: $(STATICCHECK) generate
	$(STATICCHECK) -version
	$(STATICCHECK) -- ./...

.PHONY: duplicates
duplicates: $(GOLANGCI_LINT) generate
	@$(GOLANGCI_LINT) run -E dupl

.PHONY: git-state
git-state: $(GOX) $(GOVVV) release
	git diff --quiet || (git --no-pager diff && false)

.PHONY: docs
docs: build
	rm -rf ./_docs/*
	MWCLI_DOC_GEN="./_docs" ./bin/mw
	for path in ./_docs/*.md; do \
		echo "Converting "$${path}" to wikitext"; \
		pandoc --from markdown --to mediawiki --output $${path::-3}.wiki "$${path}"; \
	done

.PHONY: docs-publish
docs-publish: docs
	for path in ./_docs/*.wiki; do \
		file=$${path/.\/_docs\//}; \
		fileNoExt=$${file::-5}; \
		echo $${fileNoExt}; \
		echo $${path}; \
		printf "__NOTOC__" | cat $${path} - | ./bin/mw wiki --wiki https://www.mediawiki.org/w/api.php --user ${user} --password ${password} page --title Cli/ref/$${fileNoExt} put --summary "Pushing auto generated docs for mwcli from cobra" --minor; \
	done