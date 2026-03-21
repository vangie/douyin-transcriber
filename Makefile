.PHONY: build test clean install lint release

BINARY_NAME=dyt
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

test:
	go test ./... -v -count=1

test-short:
	go test ./... -short -count=1

clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/

install: build
	cp $(BINARY_NAME) $(GOPATH)/bin/ 2>/dev/null || cp $(BINARY_NAME) ~/go/bin/

lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "Install golangci-lint: brew install golangci-lint" && exit 1)
	golangci-lint run

release:
	@which goreleaser > /dev/null 2>&1 || (echo "Install goreleaser: brew install goreleaser" && exit 1)
	goreleaser release --clean

release-snapshot:
	@which goreleaser > /dev/null 2>&1 || (echo "Install goreleaser: brew install goreleaser" && exit 1)
	goreleaser release --snapshot --clean
