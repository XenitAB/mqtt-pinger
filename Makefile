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

.SILENT: run
.PHONY: run
run:
	go run ./src/... --ping-interval 1 --brokers 127.0.0.1:1883 127.0.0.1:1884 127.0.0.1:1885

.SILENT: start-mqtt
.PHONY: start-mqtt
start-mqtt:
	docker network create --driver bridge test-mqtt-pinger 1>/dev/null
	docker run -d --rm --network test-mqtt-pinger -e "DOCKER_VERNEMQ_ACCEPT_EULA=yes" -e "DOCKER_VERNEMQ_ALLOW_ANONYMOUS=on" -p 1883:1883 --name test-mqtt-pinger-vmq0 vernemq/vernemq 1>/dev/null
	FIRST_VERNEMQ_IP=$$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' test-mqtt-pinger-vmq0)
	docker run -d --rm --network test-mqtt-pinger -e "DOCKER_VERNEMQ_ACCEPT_EULA=yes" -e "DOCKER_VERNEMQ_ALLOW_ANONYMOUS=on" -e "DOCKER_VERNEMQ_DISCOVERY_NODE=$${FIRST_VERNEMQ_IP}" -p 1884:1883 --name test-mqtt-pinger-vmq1 vernemq/vernemq 1>/dev/null
	docker run -d --rm --network test-mqtt-pinger -e "DOCKER_VERNEMQ_ACCEPT_EULA=yes" -e "DOCKER_VERNEMQ_ALLOW_ANONYMOUS=on" -e "DOCKER_VERNEMQ_DISCOVERY_NODE=$${FIRST_VERNEMQ_IP}" -p 1885:1883 --name test-mqtt-pinger-vmq2 vernemq/vernemq 1>/dev/null

.SILENT: stop-mqtt
.PHONY: stop-mqtt
stop-mqtt:
	-docker kill $$(docker ps -f name=test-mqtt-pinger-vmq0 -q) 1>/dev/null 2>&1
	-docker kill $$(docker ps -f name=test-mqtt-pinger-vmq1 -q) 1>/dev/null 2>&1
	-docker kill $$(docker ps -f name=test-mqtt-pinger-vmq2 -q) 1>/dev/null 2>&1
	-docker network rm test-mqtt-pinger 1>/dev/null 2>&1

.SILENT: e2e
.PHONY: e2e
e2e:
	(
		cd ./test
		go test -v -timeout 5m ./e2e_test.go
	)
