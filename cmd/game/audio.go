//go:build js && wasm

package main

import (
	"encoding/binary"
	"math"
	"math/rand"
	"syscall/js"
)

// audio synthesises every sound effect and the soundtrack with the Web Audio
// API - there are no sample files.
type audio struct {
	ok     bool
	muted  bool
	ac     js.Value // AudioContext
	master js.Value
	music  js.Value // music bus
	sfx    js.Value // effects bus
	bass   js.Value // low-passed bus for the bass line
	soft   js.Value // low-passed bus for arpeggio/pad
	nbuf   js.Value // white-noise buffer
	thrust js.Value // gain of the looping engine noise
	thrOn  bool

	// sequencer
	mode   int // 0 title, 1 level, 2 boss, 3 quiet
	level  int
	next   float64 // time of the next 16th note
	step   int
	motifA [32]int
	motifB [32]int
}

const (
	musTitle = iota
	musLevel
	musBoss
	musQuiet
)

// unlock creates (or resumes) the audio context. Browsers only allow this
// from a user gesture, so it is called from input handlers.
func (a *audio) unlock() {
	if a.ok {
		if a.ac.Get("state").String() == "suspended" {
			a.ac.Call("resume")
		}
		return
	}
	defer func() { recover() }()
	ctor := win.Get("AudioContext")
	if !ctor.Truthy() {
		ctor = win.Get("webkitAudioContext")
	}
	if !ctor.Truthy() {
		return
	}
	a.ac = ctor.New()
	a.master = a.gainNode(0.8, a.ac.Get("destination"))
	if a.muted {
		a.master.Get("gain").Set("value", 0)
	}
	a.music = a.gainNode(0.5, a.master)
	a.sfx = a.gainNode(0.7, a.master)
	a.bass = a.filter("lowpass", 520, a.music)
	a.soft = a.filter("lowpass", 2600, a.music)

	// Two seconds of white noise, shared by all percussive sounds.
	sr := a.ac.Get("sampleRate").Int()
	n := sr * 2
	raw := make([]byte, n*4)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(rand.Float32()*2-1))
	}
	u8 := win.Get("Uint8Array").New(len(raw))
	js.CopyBytesToJS(u8, raw)
	a.nbuf = a.ac.Call("createBuffer", 1, n, sr)
	a.nbuf.Call("copyToChannel", win.Get("Float32Array").New(u8.Get("buffer")), 0)

	// Engine rumble: looping noise whose gain follows the thrust key.
	src := a.ac.Call("createBufferSource")
	src.Set("buffer", a.nbuf)
	src.Set("loop", true)
	a.thrust = a.gainNode(0, a.sfx)
	src.Call("connect", a.filter("lowpass", 380, a.thrust))
	src.Call("start")

	a.ok = true
	a.next = a.now() + 0.1
	a.makeMotifs()
}

func (a *audio) now() float64 { return a.ac.Get("currentTime").Float() }

func (a *audio) gainNode(v float64, dest js.Value) js.Value {
	n := a.ac.Call("createGain")
	n.Get("gain").Set("value", v)
	n.Call("connect", dest)
	return n
}

func (a *audio) filter(kind string, freq float64, dest js.Value) js.Value {
	f := a.ac.Call("createBiquadFilter")
	f.Set("type", kind)
	f.Get("frequency").Set("value", freq)
	f.Call("connect", dest)
	return f
}

func (a *audio) toggleMute() {
	a.muted = !a.muted
	if a.ok {
		v := 0.8
		if a.muted {
			v = 0
		}
		a.master.Get("gain").Call("setTargetAtTime", v, a.now(), 0.02)
	}
}

// env builds a gain node with an attack/decay envelope.
func (a *audio) env(dest js.Value, t0, attack, dur, vol float64) js.Value {
	gn := a.ac.Call("createGain")
	p := gn.Get("gain")
	p.Call("setValueAtTime", 0.0001, t0)
	p.Call("linearRampToValueAtTime", vol, t0+attack)
	p.Call("exponentialRampToValueAtTime", 0.0001, t0+dur)
	gn.Call("connect", dest)
	return gn
}

