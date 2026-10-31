GO ?= go
CONFIG ?= config/app.development.yaml

.PHONY: run build test vet fmt check clean

run:
	$(GO) run ./cmd/api --config="$(CONFIG)"

build:
	$(GO) build -o bin/api ./cmd/api

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w ./cmd ./internal ./pkg

check:
	@test -z "$$(gofmt -l ./cmd ./internal ./pkg)" || (echo "Run make fmt"; exit 1)
	$(GO) vet ./...
	$(GO) test ./...
	$(GO) build -o bin/api ./cmd/api

clean:
	rm -f bin/api
