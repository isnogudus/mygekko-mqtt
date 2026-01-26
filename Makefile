.PHONY: build clean test

BINARY := mygekko-mqtt
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w"

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY) .

clean:
	rm -f $(BINARY) $(BINARY)-*

test:
	go test -v ./...

# Cross-compile for common targets
build-all: build-linux build-openbsd build-darwin

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 .

build-openbsd:
	CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-openbsd-amd64 .

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 .
