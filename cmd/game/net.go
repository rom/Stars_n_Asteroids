//go:build js && wasm

package main

import "syscall/js"

// scoreRow is one line of the high-score table as shown on the title screen.
type scoreRow struct {
	name         string
	score, level int
}

var (
	overlay   js.Value
	nameInput js.Value
)

var (
	toJSON = js.FuncOf(func(_ js.Value, a []js.Value) any { return a[0].Call("json") })

	gotScores = js.FuncOf(func(_ js.Value, a []js.Value) any {
		d := a[0]
		if d.Type() != js.TypeObject {
			return nil
		}
		list := d.Get("scores")
		if list.Type() != js.TypeObject {
			return nil
		}
		rows := make([]scoreRow, 0, list.Length())
		for i := 0; i < list.Length(); i++ {
			e := list.Index(i)
			rows = append(rows, scoreRow{e.Get("name").String(), e.Get("score").Int(), e.Get("level").Int()})
		}
		g.scores = rows
		g.scoresErr = false
		if r := d.Get("rank"); r.Type() == js.TypeNumber {
			g.myRank = r.Int()
		}
		return nil
	})

	netFail = js.FuncOf(func(_ js.Value, a []js.Value) any {
		g.scoresErr = true
		return nil
	})
)

func fetchScores() {
	win.Call("fetch", "api/scores").Call("then", toJSON).Call("then", gotScores).Call("catch", netFail)
}

func postScore(name string, score, level int) {
	body := win.Get("JSON").Call("stringify", map[string]any{"name": name, "score": score, "level": level})
	win.Call("fetch", "api/scores", map[string]any{
		"method":  "POST",
		"headers": map[string]any{"Content-Type": "application/json"},
		"body":    body,
	}).Call("then", toJSON).Call("then", gotScores).Call("catch", netFail)
}

func setupEntryForm() {
	overlay = doc.Call("getElementById", "overlay")
	nameInput = doc.Call("getElementById", "nameInput")

	listen(doc.Call("getElementById", "nameForm"), "submit", func(e js.Value) {
		e.Call("preventDefault")
		name := nameInput.Get("value").String()
		if name == "" {
			nameInput.Call("focus")
			return
		}
		try(func() { win.Get("localStorage").Call("setItem", "sna-name", name) })
		postScore(name, g.score, g.reached)
		closeEntry(false)
	})
	listen(doc.Call("getElementById", "skipBtn"), "click", func(js.Value) { closeEntry(true) })
}

func showEntry(title, text string) {
	doc.Call("getElementById", "entryTitle").Set("textContent", title)
	doc.Call("getElementById", "entryText").Set("textContent", text)
	try(func() {
		if v := win.Get("localStorage").Call("getItem", "sna-name"); v.Type() == js.TypeString {
			nameInput.Set("value", v.String())
		}
	})
	overlay.Get("classList").Call("remove", "hidden")
	nameInput.Call("focus")
	nameInput.Call("select")
}

func closeEntry(refresh bool) {
	overlay.Get("classList").Call("add", "hidden")
	nameInput.Call("blur")
	clear(keys)
	g.toTitle(refresh)
}
