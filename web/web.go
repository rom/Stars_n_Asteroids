// Package web embeds the browser client (HTML, JS glue and the compiled
// WebAssembly game). Run `make wasm` before building the server so that
// static/game.wasm and static/wasm_exec.js exist.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var files embed.FS

// Static returns the embedded client files.
func Static() fs.FS {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
