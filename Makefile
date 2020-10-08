# Go parameters
GOCMD?=go
PACKAGE_BASE=github.com/buildpacks/imgutil

all: test

install-goimports:
	@echo "> Installing goimports..."
	cd tools && $(GOCMD) install golang.org/x/tools/cmd/goimports

format: install-goimports
	@echo "> Formating code..."
	@goimports -l -w -local ${PACKAGE_BASE} .

install-golangci-lint:
	@echo "> Installing golangci-lint..."
	cd tools && $(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint

lint: install-golangci-lint
	@echo "> Linting code..."
	@golangci-lint run -c golangci.yaml

generate: build-bcdhive-generator
ifneq ($(OS),Windows_NT)
	$(GOCMD) generate ./...
else
	@echo "> Cannot generate on Windows"
endif

build-bcdhive-generator:
ifneq ($(OS),Windows_NT)
	@echo "> Building bcdhive-generator in Docker"
	docker build tools/bcdhive_generator --tag bcdhive-generator
else
	@echo "> Cannot generate on Windows"
endif

test: generate format lint
	$(GOCMD) test -parallel=1 -count=1 -v ./...
