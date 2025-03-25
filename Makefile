BIN_DIR := "/bin"
BIN_MIGRATOR := "./bin/gomigrator"

MAIN_MIGRATOR := "./cmd/gomigrator"

GIT_HASH := $(shell git log --format="%h" -n 1)
LDFLAGS := -X main.release="develop" -X main.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%S) -X main.gitHash=$(GIT_HASH)

test:
	go test -race ./app

integration-tests:
	go test ./integration

install-lint-deps:
	export GOROOT=$(go env GOROOT)
	(which golangci-lint > /dev/null) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.55.2

lint: install-lint-deps
	golangci-lint run ./... --exclude=".*internal/server/.*"

build: build-migrator

build-migrator: $(BIN_DIR)
	go build -o $(BIN_MIGRATOR) -ldflags "$(LDFLAGS)" $(MAIN_MIGRATOR)

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

migrate-create:
	$(BIN_MIGRATOR) create $(name)

migrate-up:
	$(BIN_MIGRATOR) up

migrate-down:
	$(BIN_MIGRATOR) down

migrate-redo:
	$(BIN_MIGRATOR) redo

migrate-status:
	$(BIN_MIGRATOR) status

migrate-dbversion:
	$(BIN_MIGRATOR) dbversion

postgres:
	docker run --name postgresdb --env POSTGRES_PASSWORD="1234512345" --publish "5436:5432" --detach --rm postgres

.PHONY: install-lint-deps lint test \
		build build-migrator \
		migrate-create migrate-up migrate-down migrate-redo migrate-status migrate-dbversion \
		postgres integration-tests
