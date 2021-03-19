# Auto generated binary variables helper managed by https://github.com/bwplotka/bingo v0.2.3. DO NOT EDIT.
# All tools are designed to be build inside $GOBIN.
GOPATH ?= $(shell go env GOPATH)
GOBIN  ?= $(firstword $(subst :, ,${GOPATH}))/bin
GO     ?= $(shell which go)

# Bellow generated variables ensure that every time a tool under each variable is invoked, the correct version
# will be used; reinstalling only if needed.
# For example for buf variable:
#
# In your main Makefile (for non array binaries):
#
#include .bingo/Variables.mk # Assuming -dir was set to .bingo .
#
#command: $(BUF)
#	@echo "Running buf"
#	@$(BUF) <flags/args..>
#
BUF := $(GOBIN)/buf-v0.20.5
$(BUF): .bingo/buf.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/buf-v0.20.5"
	@cd .bingo && $(GO) build -mod=mod -modfile=buf.mod -o=$(GOBIN)/buf-v0.20.5 "github.com/bufbuild/buf/cmd/buf"

GOMPLATE := $(GOBIN)/gomplate-v3.8.0
$(GOMPLATE): .bingo/gomplate.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/gomplate-v3.8.0"
	@cd .bingo && $(GO) build -mod=mod -modfile=gomplate.mod -o=$(GOBIN)/gomplate-v3.8.0 "github.com/hairyhenderson/gomplate/v3/cmd/gomplate"

GOVVV := $(GOBIN)/govvv-v0.3.0
$(GOVVV): .bingo/govvv.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/govvv-v0.3.0"
	@cd .bingo && $(GO) build -mod=mod -modfile=govvv.mod -o=$(GOBIN)/govvv-v0.3.0 "github.com/ahmetb/govvv"

GOX := $(GOBIN)/gox-v1.0.1
$(GOX): .bingo/gox.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/gox-v1.0.1"
	@cd .bingo && $(GO) build -mod=mod -modfile=gox.mod -o=$(GOBIN)/gox-v1.0.1 "github.com/mitchellh/gox"

PROTOC_GEN_BUF_CHECK_BREAKING := $(GOBIN)/protoc-gen-buf-check-breaking-v0.20.5
$(PROTOC_GEN_BUF_CHECK_BREAKING): .bingo/protoc-gen-buf-check-breaking.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/protoc-gen-buf-check-breaking-v0.20.5"
	@cd .bingo && $(GO) build -mod=mod -modfile=protoc-gen-buf-check-breaking.mod -o=$(GOBIN)/protoc-gen-buf-check-breaking-v0.20.5 "github.com/bufbuild/buf/cmd/protoc-gen-buf-check-breaking"

PROTOC_GEN_BUF_CHECK_LINT := $(GOBIN)/protoc-gen-buf-check-lint-v0.20.5
$(PROTOC_GEN_BUF_CHECK_LINT): .bingo/protoc-gen-buf-check-lint.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/protoc-gen-buf-check-lint-v0.20.5"
	@cd .bingo && $(GO) build -mod=mod -modfile=protoc-gen-buf-check-lint.mod -o=$(GOBIN)/protoc-gen-buf-check-lint-v0.20.5 "github.com/bufbuild/buf/cmd/protoc-gen-buf-check-lint"

