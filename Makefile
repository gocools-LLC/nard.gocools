BINARY := nard

.PHONY: build test test-ipv6 smoke-devnet run fmt lint

build:
	go build ./...

test:
	go test ./...

test-ipv6:
	go test ./... -run 'IPv6|DualStack' -count=1 -v

smoke-devnet:
	./scripts/devnet-smoke.sh

run:
	go run ./cmd/$(BINARY)

fmt:
	gofmt -w ./cmd ./internal ./pkg

lint:
	go vet ./...
