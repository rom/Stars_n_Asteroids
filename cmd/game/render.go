//go:build js && wasm

package main

import (
	"math"
	"strconv"

	"stars_n_asteroids/internal/levels"
	"stars_n_asteroids/internal/version"
)

const fontFace = `"Trebuchet MS", "Segoe UI", system-ui, sans-serif`

var fontCache = map[int]string{}

func setFont(size int) {
	f, ok := fontCache[size]
	if !ok {
		f = "bold " + strconv.Itoa(size) + "px " + fontFace
		fontCache[size] = f
	}
	ctx.Set("font", f)
}

func text(s string, x, y float64, size int, color, align string) {
	setFont(size)
	ctx.Set("textAlign", align)
	ctx.Set("fillStyle", color)
	ctx.Call("fillText", s, x, y)
}

func hsl(h, s, l float64) string {
	return "hsl(" + ftoa(math.Mod(h, 360)) + "," + ftoa(s) + "%," + ftoa(l) + "%)"
}

// path traces a closed polygon given in local coordinates (x forward).
func path(x, y, a, sc float64, pts []float64) {
	cs, sn := math.Cos(a)*sc, math.Sin(a)*sc
	ctx.Call("beginPath")
	for i := 0; i < len(pts); i += 2 {
		px := x + pts[i]*cs - pts[i+1]*sn
		py := y + pts[i]*sn + pts[i+1]*cs
		if i == 0 {
			ctx.Call("moveTo", px, py)
		} else {
			ctx.Call("lineTo", px, py)
		}
	}
	ctx.Call("closePath")
}

func circle(x, y, r float64) {
	ctx.Call("beginPath")
	ctx.Call("arc", x, y, r, 0, tau)
}

func fill(color string) {
	ctx.Set("fillStyle", color)
	ctx.Call("fill")
}

func stroke(color string, w float64) {
	ctx.Set("strokeStyle", color)
	ctx.Set("lineWidth", w)
	ctx.Call("stroke")
}

func alpha(a float64) { ctx.Set("globalAlpha", math.Max(0, math.Min(1, a))) }

var (
	shipHull   = []float64{19, 0, -3, -8, -14, -15, -9, -5, -12, 0, -9, 5, -14, 15, -3, 8}
	shipFlame  = []float64{-11, -4.5, -26, 0, -11, 4.5}
	droneHull  = []float64{17, 0, 5, -8, -5, -8, -17, 0, -5, 8, 5, 8}
	hunterHull = []float64{17, 0, -11, -13, -4, 0, -11, 13}
	starShape  = []float64{}
	hexShape   = []float64{}
)

func init() {
	for i := 0; i < 10; i++ {
		r := 1.0
		if i%2 == 1 {
			r = 0.45
		}
		a := float64(i) * tau / 10
		starShape = append(starShape, math.Cos(a)*r, math.Sin(a)*r)
	}
	for i := 0; i < 6; i++ {
		a := float64(i) * tau / 6
		hexShape = append(hexShape, math.Cos(a), math.Sin(a))
	}
}

func drawShip(x, y, a, sc float64, flame bool, t float64) {
	if flame {
		fl := shipFlame
		fl[2] = -20 - 8*math.Abs(math.Sin(t*40))
		path(x, y, a, sc, fl)
		fill("#ffb347")
	}
	path(x, y, a, sc, shipHull)
	fill(cfg.colors[cShip])
	stroke("#d8f6ff", 1.6)
	circle(x+math.Cos(a)*4*sc, y+math.Sin(a)*4*sc, 3.2*sc)
	fill("#06283a")
}

