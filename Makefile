# Go parameters
GOCMD?=go
GO_VERSION=$(shell go list -m -f "{{.GoVersion}}")
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
	cd tools && $(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

lint: install-golangci-lint
	@echo "> Linting code..."
	@golangci-lint run -c golangci.yaml

tools/bcdhive_generator/.build: $(wildcard tools/bcdhive_generator/*)
ifneq ($(OS),Windows_NT)
	@echo "> Building bcdhive-generator in Docker using current golang version"
	docker build tools/bcdhive_generator --tag bcdhive-generator --build-arg go_version=$(GO_VERSION)

	@touch tools/bcdhive_generator/.build
else
	@echo "> Cannot generate on Windows"
endif

layer/bcdhive_generated.go: layer/windows_baselayer.go tools/bcdhive_generator/.build
ifneq ($(OS),Windows_NT)
	$(GOCMD) generate ./...
else
	@echo "> Cannot generate on Windows"
endif

test: layer/bcdhive_generated.go format lint
	$(GOCMD) test -parallel=1 -count=1 -coverprofile=coverage.out -v ./...

codecov: test
	$(GOCMD) tool cover -html=coverage.out

tidy:
	$(GOCMD) mod tidy && cd tools && $(GOCMD) mod tidy -compat=1.17 && cd bcdhive_generator && $(GOCMD) mod tidy