// tone plays an oscillator sweeping from f0 to f1.
func (a *audio) tone(dest js.Value, wave string, f0, f1, t0, attack, dur, vol float64) {
	o := a.ac.Call("createOscillator")
	o.Set("type", wave)
	fp := o.Get("frequency")
	fp.Call("setValueAtTime", f0, t0)
	if f1 != f0 {
		fp.Call("exponentialRampToValueAtTime", f1, t0+dur)
	}
	o.Call("connect", a.env(dest, t0, attack, dur, vol))
	o.Call("start", t0)
	o.Call("stop", t0+dur+0.05)
}

// noise plays filtered white noise with the filter sweeping from f0 to f1.
func (a *audio) noise(dest js.Value, kind string, f0, f1, t0, dur, vol float64) {
	src := a.ac.Call("createBufferSource")
	src.Set("buffer", a.nbuf)
	f := a.ac.Call("createBiquadFilter")
	f.Set("type", kind)
	fp := f.Get("frequency")
	fp.Call("setValueAtTime", f0, t0)
	if f1 != f0 {
		fp.Call("exponentialRampToValueAtTime", f1, t0+dur)
	}
	src.Call("connect", f)
	f.Call("connect", a.env(dest, t0, 0.004, dur, vol))
	src.Call("start", t0, rand.Float64()*1.5)
	src.Call("stop", t0+dur+0.05)
}

// ---------------------------------------------------------------- effects

func (a *audio) shoot() {
	if !a.ok {
		return
	}
	t := a.now()
	a.tone(a.sfx, "square", 980+rand.Float64()*60, 240, t, 0.003, 0.13, 0.12)
	a.tone(a.sfx, "sawtooth", 1900, 500, t, 0.003, 0.06, 0.05)
}

func (a *audio) enemyShoot() {
	if !a.ok {
		return
	}
	a.tone(a.sfx, "sawtooth", 300, 620, a.now(), 0.005, 0.18, 0.1)
}

// boom plays an explosion; size 1 (small) to 4 (huge).
func (a *audio) boom(size int) {
	if !a.ok {
		return
	}
	t := a.now()
	s := float64(size)
	a.noise(a.sfx, "lowpass", 2600-s*400, 90, t, 0.18+s*0.14, 0.28+s*0.07)
	a.tone(a.sfx, "sine", 150-s*18, 34, t, 0.004, 0.16+s*0.1, 0.3+s*0.06)
}

func (a *audio) shatter() {
	if !a.ok {
		return
	}
	t := a.now()
	a.noise(a.sfx, "highpass", 3500, 7000, t, 0.22, 0.2)
	a.tone(a.sfx, "triangle", 2400, 1300, t, 0.002, 0.15, 0.1)
	a.tone(a.sfx, "triangle", 3100, 1900, t+0.04, 0.002, 0.15, 0.08)
}

func (a *audio) clang() {
	if !a.ok {
		return
	}
	t := a.now()
	a.tone(a.sfx, "square", 1320, 1290, t, 0.002, 0.16, 0.07)
	a.tone(a.sfx, "square", 1987, 1950, t, 0.002, 0.12, 0.05)
}

func (a *audio) star(mult int) {
	if !a.ok {
		return
	}
	t := a.now()
	f := 880 * math.Pow(2, float64(mult-1)/12)
	a.tone(a.sfx, "sine", f, f, t, 0.004, 0.09, 0.16)
	a.tone(a.sfx, "sine", f*1.5, f*1.5, t+0.07, 0.004, 0.16, 0.16)
}

func (a *audio) arp(wave string, base float64, semis []int, gap, dur, vol float64) {
	if !a.ok {
		return
	}
	t := a.now()
	for i, s := range semis {
		f := base * math.Pow(2, float64(s)/12)
		a.tone(a.sfx, wave, f, f, t+float64(i)*gap, 0.005, dur, vol)
	}
}