func (g *game) draw() {
	ctx.Call("setTransform", scale, 0, 0, scale, 0, 0)
	alpha(1)

	// Backdrop: sector-tinted gradient, a nebula glow and drifting stars.
	hue := g.lvl.Hue
	grad := ctx.Call("createLinearGradient", 0, 0, 0, H)
	grad.Call("addColorStop", 0, hsl(hue, 55, 9))
	grad.Call("addColorStop", 1, "#02030a")
	ctx.Set("fillStyle", grad)
	ctx.Call("fillRect", 0, 0, W, H)

	nx := W * (0.2 + 0.6*math.Mod(float64(g.lvl.Num)*0.37, 1))
	ny := H * (0.25 + 0.5*math.Mod(float64(g.lvl.Num)*0.61, 1))
	neb := ctx.Call("createRadialGradient", nx, ny, 10, nx, ny, 420)
	neb.Call("addColorStop", 0, "hsla("+ftoa(math.Mod(hue+40, 360))+",80%,45%,0.22)")
	neb.Call("addColorStop", 1, "hsla("+ftoa(math.Mod(hue+40, 360))+",80%,45%,0)")
	ctx.Set("fillStyle", neb)
	ctx.Call("fillRect", 0, 0, W, H)

	ctx.Set("fillStyle", "#dfeaff")
	for _, b := range g.bg {
		alpha(b.z * (0.55 + 0.45*math.Sin(g.t*2+b.tw)))
		sz := 1 + b.z*1.6
		ctx.Call("fillRect", b.x, b.y, sz, sz)
	}
	alpha(1)

	if g.shake > 0 && !g.paused {
		ctx.Call("translate", math.Sin(g.t*91)*g.shake*0.5, math.Cos(g.t*77)*g.shake*0.5)
	}

	g.drawWorld()

	ctx.Call("setTransform", scale, 0, 0, scale, 0, 0)
	if g.flash > 0 {
		alpha(g.flash * 1.6)
		ctx.Set("fillStyle", "#ffffff")
		ctx.Call("fillRect", 0, 0, W, H)
		alpha(1)
	}

	switch g.st {
	case stTitle:
		g.drawTitle()
	case stPlay:
		g.drawHUD()
		g.drawBanners()
	case stOver:
		g.drawHUD()
		ctx.Set("fillStyle", "rgba(2,3,10,0.55)")
		ctx.Call("fillRect", 0, 0, W, H)
		if g.won {
			text("GALAXY CLEARED", W/2, 280, 76, "#ffd84a", "center")
			text("All 35 levels survived. The stars are yours.", W/2, 335, 24, "#cfe6ff", "center")
		} else {
			text("GAME OVER", W/2, 290, 84, "#ff5c6c", "center")
			text("Reached level "+strconv.Itoa(g.reached)+" - "+levels.Get(g.reached).Name, W/2, 340, 24, "#cfe6ff", "center")
		}
		text("SCORE  "+commas(g.score), W/2, 400, 40, "#ffffff", "center")
		if g.asked && (g.score == 0 || g.practice > 1) {
			text("PRESS ENTER", W/2, 470, 22, "#9fc4e8", "center")
		}
	}
	if g.practice > 1 {
		text("PRACTICE RUN - NOT RANKED", 16, H-14, 14, "#7f93b3", "left")
	}
	if snd.muted {
		text("MUTED", W/2, H-14, 14, "#7f93b3", "center")
	}
}

