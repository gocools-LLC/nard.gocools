BINARY := nard

.PHONY: build test test-ipv6 run fmt lint

build:
	go build ./...

test:
	go test ./...

test-ipv6:
	go test ./... -run 'IPv6|DualStack' -count=1 -v

run:
	go run ./cmd/$(BINARY)

fmt:
	gofmt -w ./cmd ./internal ./pkg

lint:
	go vet ./...
