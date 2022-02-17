.ONESHELL:
SHELL := /bin/bash

TEST_ENV_FILE = tmp/test.env

ifneq (,$(wildcard $(TEST_ENV_FILE)))
    include $(TEST_ENV_FILE)
    export
endif

.SILENT: all
.PHONY: all
all: lint tidy fmt vet test build

.SILENT: build
.PHONY: build
build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/mqtt-pinger ./src/...

.SILENT: lint
.PHONY: lint
lint:
	golangci-lint run ./src/...

.SILENT: fmt
.PHONY: fmt
fmt:
	go fmt ./src/...

.SILENT: tidy
.PHONY: tidy
tidy:
	go mod tidy

.SILENT: vet
.PHONY: vet
vet:
	go vet ./src/...

.SILENT: test
.PHONY: test
test: fmt vet
	go test -timeout 2m ./src/... -cover

.SILENT: cover
.PHONY: cover
cover:
	mkdir -p tmp
	go test -timeout 5m -coverprofile=tmp/coverage.out ./src/...
	go tool cover -html=tmp/coverage.out

.SILENT: e2e
.PHONY: e2e
e2e:
	./test/endtoend.sh
