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

then open <http://localhost:8080>. To use another port:

```bash
make run PORT=9000
```

Without `make`:

```bash
GOOS=js GOARCH=wasm go build -o web/static/game.wasm ./cmd/game
```

```bash
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/static/
```

```bash
go run ./cmd/server -port 9000
```

The client is embedded in the server binary, so `bin/server` is all you need to
deploy. Options:

| Flag    | Default            | Meaning                                              |
|---------|--------------------|------------------------------------------------------|
| `-port` | `8080` (or `$PORT`) | TCP port to listen on                               |
| `-host` | *(all interfaces)* | interface to bind, e.g. `127.0.0.1`                  |
| `-data` | `data/scores.json` | high-score file                                      |
| `-web`  | *(embedded)*       | serve client files from a directory (for development) |

`make test` runs the unit tests and vets the WebAssembly client.

## How to play

| Default keys        | Action            |
|---------------------|-------------------|
| ← → / A D           | rotate            |
| ↑ / W               | thrust            |
| Space / K           | fire              |
| B / Left Shift      | nova bomb         |
| J / ↓               | space jump        |
| P / Esc             | pause             |
| M                   | mute              |
| O                   | settings (title screen or while paused) |

Touch devices get on-screen buttons.

* **Rock** asteroids split in two. **Ice** is fast and shatters into three.
  **Iron** takes several hits. **Crystal** detonates with a shockwave that
  wrecks everything nearby – including you – and can chain.
* Destroyed asteroids drop **stars**. Every four stars raise your score
  multiplier (up to x9); losing a ship resets it.
* **Drones** shoot, **hunters** ram. They drop power-ups: **T**riple shot,
  **R**apid fire, **S**hield and nova **B**ombs.
* **Space jump** wormholes the ship to a spot on screen with nothing near it
  (rocks, enemies, shots, blasts or loot). You arrive at rest, and the jump
  drive needs 5 seconds to recharge – the bar in the bottom-right corner shows it.
* An extra ship every 30,000 points.
* Clear a level by destroying everything. Levels 7, 14, 21, 28 and 35 are
  guardian fights.

## Settings

Press **O** or the ⚙ button on the title screen (or while paused). Settings are
saved in the browser.

* **Difficulty** – super easy, easy, normal, hard, super hard, insane. It changes
  starting ships and bombs, hostile speed, enemy fire rate and shot speed, rock
  count and guardian toughness. Harder settings pay more (x0.5 up to x3), and the
  difficulty is shown next to each high score. A change made while paused
  applies to the next game.
* **Controls** – two key slots per action. Click a slot and press the new key;
  a key already in use is taken from its old action.
* **Colours** – ship, shots, each asteroid kind, enemies and stars, with presets
  (default, neon, retro phosphor, colour-blind friendly, monochrome).

Append `?level=N` to the URL for a practice run starting at level N (practice
scores are not ranked).

## High-score API

* `GET /api/scores` → `{"scores":[{"name","score","level","difficulty","date"}, …]}` (top 10)
* `POST /api/scores` with `{"name","score","level","difficulty"}` (difficulty is
  optional and defaults to `normal`) → `{"rank":N,"scores":[…]}`

Scores are validated for shape and range only; the game runs in the browser, so
the table is not cheat-proof.