func (g *game) drawWorld() {
	// Shockwaves.
	for _, s := range g.shocks {
		k := s.t / s.dur
		alpha(1 - k)
		circle(s.x, s.y, s.max*k)
		col := "#ff9bf5"
		if s.friendly {
			col = "#9fe8ff"
		}
		stroke(col, 10*(1-k)+2)
		alpha((1 - k) * 0.15)
		fill(col)
	}
	alpha(1)

	// Pickups.
	for _, p := range g.picks {
		if p.life < 2.5 && math.Mod(p.life, 0.3) < 0.12 {
			continue
		}
		pulse := 1 + 0.15*math.Sin(g.t*8+p.x)
		if p.kind == pStar {
			path(p.x, p.y, g.t*2+p.x, 11*pulse, starShape)
			fill(cfg.colors[cStar])
			stroke("#fff6c9", 1.2)
			continue
		}
		col, label := "#7dff9a", "T"
		switch p.kind {
		case pRapid:
			col, label = "#ff9d5c", "R"
		case pShield:
			col, label = "#7df0ff", "S"
		case pBomb:
			col, label = "#ff6bf0", "B"
		}
		circle(p.x, p.y, 13*pulse)
		fill("#0a1326")
		stroke(col, 2.5)
		text(label, p.x, p.y+5.5, 16, col, "center")
	}

	// Asteroids.
	for _, a := range g.asts {
		n := len(a.shape)
		ctx.Call("beginPath")
		for i, m := range a.shape {
			ang := a.a + float64(i)*tau/float64(n)
			px, py := a.x+math.Cos(ang)*a.r*m, a.y+math.Sin(ang)*a.r*m
			if i == 0 {
				ctx.Call("moveTo", px, py)
			} else {
				ctx.Call("lineTo", px, py)
			}
		}
		ctx.Call("closePath")
		if a.flash > 0 {
			fill("#ffffff")
			continue
		}
		// Tinted translucent body with a solid outline in the chosen colour.
		col := cfg.colors[astColor[a.kind]]
		ctx.Set("fillStyle", "#0a0d16")
		ctx.Call("fill")
		switch a.kind {
		case kRock:
			alpha(0.38)
			fill(col)
			alpha(1)
			stroke(col, 2)
		case kIce:
			alpha(0.5)
			fill(col)
			alpha(1)
			stroke("#ffffff", 1.2)
			stroke(col, 2.4)
		case kIron:
			alpha(0.25)
			fill(col)
			alpha(1)
			stroke(col, 3.8)
			circle(a.x, a.y, a.r*0.35)
			alpha(0.6)
			stroke(col, 2)
			alpha(1)
		case kCrystal:
			alpha(0.6 + 0.25*math.Sin(g.t*7+a.x))
			fill(col)
			alpha(1)
			stroke(col, 2.2)
			circle(a.x, a.y, a.r*0.28)
			fill("#ffffff")
		}
	}

	// Enemies.
	for _, e := range g.enemies {
		white := e.flash > 0
		switch e.kind {
		case eDrone:
			path(e.x, e.y, 0, 1, droneHull)
			fill("#0a0d16")
			if white {
				fill("#ffffff")
			} else {
				alpha(0.55)
				fill(cfg.colors[cEnemy])
				alpha(1)
			}
			stroke(cfg.colors[cEnemy], 2)
			circle(e.x, e.y, 4+1.5*math.Sin(e.t*9))
			fill("#ffe07a")
		case eHunter:
			path(e.x, e.y, e.a, 1, hunterHull)
			fill("#0a0d16")
			if white {
				fill("#ffffff")
			} else {
				alpha(0.8)
				fill(cfg.colors[cEnemy])
				alpha(1)
			}
			stroke("#ffffff", 1.4)
		case eBoss:
			for i := 0; i < 3; i++ {
				a0 := -e.a*1.6 + float64(i)*tau/3
				ctx.Call("beginPath")
				ctx.Call("arc", e.x, e.y, e.r+16, a0, a0+1.3)
				stroke("#ff6bf0", 5)
			}
			path(e.x, e.y, e.a, e.r, hexShape)
			if white {
				fill("#ffffff")
			} else {
				fill("#2a1445")
			}
			stroke("#d7a8ff", 3.5)
			path(e.x, e.y, -e.a, e.r*0.6, hexShape)
			stroke("#8a5cc9", 2)
			frac := float64(e.hp) / float64(e.maxhp)
			circle(e.x, e.y, 15+4*math.Sin(e.t*6))
			fill(hsl(frac*50, 100, 60))
		}
	}

	// Bullets.
	for _, b := range g.bullets {
		if b.enemy {
			circle(b.x, b.y, 5)
			fill(cfg.colors[cEnemy])
			circle(b.x, b.y, 2.2)
			fill("#ffe1e6")
		} else {
			ctx.Set("strokeStyle", cfg.colors[cShot])
			ctx.Set("lineWidth", 3.2)
			ctx.Set("lineCap", "round")
			ctx.Call("beginPath")
			ctx.Call("moveTo", b.x, b.y)
			ctx.Call("lineTo", b.x-b.vx*0.014, b.y-b.vy*0.014)
			ctx.Call("stroke")
		}
	}

	// Space-jump trails: a fading line and two collapsing rings.
	for _, w := range g.warps {
		k := w.t / 0.45
		alpha(0.7 * (1 - k))
		ctx.Call("setLineDash", []any{6, 10})
		ctx.Call("beginPath")
		ctx.Call("moveTo", w.x0, w.y0)
		ctx.Call("lineTo", w.x1, w.y1)
		stroke("#b78cff", 2)
		ctx.Call("setLineDash", []any{})
		circle(w.x0, w.y0, 6+40*k)
		stroke("#b78cff", 3)
		circle(w.x1, w.y1, 46-40*k)
		stroke("#e3d2ff", 3)
	}
	alpha(1)

	// Ship.
	if s := &g.ship; s.alive && (s.inv <= 0 || math.Mod(g.t, 0.2) < 0.12) {
		drawShip(s.x, s.y, s.a, 1, s.thrust, g.t)
		if s.shield {
			circle(s.x, s.y, 25+math.Sin(g.t*6))
			alpha(0.18)
			fill("#7df0ff")
			alpha(0.9)
			stroke("#7df0ff", 2)
			alpha(1)
		}
	}

	// Particles (additive).
	ctx.Set("globalCompositeOperation", "lighter")
	last := ""
	for i := range g.parts {
		p := &g.parts[i]
		if p.color != last {
			ctx.Set("fillStyle", p.color)
			last = p.color
		}
		alpha(p.life / p.max)
		ctx.Call("fillRect", p.x-p.size/2, p.y-p.size/2, p.size, p.size)
	}
	ctx.Set("globalCompositeOperation", "source-over")

	for _, f := range g.floats {
		alpha(f.life)
		text(f.s, f.x, f.y, 16, f.color, "center")
	}
	alpha(1)
}

