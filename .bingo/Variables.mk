# Auto generated binary variables helper managed by https://github.com/bwplotka/bingo v0.6. DO NOT EDIT.
# All tools are designed to be build inside $GOBIN.
BINGO_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
GOPATH ?= $(shell go env GOPATH)
GOBIN  ?= $(firstword $(subst :, ,${GOPATH}))/bin
GO     ?= $(shell which go)

# Below generated variables ensure that every time a tool under each variable is invoked, the correct version
# will be used; reinstalling only if needed.
# For example for bingo variable:
#
# In your main Makefile (for non array binaries):
#
#include .bingo/Variables.mk # Assuming -dir was set to .bingo .
#
#command: $(BINGO)
#	@echo "Running bingo"
#	@$(BINGO) <flags/args..>
#
BINGO := $(GOBIN)/bingo-v0.5.1
$(BINGO): $(BINGO_DIR)/bingo.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/bingo-v0.5.1"
	@cd $(BINGO_DIR) && $(GO) build -mod=mod -modfile=bingo.mod -o=$(GOBIN)/bingo-v0.5.1 "github.com/bwplotka/bingo"

GOLANGCI_LINT := $(GOBIN)/golangci-lint-v1.46.2
$(GOLANGCI_LINT): $(BINGO_DIR)/golangci-lint.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/golangci-lint-v1.46.2"
	@cd $(BINGO_DIR) && $(GO) build -mod=mod -modfile=golangci-lint.mod -o=$(GOBIN)/golangci-lint-v1.46.2 "github.com/golangci/golangci-lint/cmd/golangci-lint"

GOLINT := $(GOBIN)/golint-v0.0.0-20210508222113-6edffad5e616
$(GOLINT): $(BINGO_DIR)/golint.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/golint-v0.0.0-20210508222113-6edffad5e616"
	@cd $(BINGO_DIR) && $(GO) build -mod=mod -modfile=golint.mod -o=$(GOBIN)/golint-v0.0.0-20210508222113-6edffad5e616 "golang.org/x/lint/golint"

GOVVV := $(GOBIN)/govvv-v0.3.0
$(GOVVV): $(BINGO_DIR)/govvv.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/govvv-v0.3.0"
	@cd $(BINGO_DIR) && $(GO) build -mod=mod -modfile=govvv.mod -o=$(GOBIN)/govvv-v0.3.0 "github.com/ahmetb/govvv"

GOX := $(GOBIN)/gox-v1.0.1
$(GOX): $(BINGO_DIR)/gox.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/gox-v1.0.1"
	@cd $(BINGO_DIR) && $(GO) build -mod=mod -modfile=gox.mod -o=$(GOBIN)/gox-v1.0.1 "github.com/mitchellh/gox"

STATICCHECK := $(GOBIN)/staticcheck-v0.2.1
$(STATICCHECK): $(BINGO_DIR)/staticcheck.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/staticcheck-v0.2.1"
	@cd $(BINGO_DIR) && $(GO) build -mod=mod -modfile=staticcheck.mod -o=$(GOBIN)/staticcheck-v0.2.1 "honnef.co/go/tools/cmd/staticcheck"

