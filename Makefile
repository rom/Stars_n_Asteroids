GOROOT := $(shell go env GOROOT)
PORT ?= 8080
VERSION := $(shell sed -n 's/^const Version = "\(.*\)"/\1/p' internal/version/version.go)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build wasm server run test clean version dist

all: build

build: wasm server

wasm:
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o web/static/game.wasm ./cmd/game
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" web/static/wasm_exec.js

server: wasm
	go build -o bin/server ./cmd/server

run: build
	./bin/server -port $(PORT)

test:
	go test ./...
	GOOS=js GOARCH=wasm go vet ./cmd/game

version:
	@echo $(VERSION)

# dist cross-compiles the server (with the client embedded) and packs one
# archive per platform into dist/, plus checksums.txt.
dist: wasm
	rm -rf dist
	mkdir -p dist
	set -e; for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; name=stars_n_asteroids_$(VERSION)_$${os}_$${arch}; \
		bin=stars_n_asteroids; if [ $$os = windows ]; then bin=$$bin.exe; fi; \
		mkdir -p dist/build/$$name; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-s -w" -o dist/build/$$name/$$bin ./cmd/server; \
		cp README.md CHANGELOG.md dist/build/$$name/; \
		if [ $$os = windows ]; then (cd dist/build && zip -qr ../$$name.zip $$name); \
		else tar -C dist/build -czf dist/$$name.tar.gz $$name; fi; \
	done
	cd dist && sha256sum *.tar.gz *.zip > checksums.txt

clean:
	rm -rf bin dist web/static/game.wasm web/static/wasm_exec.js
