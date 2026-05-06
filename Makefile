BINARY := parcelcli
DIST := dist
VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)

.PHONY: fmt test vet build install clean ci

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './$(DIST)/*')

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p $(DIST)
	go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) ./cmd/parcelcli

install:
	go install ./cmd/parcelcli

clean:
	rm -rf $(DIST) coverage.out

ci:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './$(DIST)/*'))" || (echo "gofmt needed"; gofmt -l $$(find . -name '*.go' -not -path './$(DIST)/*'); exit 1)
	go test ./...
	go vet ./...
	go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) ./cmd/parcelcli
