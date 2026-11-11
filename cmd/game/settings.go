//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"
)

// ----------------------------------------------------------------- actions

type action int

const (
	actLeft action = iota
	actRight
	actThrust
	actFire
	actBomb
	actJump
	actPause
	actMute
	numActions
)

var actNames = [numActions]string{"left", "right", "thrust", "fire", "bomb", "jump", "pause", "mute"}
var actLabels = [numActions]string{"Rotate left", "Rotate right", "Thrust", "Fire", "Nova bomb", "Space jump", "Pause", "Mute"}

// keySlots is the number of keys that can be bound to one action.
const keySlots = 3

var defaultKeys = [numActions][keySlots]string{
	{"ArrowLeft", "KeyA", ""},
	{"ArrowRight", "KeyD", ""},
	{"ArrowUp", "KeyW", ""},
	{"Space", "KeyK", ""},
	{"KeyB", "ShiftLeft", "ShiftRight"},
	{"KeyJ", "ArrowDown", ""},
	{"KeyP", "", ""},
	{"KeyM", "", ""},
}

// keyLabel turns a KeyboardEvent.code into something short and readable.
func keyLabel(code string) string {
	switch {
	case code == "":
		return "-"
	case strings.HasPrefix(code, "Key"):
		return code[3:]
	case strings.HasPrefix(code, "Digit"):
		return code[5:]
	case strings.HasPrefix(code, "Numpad"):
		return "Num " + code[6:]
	case strings.HasPrefix(code, "Arrow"):
		return map[string]string{"Left": "←", "Right": "→", "Up": "↑", "Down": "↓"}[code[5:]]
	case strings.HasSuffix(code, "Left") && len(code) > 4:
		return "L-" + code[:len(code)-4]
	case strings.HasSuffix(code, "Right") && len(code) > 5:
		return "R-" + code[:len(code)-5]
	}
	return code
}

// -------------------------------------------------------------- difficulty

type difficulty struct {
	id, name     string
	short        string  // tag shown in the high-score table
	lives, bombs int     // at the start of a run
	speed        float64 // hostile speed multiplier
	fire         float64 // enemy cooldown multiplier (higher = calmer)
	bullet       float64 // enemy bullet speed multiplier
	boss         float64 // guardian hit-point multiplier
	extra        int     // extra rocks per level
	score        float64 // score multiplier
	shield       bool    // every new ship starts shielded
	blurb        string
}

var difficulties = []difficulty{
	{"supereasy", "SUPER EASY", "S.EASY", 6, 3, 0.6, 1.9, 0.7, 0.6, -1, 0.5, true,
		"6 ships, 3 bombs, a shield on every ship, slow and sleepy enemies. Score x0.5"},
	{"easy", "EASY", "EASY", 4, 2, 0.8, 1.35, 0.85, 0.8, 0, 0.75, false,
		"4 ships, 2 bombs, slower rocks and enemies. Score x0.75"},
	{"normal", "NORMAL", "NORMAL", 3, 1, 1, 1, 1, 1, 0, 1, false,
		"3 ships, 1 bomb. The game as designed. Score x1"},
	{"hard", "HARD", "HARD", 3, 1, 1.2, 0.8, 1.15, 1.25, 1, 1.5, false,
		"Faster everything, more rocks, tougher guardians. Score x1.5"},
	{"superhard", "SUPER HARD", "S.HARD", 2, 0, 1.4, 0.65, 1.3, 1.5, 2, 2, false,
		"2 ships, no bombs, fast and trigger-happy enemies. Score x2"},
	{"insane", "INSANE", "INSANE", 1, 0, 1.65, 0.5, 1.5, 1.8, 3, 3, false,
		"One ship. No bombs. Everything wants you gone. Score x3"},
}

const defaultDiff = 2

func diffShort(id string) string {
	for _, d := range difficulties {
		if d.id == id {
			return d.short
		}
	}
	return "NORMAL"
}

// ------------------------------------------------------------------ colours

const (
	cShip = iota
	cShot
	cRock
	cIce
	cIron
	cCrystal
	cEnemy
	cStar
	numColors
)

var colorLabels = [numColors]string{"Ship", "Shots", "Rock", "Ice", "Iron", "Crystal", "Enemies", "Stars"}

type theme struct {
	name   string
	colors [numColors]string
}

