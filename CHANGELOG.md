# Changelog

## v1.0.0

First release.

- 2D space shoot-'em-up played in the browser, written entirely in Go: the
  client is compiled to WebAssembly, the back-end is a single Go binary with
  the client embedded.
- 35 levels across five sectors, with a guardian boss on every seventh level.
- Four asteroid kinds (rock, ice, iron, exploding crystal), shooting drones and
  ramming hunters.
- Stars raise a score multiplier; power-ups give triple shot, rapid fire,
  shields and nova bombs.
- Space jump: wormhole to an empty spot on screen, with a 5 second cooldown.
- Synthesised sound effects and a procedural soundtrack - no audio files.
- Persistent high-score list served by the back-end.
- Settings menu: remappable keys, six difficulty levels (super easy to insane)
  and colour themes.
- Keyboard and touch controls.
- Server options: `-port`, `-host`, `-data`, `-version`.
