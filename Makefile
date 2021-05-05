# Go parameters
GOCMD?=go
GO_VERSION=$(shell go list -m -f "{{.GoVersion}}")
PACKAGE_BASE=github.com/buildpacks/imgutil
WINDOWS_COMPILATION_IMAGE?=golang:1.15-windowsservercore-1809
DOCKER_CMD?=make test

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
	$(GOCMD) test -parallel=1 -count=1 -v ./...

# Ensure workdir is clean and build image from .git
docker-build-source-image-windows:
	$(if $(shell git status --short), @echo Uncommitted changes. Refusing to run. && exit 1)
	docker build .git -f tools/Dockerfile.windows --tag imgutil-img --build-arg image_tag=$(WINDOWS_COMPILATION_IMAGE) --cache-from=lifecycle-img --isolation=process --compress

docker-run-windows: docker-build-source-image-windows
docker-run-windows:
	@echo "> Running '$(DOCKER_CMD)' in docker windows..."
	docker run -v gopathcache:c:/gopath -v '\\.\pipe\docker_engine:\\.\pipe\docker_engine' --isolation=process --interactive --tty --rm imgutil-img $(DOCKER_CMD)

