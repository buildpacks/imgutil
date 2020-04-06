# Go parameters
GOCMD?=go
GOTEST=$(GOCMD) test -mod=vendor
PACKAGE_BASE=github.com/buildpacks/imgutil
PACKAGES:=$(shell $(GOCMD) list -mod=vendor ./... | grep -v /testdata/)
SRC:=$(shell find . -type f -name '*.go' -not -path "*/vendor/*")

all: test

install-goimports:
	@echo "> Installing goimports..."
	cd tools; $(GOCMD) install -mod=vendor golang.org/x/tools/cmd/goimports

format: install-goimports
	@echo "> Formating code..."
	@goimports -l -w -local ${PACKAGE_BASE} ${SRC}

install-golangci-lint:
	@echo "> Installing golangci-lint..."
	cd tools; $(GOCMD) install -mod=vendor github.com/golangci/golangci-lint/cmd/golangci-lint

lint: install-golangci-lint
	@echo "> Linting code..."
	@golangci-lint run -c golangci.yaml

test: format lint
	$(GOTEST) -parallel=1 -count=1 -v ./...