func (g *game) drawHUD() {
	s := &g.ship
	text(commas(g.score), 18, 36, 30, "#ffffff", "left")
	if g.mult > 1 {
		text("x"+strconv.Itoa(g.mult), 18, 64, 22, "#ffd84a", "left")
	}
	// Progress towards the next multiplier step.
	if g.mult < maxMult {
		for i := 0; i < starsPerX; i++ {
			path(70+float64(i)*16, 57, -math.Pi/2, 6, starShape)
			if i < g.stars%starsPerX {
				fill(cfg.colors[cStar])
			} else {
				stroke("#6b6230", 1)
			}
		}
	}

	text("LEVEL "+strconv.Itoa(g.lvl.Num)+" / "+strconv.Itoa(levels.Count), W/2, 28, 18, "#cfe6ff", "center")
	text(g.lvl.SectorName+" - "+g.lvl.Name, W/2, 48, 14, "#8fa9cc", "center")

	for i := 0; i < g.lives; i++ {
		drawShip(W-28-float64(i)*30, 30, -math.Pi/2, 0.8, false, 0)
	}
	for i := 0; i < g.bombs; i++ {
		circle(W-28-float64(i)*26, 62, 8)
		fill("#0a1326")
		stroke("#ff6bf0", 2.5)
	}

	// Active power-ups.
	y := H - 18.0
	bar := func(label, col string, left, total float64) {
		text(label, 18, y, 14, col, "left")
		ctx.Set("fillStyle", "rgba(255,255,255,0.15)")
		ctx.Call("fillRect", 96, y-10, 110, 8)
		ctx.Set("fillStyle", col)
		ctx.Call("fillRect", 96, y-10, 110*left/total, 8)
		y -= 20
	}
	if s.triple > 0 {
		bar("TRIPLE", "#7dff9a", s.triple, 14)
	}
	if s.rapid > 0 {
		bar("RAPID", "#ff9d5c", s.rapid, 14)
	}
	if s.shield {
		text("SHIELD", 18, y, 14, "#7df0ff", "left")
	}

	// Space-jump charge.
	if s.alive {
		const bw = 110.0
		frac := 1 - s.jumpCool/jumpCooldown
		col := "#b78cff"
		label := "JUMP " + keyLabel(cfg.keys[actJump][0])
		if frac < 1 {
			col = "#6a5a8c"
			label = "JUMP " + strconv.FormatFloat(s.jumpCool, 'f', 1, 64) + "s"
		}
		text(label, W-18-bw-10, H-18, 14, col, "right")
		ctx.Set("fillStyle", "rgba(255,255,255,0.15)")
		ctx.Call("fillRect", W-18-bw, H-28, bw, 8)
		ctx.Set("fillStyle", col)
		ctx.Call("fillRect", W-18-bw, H-28, bw*frac, 8)
	}
	if g.diff.id != "normal" {
		text(g.diff.name, W/2, 66, 12, "#8fa9cc", "center")
	}

	// Guardian health.
	for _, e := range g.enemies {
		if e.kind == eBoss {
			w := 420.0
			ctx.Set("fillStyle", "rgba(255,255,255,0.15)")
			ctx.Call("fillRect", W/2-w/2, 74, w, 10)
			ctx.Set("fillStyle", "#ff6bf0")
			ctx.Call("fillRect", W/2-w/2, 74, w*float64(e.hp)/float64(e.maxhp), 10)
		}
	}
}