func (a *audio) powerup()   { a.arp("square", 440, []int{0, 4, 7, 12, 16}, 0.055, 0.14, 0.1) }
func (a *audio) extraLife() { a.arp("triangle", 660, []int{0, 7, 12, 7, 12, 19}, 0.08, 0.2, 0.2) }
func (a *audio) levelUp()   { a.arp("triangle", 392, []int{0, 4, 7, 12, 7, 12, 16}, 0.1, 0.3, 0.2) }
func (a *audio) win() {
	a.arp("triangle", 330, []int{0, 4, 7, 12, 16, 19, 24, 19, 24, 28}, 0.14, 0.6, 0.22)
}
func (a *audio) gameOver()  { a.arp("sawtooth", 220, []int{12, 8, 7, 3, 0, -5, -12}, 0.2, 0.5, 0.12) }
func (a *audio) shieldHit() { a.arp("sine", 300, []int{12, 5, 0}, 0.05, 0.25, 0.25) }

func (a *audio) die() {
	if !a.ok {
		return
	}
	t := a.now()
	a.tone(a.sfx, "sawtooth", 520, 40, t, 0.005, 0.9, 0.25)
	a.noise(a.sfx, "lowpass", 3000, 60, t, 1.1, 0.45)
}

func (a *audio) bomb() {
	if !a.ok {
		return
	}
	t := a.now()
	a.tone(a.sfx, "sine", 90, 24, t, 0.01, 1.2, 0.6)
	a.noise(a.sfx, "lowpass", 200, 5000, t, 0.5, 0.4)
	a.noise(a.sfx, "lowpass", 5000, 80, t+0.3, 1.0, 0.4)
}

func (a *audio) bossWarn() {
	if !a.ok {
		return
	}
	t := a.now()
	for i := 0; i < 3; i++ {
		a.tone(a.sfx, "sawtooth", 220, 440, t+float64(i)*0.45, 0.02, 0.4, 0.16)
	}
}

func (a *audio) setThrust(on bool) {
	if !a.ok || on == a.thrOn {
		return
	}
	a.thrOn = on
	v := 0.0
	if on {
		v = 0.5
	}
	a.thrust.Get("gain").Call("setTargetAtTime", v, a.now(), 0.04)
}

// ------------------------------------------------------------------ music

// Each sector has its own key and chord progression (semitones above the
// key note; the bool says whether the chord is major).
type chord struct {
	root  int
	major bool
}

var progressions = [][4]chord{
	{{0, false}, {8, true}, {3, true}, {10, true}}, // i  VI III VII
	{{0, false}, {5, false}, {8, true}, {7, true}}, // i  iv VI  V
	{{0, false}, {3, true}, {10, true}, {5, false}},
	{{0, false}, {10, true}, {8, true}, {7, true}},
	{{0, false}, {1, true}, {8, true}, {7, true}},
}

var keyOffsets = []int{0, 3, -2, 5, 1} // per sector, relative to A
var pentatonic = []int{0, 3, 5, 7, 10, 12, 15}

func (a *audio) setMusic(mode, level int) {
	if a.mode == mode && a.level == level {
		return
	}
	a.mode, a.level = mode, level
	a.step = 0
	a.makeMotifs()
}

// makeMotifs composes the lead melody for the current level: two short
// phrases that are played A A B A. The same level always gets the same tune.
func (a *audio) makeMotifs() {
	r := rand.New(rand.NewSource(int64(a.level)*7919 + int64(a.mode)))
	gen := func(m *[32]int) {
		idx := 2 + r.Intn(3)
		for i := range m {
			m[i] = -1
			p := 0.16
			if i%4 == 0 {
				p = 0.7
			} else if i%2 == 0 {
				p = 0.38
			}
			if r.Float64() < p {
				idx += r.Intn(5) - 2
				if idx < 0 {
					idx = 1
				}
				if idx >= len(pentatonic) {
					idx = len(pentatonic) - 2
				}
				m[i] = pentatonic[idx]
			}
		}
	}
	gen(&a.motifA)
	gen(&a.motifB)
}