var themes = []theme{
	{"Default", [numColors]string{"#1fa8d8", "#c9ffe9", "#c9a98a", "#9fe3ff", "#c5d0de", "#ff6bf0", "#ff5c7a", "#ffd84a"}},
	{"Neon", [numColors]string{"#00ffd0", "#faff00", "#ff7a00", "#00b3ff", "#b78cff", "#ff00aa", "#ff2b2b", "#aaff00"}},
	{"Retro phosphor", [numColors]string{"#33ff66", "#b6ffc8", "#2fd45a", "#7dffa0", "#1fa844", "#d8ffe2", "#9dff3a", "#eaff8a"}},
	{"Colour-blind friendly", [numColors]string{"#56b4e9", "#ffffff", "#e69f00", "#0072b2", "#bbbbbb", "#cc79a7", "#d55e00", "#f0e442"}},
	{"Monochrome", [numColors]string{"#ffffff", "#ffffff", "#bdbdbd", "#e6e6e6", "#8f8f8f", "#ffffff", "#d0d0d0", "#ffffff"}},
}

func validColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------- settings

type settings struct {
	diff   int
	keys   [numActions][keySlots]string
	colors [numColors]string
}

var cfg = settings{diff: defaultDiff, keys: defaultKeys, colors: themes[0].colors}

const settingsKey = "sna-settings"

func loadSettings() {
	try(func() {
		raw := win.Get("localStorage").Call("getItem", settingsKey)
		if raw.Type() != js.TypeString {
			return
		}
		o := win.Get("JSON").Call("parse", raw)
		if o.Type() != js.TypeObject {
			return
		}
		if d := o.Get("difficulty"); d.Type() == js.TypeString {
			for i, x := range difficulties {
				if x.id == d.String() {
					cfg.diff = i
				}
			}
		}
		if k := o.Get("keys"); k.Type() == js.TypeObject {
			for a := range cfg.keys {
				pair := k.Get(actNames[a])
				if pair.Type() != js.TypeObject {
					continue
				}
				for slot := 0; slot < keySlots; slot++ {
					if v := pair.Index(slot); v.Type() == js.TypeString && len(v.String()) <= 32 {
						cfg.keys[a][slot] = v.String()
					}
				}
			}
		}
		if c := o.Get("colors"); c.Type() == js.TypeObject {
			for i := range cfg.colors {
				if v := c.Index(i); v.Type() == js.TypeString && validColor(v.String()) {
					cfg.colors[i] = v.String()
				}
			}
		}
	})
}

func saveSettings() {
	keys := map[string]any{}
	for a, pair := range cfg.keys {
		keys[actNames[a]] = []any{pair[0], pair[1], pair[2]}
	}
	colors := make([]any, numColors)
	for i, c := range cfg.colors {
		colors[i] = c
	}
	try(func() {
		data := win.Get("JSON").Call("stringify", map[string]any{
			"difficulty": difficulties[cfg.diff].id,
			"keys":       keys,
			"colors":     colors,
		})
		win.Get("localStorage").Call("setItem", settingsKey, data)
	})
}

// bind assigns code to an action slot, taking it away from whatever used it.
func bind(a action, slot int, code string) {
	if code != "" {
		for i := range cfg.keys {
			for j := range cfg.keys[i] {
				if cfg.keys[i][j] == code {
					cfg.keys[i][j] = ""
				}
			}
		}
	}
	cfg.keys[a][slot] = code
}

