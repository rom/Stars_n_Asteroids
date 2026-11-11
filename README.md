# Stars n Asteroids

A 2D space shoot-'em-up in the spirit of the classic rock-blasting arcade games,
written entirely in Go:

* **Client** – Go compiled to WebAssembly, drawing on an HTML canvas (`cmd/game`).
* **Back-end** – a Go web server that serves the game and a persistent
  high-score API (`cmd/server`).
* **Sound** – every effect and the soundtrack are synthesised at run time with
  the Web Audio API; there are no asset files. Each sector has its own key and
  chord progression, every level its own melody, and the tempo rises as you go.
* **35 levels** in five sectors, each sector ending in a guardian boss
  (`internal/levels`).

No third-party dependencies – only the Go standard library.

## Run it

Requires Go 1.24+.

```bash
make run
```

then open <http://localhost:8080>. Without `make`:

```bash
GOOS=js GOARCH=wasm go build -o web/static/game.wasm ./cmd/game
```

```bash
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/static/
```

```bash
go run ./cmd/server
```

The client is embedded in the server binary, so `bin/server` is all you need to
deploy. Options:

| Flag    | Default            | Meaning                                              |
|---------|--------------------|------------------------------------------------------|
| `-addr` | `:8080` (or `$PORT`) | listen address                                     |
| `-data` | `data/scores.json` | high-score file                                      |
| `-web`  | *(embedded)*       | serve client files from a directory (for development) |

`make test` runs the unit tests and vets the WebAssembly client.

## How to play

| Key                 | Action            |
|---------------------|-------------------|
| ← → / A D           | rotate            |
| ↑ / W               | thrust            |
| Space               | fire              |
| B / X / Shift       | nova bomb         |
| P / Esc             | pause             |
| M                   | mute              |

Touch devices get on-screen buttons.

* **Rock** asteroids split in two. **Ice** is fast and shatters into three.
  **Iron** takes several hits. **Crystal** detonates with a shockwave that
  wrecks everything nearby – including you – and can chain.
* Destroyed asteroids drop **stars**. Every four stars raise your score
  multiplier (up to x9); losing a ship resets it.
* **Drones** shoot, **hunters** ram. They drop power-ups: **T**riple shot,
  **R**apid fire, **S**hield and nova **B**ombs.
* An extra ship every 30,000 points.
* Clear a level by destroying everything. Levels 7, 14, 21, 28 and 35 are
  guardian fights.

Append `?level=N` to the URL for a practice run starting at level N (practice
scores are not ranked).

## High-score API

* `GET /api/scores` → `{"scores":[{"name","score","level","date"}, …]}` (top 10)
* `POST /api/scores` with `{"name","score","level"}` → `{"rank":N,"scores":[…]}`

Scores are validated for shape and range only; the game runs in the browser, so
the table is not cheat-proof.