func (a *audio) bpm() float64 {
	switch a.mode {
	case musLevel:
		return 106 + float64(a.level)*1.2
	case musBoss:
		return 120 + float64(a.level)*1.0
	}
	return 84
}

// tick schedules the notes that fall inside the look-ahead window.
func (a *audio) tick() {
	if !a.ok {
		return
	}
	now := a.now()
	if a.next < now-0.05 { // tab was in the background
		a.next = now + 0.05
	}
	stepDur := 60 / a.bpm() / 4
	for a.next < now+0.18 {
		a.playStep(a.step, a.next, stepDur)
		a.next += stepDur
		a.step++
	}
}

func semis(base float64, s int) float64 { return base * math.Pow(2, float64(s)/12) }

func (a *audio) playStep(step int, t, sd float64) {
	sector := 0
	if a.mode == musLevel || a.mode == musBoss {
		sector = (a.level - 1) / 7 % len(progressions)
	}
	key := semis(110, keyOffsets[sector]) // A2-ish
	bar := step / 16
	s := step % 16
	ch := progressions[sector][bar%4]
	third := 3
	if ch.major {
		third = 4
	}
	root := semis(key, ch.root)
	if ch.root >= 7 {
		root /= 2 // keep the bass in a narrow range
	}

	// Pad: one soft chord per bar.
	if s == 0 {
		vol := 0.05
		if a.mode == musTitle || a.mode == musQuiet {
			vol = 0.09
		}
		for _, iv := range []int{0, third, 7} {
			a.tone(a.soft, "triangle", semis(root*2, iv), semis(root*2, iv), t, sd*4, sd*16, vol)
		}
	}

	if a.mode == musQuiet {
		return
	}

	// Arpeggio.
	pattern := []int{0, 7, 12, third + 12, 7, 12, third + 12, 19}
	if a.mode == musTitle {
		if s%2 == 0 {
			f := semis(root*2, pattern[(s/2)%8])
			a.tone(a.soft, "triangle", f, f, t, 0.01, sd*3, 0.09)
		}
		return
	}
	f := semis(root*2, pattern[s%8])
	a.tone(a.soft, "square", f, f, t, 0.004, sd*0.9, 0.035)

	// Bass.
	switch s {
	case 0, 3, 8, 11:
		a.tone(a.bass, "sawtooth", root, root, t, 0.005, sd*2.2, 0.3)
	case 6, 14:
		a.tone(a.bass, "sawtooth", root*2, root*2, t, 0.005, sd*1.5, 0.22)
	}
	if a.mode == musBoss && s%2 == 1 {
		a.tone(a.bass, "sawtooth", root, root, t, 0.005, sd*0.8, 0.16)
	}

	// Drums.
	kick := s%4 == 0 || (a.mode == musBoss && (s == 10 || s == 15))
	if kick {
		a.tone(a.music, "sine", 150, 42, t, 0.002, 0.16, 0.5)
	}
	if s == 4 || s == 12 {
		a.noise(a.music, "bandpass", 1800, 1200, t, 0.13, 0.22)
	}
	if s%2 == 0 || a.mode == musBoss || a.level > 14 {
		v := 0.05
		if s%4 == 2 {
			v = 0.09
		}
		a.noise(a.music, "highpass", 7000, 7000, t, 0.04, v)
	}

	// Lead melody: phrases A A B A, resting for the first four bars.
	if bar%16 < 4 {
		return
	}
	m := &a.motifA
	if (bar/2)%4 == 2 {
		m = &a.motifB
	}
	if n := m[(bar%2)*16+s]; n >= 0 {
		lf := semis(key*4, n)
		wave := "triangle"
		vol := 0.11
		if a.mode == musBoss {
			wave, vol = "sawtooth", 0.05
		}
		a.tone(a.soft, wave, lf, lf, t, 0.008, sd*2.4, vol)
	}
}