func isBound(code string) bool {
	for _, pair := range cfg.keys {
		if pair[0] == code || pair[1] == code || pair[2] == code {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------- settings menu

var (
	settingsOpen bool
	capAct       = action(-1) // action whose key is being captured
	capSlot      int

	settingsEl, gearBtn, themeSel, diffInfo js.Value
	diffBtns                                []js.Value
	keyBtns                                 [numActions][keySlots]js.Value
	colorInputs                             [numColors]js.Value
	gearShown                               bool
)

func el(tag, class, txt string) js.Value {
	e := doc.Call("createElement", tag)
	if class != "" {
		e.Set("className", class)
	}
	if txt != "" {
		e.Set("textContent", txt)
	}
	return e
}

func setupSettings() {
	loadSettings()
	settingsEl = doc.Call("getElementById", "settings")
	gearBtn = doc.Call("getElementById", "gearBtn")
	diffInfo = doc.Call("getElementById", "diffInfo")
	themeSel = doc.Call("getElementById", "themeSel")

	row := doc.Call("getElementById", "diffRow")
	for i, d := range difficulties {
		b := el("button", "", d.name)
		b.Set("type", "button")
		listen(b, "click", func(js.Value) {
			cfg.diff = i
			saveSettings()
			refreshSettings()
		})
		row.Call("appendChild", b)
		diffBtns = append(diffBtns, b)
	}

	rows := doc.Call("getElementById", "keyRows")
	for a := action(0); a < numActions; a++ {
		rows.Call("appendChild", el("span", "lbl", actLabels[a]))
		for slot := 0; slot < keySlots; slot++ {
			b := el("button", "key", "")
			b.Set("type", "button")
			listen(b, "click", func(js.Value) {
				capAct, capSlot = a, slot
				refreshSettings()
			})
			rows.Call("appendChild", b)
			keyBtns[a][slot] = b
		}
	}

	themeSel.Call("appendChild", el("option", "", "Custom"))
	for _, t := range themes {
		themeSel.Call("appendChild", el("option", "", t.name))
	}
	listen(themeSel, "change", func(js.Value) {
		if i := themeSel.Get("selectedIndex").Int(); i > 0 {
			cfg.colors = themes[i-1].colors
			saveSettings()
			refreshSettings()
		}
	})
	crow := doc.Call("getElementById", "colorRows")
	for i := 0; i < numColors; i++ {
		lab := el("label", "", colorLabels[i])
		in := el("input", "", "")
		in.Set("type", "color")
		listen(in, "input", func(js.Value) {
			if v := in.Get("value").String(); validColor(v) {
				cfg.colors[i] = v
				saveSettings()
				refreshSettings()
			}
		})
		lab.Call("appendChild", in)
		crow.Call("appendChild", lab)
		colorInputs[i] = in
	}

	listen(doc.Call("getElementById", "resetBtn"), "click", func(js.Value) {
		cfg = settings{diff: defaultDiff, keys: defaultKeys, colors: themes[0].colors}
		capAct = -1
		saveSettings()
		refreshSettings()
	})
	listen(doc.Call("getElementById", "doneBtn"), "click", func(js.Value) { closeSettings() })
	listen(gearBtn, "click", func(js.Value) {
		snd.unlock()
		openSettings()
	})
	refreshSettings()
}

func refreshSettings() {
	for i, b := range diffBtns {
		b.Get("classList").Call("toggle", "on", i == cfg.diff)
	}
	diffInfo.Set("textContent", difficulties[cfg.diff].blurb)
	for a := range keyBtns {
		for slot, b := range keyBtns[a] {
			waiting := capAct == action(a) && capSlot == slot
			if waiting {
				b.Set("textContent", "press a key…")
			} else {
				b.Set("textContent", keyLabel(cfg.keys[a][slot]))
			}
			b.Get("classList").Call("toggle", "on", waiting)
		}
	}
	sel := 0
	for i, t := range themes {
		if t.colors == cfg.colors {
			sel = i + 1
		}
	}
	themeSel.Set("selectedIndex", sel)
	for i, in := range colorInputs {
		in.Set("value", strings.ToLower(cfg.colors[i]))
	}
}

// settingsKeyDown handles a key press while the menu is open.
func settingsKeyDown(code string, e js.Value) {
	if capAct < 0 {
		if code == "Escape" {
			closeSettings()
		}
		return
	}
	e.Call("preventDefault")
	switch code {
	case "Escape": // cancel
	case "Backspace", "Delete":
		bind(capAct, capSlot, "")
	default:
		bind(capAct, capSlot, code)
	}
	capAct = -1
	saveSettings()
	refreshSettings()
}

func openSettings() {
	if settingsOpen {
		return
	}
	settingsOpen = true
	capAct = -1
	clear(keys)
	refreshSettings()
	settingsEl.Get("classList").Call("remove", "hidden")
}

func closeSettings() {
	settingsOpen = false
	capAct = -1
	clear(keys)
	clear(pressed)
	settingsEl.Get("classList").Call("add", "hidden")
	doc.Get("activeElement").Call("blur")
}

// showGear shows the settings button only where the menu may be opened.
func showGear(on bool) {
	if on != gearShown {
		gearShown = on
		gearBtn.Get("classList").Call("toggle", "hidden", !on)
	}
}