func (g *game) drawBanners() {
	if g.intro > 0 {
		alpha(math.Min(1, g.intro/0.6))
		if g.lvl.Boss {
			text("WARNING", W/2, 250, 30, "#ff5c6c", "center")
		} else {
			text("LEVEL "+strconv.Itoa(g.lvl.Num), W/2, 250, 30, "#9fc4e8", "center")
		}
		text(g.lvl.Name, W/2, 312, 60, "#ffffff", "center")
		text(g.lvl.SectorName, W/2, 348, 20, hsl(g.lvl.Hue, 80, 70), "center")
		alpha(1)
	}
	if g.clearT > 0 && g.lvl.Num < levels.Count {
		text("LEVEL CLEAR", W/2, H/2-20, 54, "#7dff9a", "center")
	}
	if g.paused {
		ctx.Set("fillStyle", "rgba(2,3,10,0.6)")
		ctx.Call("fillRect", 0, 0, W, H)
		text("PAUSED", W/2, H/2, 64, "#ffffff", "center")
		text(keyLabel(cfg.keys[actPause][0])+" / Esc to resume  -  O for settings", W/2, H/2+44, 20, "#9fc4e8", "center")
		text("difficulty changes apply to the next game", W/2, H/2+72, 14, "#7f93b3", "center")
	}
}

func (g *game) drawTitle() {
	ctx.Set("fillStyle", "rgba(2,3,10,0.45)")
	ctx.Call("fillRect", 0, 0, W, H)

	grad := ctx.Call("createLinearGradient", 300, 0, 980, 0)
	grad.Call("addColorStop", 0, "#ffd84a")
	grad.Call("addColorStop", 0.5, "#ff6bf0")
	grad.Call("addColorStop", 1, "#39d5ff")
	setFont(84)
	ctx.Set("textAlign", "center")
	ctx.Set("fillStyle", grad)
	ctx.Call("fillText", "STARS n ASTEROIDS", W/2, 120)
	path(W/2-455, 92, g.t, 22, starShape)
	fill("#ffd84a")
	path(W/2+455, 92, -g.t, 22, starShape)
	fill("#39d5ff")
	text("35 levels  -  5 sectors  -  5 guardians", W/2, 158, 20, "#9fc4e8", "center")

	// High scores.
	text("HIGH SCORES", W/2, 215, 24, "#ffd84a", "center")
	switch {
	case len(g.scores) == 0 && g.scoresErr:
		text("score server unreachable", W/2, 260, 18, "#ff8a8a", "center")
	case len(g.scores) == 0:
		text("no scores yet - be the first!", W/2, 260, 18, "#8fa9cc", "center")
	}
	for i, r := range g.scores {
		if i >= 10 {
			break
		}
		y := 250 + float64(i)*28
		col := "#cfe6ff"
		if i+1 == g.myRank {
			col = "#7dff9a"
			if math.Mod(g.t, 0.8) < 0.4 {
				col = "#ffffff"
			}
		}
		text(strconv.Itoa(i+1)+".", W/2-245, y, 20, col, "right")
		text(r.name, W/2-230, y, 20, col, "left")
		text(commas(r.score), W/2+90, y, 20, col, "right")
		text("L"+strconv.Itoa(r.level), W/2+150, y, 16, "#8fa9cc", "right")
		text(diffShort(r.diff), W/2+165, y, 14, "#8fa9cc", "left")
	}
	if g.myRank > 10 {
		text("your rank: "+strconv.Itoa(g.myRank), W/2, 250+10*28, 18, "#7dff9a", "center")
	}

	if math.Mod(g.t, 1.1) < 0.75 {
		text("PRESS ENTER OR TAP TO START", W/2, 588, 28, "#ffffff", "center")
	}
	text("DIFFICULTY: "+difficulties[cfg.diff].name+"     -     O  or  the gear button  for settings", W/2, 616, 16, "#b78cff", "center")
	kl := func(a action) string { return keyLabel(cfg.keys[a][0]) }
	text(kl(actLeft)+" "+kl(actRight)+"  steer     "+kl(actThrust)+"  thrust     "+kl(actFire)+"  fire     "+kl(actBomb)+"  nova bomb     "+
		kl(actJump)+"  space jump     "+kl(actPause)+"  pause     "+kl(actMute)+"  mute", W/2, 644, 16, "#9fc4e8", "center")
	text("Collect stars to raise your score multiplier.  Crystal asteroids explode - keep your distance, or use them.",
		W/2, 670, 15, "#7f93b3", "center")
	text(version.Version, W-14, H-12, 13, "#5f7190", "right")
}
