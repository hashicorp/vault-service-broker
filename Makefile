# Metadata about this makefile and position
MKFILE_PATH := $(lastword $(MAKEFILE_LIST))
CURRENT_DIR := $(patsubst %/,%,$(dir $(realpath $(MKFILE_PATH))))

# Ensure GOPATH
GOPATH ?= $(HOME)/go

# List all our actual files, excluding vendor
GOFILES ?= $(shell go list $(TEST) | grep -v /vendor/)

# Tags specific for building
GOTAGS ?=

# Number of procs to use
GOMAXPROCS ?= 4

PROJECT := $(CURRENT_DIR:$(GOPATH)/src/%=%)
OWNER := $(notdir $(patsubst %/,%,$(dir $(PROJECT))))
NAME := $(notdir $(PROJECT))
VERSION := $(shell awk -F\" '/Version/ { print $$2; exit }' "version.go")
EXTERNAL_TOOLS = \
	github.com/golang/dep/cmd/dep

# Current system information
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# List of ldflags
LD_FLAGS ?= -s -w

# List of tests to run
TEST ?= ./...

# bootstrap installs the necessary go tools for development or build.
bootstrap:
	@echo "==> Bootstrapping ${PROJECT}"
	@for t in ${EXTERNAL_TOOLS}; do \
		echo "--> Installing $$t" ; \
		go get -u "$$t"; \
	done
.PHONY: bootstrap

# build builds the binary into pkg/
build:
	@echo "==> Building ${NAME} for ${GOOS}/${GOARCH}"
	@env \
		-i \
		PATH="${PATH}" \
		CGO_ENABLED="0" \
		GOOS="${GOOS}" \
		GOARCH="${GOARCH}" \
		GOPATH="${GOPATH}" \
		go build -a -o "pkg/${GOOS}_${GOARCH}/${NAME}" -ldflags "${LD_FLAGS}"
.PHONY: build

# deps updates all dependencies for this project.
deps:
	@echo "==> Updating deps for ${PROJECT}"
	@dep ensure -update
	@dep prune
.PHONY: deps

# dev builds and installs the project locally.
dev:
	@echo "==> Installing ${NAME} for ${GOOS}/${GOARCH}"
	@env \
		-i \
		PATH="${PATH}" \
		CGO_ENABLED="0" \
		GOOS="${GOOS}" \
		GOARCH="${GOARCH}" \
		GOPATH="${GOPATH}" \
		go install -ldflags "${LD_FLAGS}"
.PHONY: dev

# docker builds the docker container.
docker:
	@echo "==> Building docker container for ${PROJECT}"
	@docker build \
		--rm \
		--force-rm \
		--no-cache \
		--squash \
		--compress \
		--file="docker/Dockerfile" \
		--build-arg="LD_FLAGS=${LD_FLAGS}" \
		--tag="${OWNER}/${NAME}" \
		--tag="${OWNER}/${NAME}:${VERSION}" \
		"${CURRENT_DIR}"
.PHONY: docker

# docker-push pushes the images to the registry
docker-push:
	@echo "==> Pushing ${PROJECT} to Docker registry"
	@docker push "${OWNER}/${NAME}:latest"
	@docker push "${OWNER}/${NAME}:${VERSION}"

# linux builds the linux binary
linux:
	@env \
		GOOS="linux" \
		GOARCH="amd64" \
		$(MAKE) -f "${MKFILE_PATH}" build
.PHONY: linux

# test runs the test suite.
test:
	@echo "==> Testing ${NAME}"
	@go test -v -timeout=30s -parallel=20 -tags="${GOTAGS}" ${GOFILES} ${TESTARGS}
.PHONY: test

# test-race runs the test suite.
test-race:
	@echo "==> Testing ${NAME} (race)"
	@go test -timeout=60s -race -tags="${GOTAGS}" ${GOFILES} ${TESTARGS}
.PHONY: test-race
