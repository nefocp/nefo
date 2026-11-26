PKG     := github.com/nefocp/nefo
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w -X $(PKG)/internal/version.Version=$(VERSION) -X $(PKG)/internal/version.Commit=$(COMMIT)
GOBUILD := CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)"

.PHONY: all web build test release-binaries release dev-init dev dev-web clean

all: web build

## web: build the UI into internal/web/dist (embedded into the binary)
web:
	cd web && npm ci && npm run build

## build: build bin/nefo for this machine
build:
	$(GOBUILD) -o bin/nefo ./cmd/nefo

## test: vet and test the Go code
test:
	go vet ./...
	go test ./...

## release-binaries: Linux amd64 + arm64 binaries, installer and checksums in dist/
release-binaries:
	rm -rf dist && mkdir -p dist
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o dist/nefo-linux-amd64 ./cmd/nefo
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o dist/nefo-linux-arm64 ./cmd/nefo
	cp scripts/install.sh dist/install.sh
	echo "$(VERSION)" > dist/VERSION
	# Put the public signing key into the installer before checksumming it
	# ($$ is a literal $ for awk: make expands a single one).
	if [ -s internal/update/release.pub ]; then \
	  awk 'FNR==NR { key = key $$0 "\n"; next } /^NEFO_SIGNING_KEY$$/ && !done { printf "%s", key; done = 1 } { print }' \
	    internal/update/release.pub dist/install.sh > dist/install.sh.tmp && mv dist/install.sh.tmp dist/install.sh; \
	fi
	cd dist && sha256sum nefo-linux-* install.sh > checksums.txt
	# Sign checksums.txt with the private key in $(SIGNING_KEY), if given.
	if [ -n "$(SIGNING_KEY)" ]; then \
	  openssl pkeyutl -sign -rawin -inkey "$(SIGNING_KEY)" -in dist/checksums.txt -out dist/checksums.txt.sig; \
	fi

release: web release-binaries

## dev-init: create a local dev config in .dev/ (HTTP on 127.0.0.1:8080, entrance "dev")
dev-init:
	go run ./cmd/nefo init --dev --config .dev/config.json --data .dev

## dev: run the backend with the dev config
dev:
	go run ./cmd/nefo serve --config .dev/config.json

## dev-web: run the Vite dev server (proxies /api to the backend)
dev-web:
	cd web && npm run dev

clean:
	rm -rf bin dist .dev
