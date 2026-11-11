GOROOT := $(shell go env GOROOT)
PORT ?= 8080

.PHONY: all build wasm server run test clean

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

clean:
	rm -rf bin web/static/game.wasm web/static/wasm_exec.js
