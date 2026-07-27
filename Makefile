.PHONY: build test check install-local

build:
	go build -o autocurl ./cmd/autocurl

test:
	go test ./...

check:
	gofmt -w .
	go vet ./...
	go test ./...

install-local:
	go install ./cmd/autocurl
