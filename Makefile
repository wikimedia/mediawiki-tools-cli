GONAME=mw

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
	go run tools/code-gen/main.go
	@cd ./mount/dev/ && find . -type f | LC_ALL=C sort > files.txt
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

.PHONY: docs
docs: build
	rm -rf ./_docs/*
	go run ./tools/docs-gen/main.go
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
		printf "__NOTOC__" | cat $${path} - | ./bin/mw --no-interaction wiki --wiki https://www.mediawiki.org/w/api.php --user ${user} --password ${password} page --title Cli/ref/$${fileNoExt} put --summary "Pushing auto generated docs for mwcli from cobra" --minor --bot; \
	done
