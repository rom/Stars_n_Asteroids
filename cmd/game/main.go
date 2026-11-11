//go:build js && wasm

// Command game is the Stars n Asteroids browser client. It compiles to
// WebAssembly and draws on an HTML canvas.
package main

import (
	"math"
	"strconv"
	"syscall/js"
)

var (
	win    = js.Global()
	doc    = win.Get("document")
	canvas js.Value
	ctx    js.Value
	scale  = 1.0

	keys    = map[string]bool{} // currently held
	pressed = map[string]bool{} // went down since the last frame

	g   *game
	snd = &audio{}
)

// down reports whether any key bound to the action is held.
func down(a action) bool {
	for _, c := range cfg.keys[a] {
		if c != "" && keys[c] {
			return true
		}
	}
	return keys["Touch:"+actNames[a]]
}

// hit reports whether a key bound to the action went down this frame.
func hit(a action) bool {
	for _, c := range cfg.keys[a] {
		if c != "" && pressed[c] {
			return true
		}
	}
	return pressed["Touch:"+actNames[a]]
}

// rawHit checks fixed (non-remappable) keys, used by the menus.
func rawHit(codes ...string) bool {
	for _, c := range codes {
		if pressed[c] {
			return true
		}
	}
	return false
}

func press(code string) {
	if !keys[code] {
		pressed[code] = true
	}
	keys[code] = true
}

// try runs f, swallowing JavaScript exceptions (e.g. blocked localStorage).
func try(f func()) {
	defer func() { recover() }()
	f()
}

func listen(target js.Value, event string, f func(e js.Value)) {
	target.Call("addEventListener", event, js.FuncOf(func(_ js.Value, args []js.Value) any {
		f(args[0])
		return nil
	}), map[string]any{"passive": false})
}

func resize() {
	ww := win.Get("innerWidth").Float()
	wh := win.Get("innerHeight").Float()
	cw := math.Min(ww, wh*W/H)
	ch := cw * H / W
	dpr := win.Get("devicePixelRatio").Float()
	if dpr < 1 || math.IsNaN(dpr) {
		dpr = 1
	}
	if dpr > 2 {
		dpr = 2
	}
	canvas.Set("width", math.Round(cw*dpr))
	canvas.Set("height", math.Round(ch*dpr))
	style := canvas.Get("style")
	style.Set("width", ftoa(math.Round(cw))+"px")
	style.Set("height", ftoa(math.Round(ch))+"px")
	scale = cw * dpr / W
}

func isTyping(e js.Value) bool {
	t := e.Get("target")
	return t.Truthy() && t.Get("tagName").Truthy() && t.Get("tagName").String() == "INPUT"
}

func setupInput() {
	listen(win, "keydown", func(e js.Value) {
		snd.unlock()
		code := e.Get("code").String()
		if settingsOpen {
			settingsKeyDown(code, e)
			return
		}
		if isTyping(e) {
			return
		}
		if !e.Get("repeat").Bool() {
			press(code)
		}
		switch code {
		case "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Space":
			e.Call("preventDefault")
		default:
			if isBound(code) && !e.Get("ctrlKey").Bool() && !e.Get("metaKey").Bool() {
				e.Call("preventDefault")
			}
		}
	})
	listen(win, "keyup", func(e js.Value) {
		delete(keys, e.Get("code").String())
	})
	listen(win, "blur", func(js.Value) {
		clear(keys)
		g.autoPause()
	})
	listen(doc, "visibilitychange", func(js.Value) {
		if doc.Get("hidden").Bool() {
			clear(keys)
			g.autoPause()
		}
	})
	listen(win, "resize", func(js.Value) { resize() })
	listen(canvas, "pointerdown", func(e js.Value) {
		snd.unlock()
		pressed["Tap"] = true
		e.Call("preventDefault")
	})
	listen(doc, "contextmenu", func(e js.Value) { e.Call("preventDefault") })

	// On-screen buttons for touch devices.
	btns := doc.Call("querySelectorAll", ".tb")
	for i := 0; i < btns.Length(); i++ {
		b := btns.Index(i)
		code := "Touch:" + b.Get("dataset").Get("act").String()
		listen(b, "pointerdown", func(e js.Value) {
			snd.unlock()
			e.Call("preventDefault")
			press(code)
			b.Get("classList").Call("add", "on")
		})
		up := func(e js.Value) {
			delete(keys, code)
			b.Get("classList").Call("remove", "on")
		}
		listen(b, "pointerup", up)
		listen(b, "pointercancel", up)
		listen(b, "pointerleave", up)
	}
}

func main() {
	canvas = doc.Call("getElementById", "game")
	ctx = canvas.Call("getContext", "2d")
	resize()

	g = newGame()
	// ?level=N starts a practice run on level N.
	try(func() {
		q := win.Get("URLSearchParams").New(win.Get("location").Get("search")).Call("get", "level")
		if q.Type() == js.TypeString {
			g.practice, _ = strconv.Atoi(q.String())
		}
	})
	fetchScores()
	setupSettings()
	setupInput()
	setupEntryForm()

	last := -1.0
	var frame js.Func
	frame = js.FuncOf(func(_ js.Value, args []js.Value) any {
		now := args[0].Float() / 1000
		dt := 0.0
		if last >= 0 {
			dt = math.Min(now-last, 0.05)
		}
		last = now
		if !settingsOpen {
			g.update(dt)
		}
		showGear(!settingsOpen && (g.st == stTitle || g.st == stPlay && g.paused))
		g.draw()
		snd.tick()
		clear(pressed)
		win.Call("requestAnimationFrame", frame)
		return nil
	})
	win.Call("requestAnimationFrame", frame)
	select {}
}
