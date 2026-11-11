//go:build js && wasm

package main

import (
	"math"
	"math/rand"
	"strconv"

	"stars_n_asteroids/internal/levels"
)

const (
	W, H = 1280.0, 720.0
	tau  = 2 * math.Pi

	startLives = 3
	maxLives   = 6
	maxBombs   = 3
	maxMult    = 9
	lifeEvery  = 30000
	starsPerX  = 4 // stars needed to raise the multiplier
)

type state int

const (
	stTitle state = iota
	stPlay
	stOver
)

// asteroid kinds
const (
	kRock = iota
	kIce
	kIron
	kCrystal
)

// pickup kinds
const (
	pStar = iota
	pTriple
	pRapid
	pShield
	pBomb
)

// enemy kinds
const (
	eDrone = iota
	eHunter
	eBoss
)

var astRadius = [4]float64{0, 15, 28, 48}

type ship struct {
	x, y, vx, vy, a float64
	alive, thrust   bool
	shield          bool
	inv             float64 // invulnerability left
	cool            float64 // gun cooldown
	triple, rapid   float64 // power-up time left
	respawn         float64
}

type asteroid struct {
	x, y, vx, vy float64
	a, spin, r   float64
	kind, size   int
	hp           int
	shape        []float64 // radius multiplier per vertex
	flash        float64
	dead         bool
}

type bullet struct {
	x, y, vx, vy float64
	life         float64
	enemy        bool
	dead         bool
}

type particle struct {
	x, y, vx, vy    float64
	life, max, size float64
	color           string
}

type pickup struct {
	x, y, vx, vy float64
	life         float64
	kind         int
	dead         bool
}

type enemy struct {
	x, y, vx, vy, a  float64
	kind             int
	hp, maxhp        int
	r                float64
	t                float64
	cool, cool2, cl3 float64
	flash            float64
	dead             bool
}

// shock is an expanding blast ring (crystal explosion or player bomb).
type shock struct {
	x, y, max float64
	t, dur    float64
	friendly  bool
	dmg       int
	applied   bool
}

type floater struct {
	x, y, life float64
	s, color   string
}

type bgStar struct{ x, y, z, tw float64 }

type game struct {
	st     state
	t      float64
	paused bool

	ship    ship
	asts    []*asteroid
	bullets []*bullet
	parts   []particle
	picks   []*pickup
	enemies []*enemy
	shocks  []*shock
	floats  []floater
	bg      []bgStar

	score, lives, bombs int
	mult, stars         int
	nextLife            int

	lvl      levels.Level
	reached  int     // highest level reached this run
	intro    float64 // level banner time left
	spawned  bool
	clearT   float64 // countdown after the level is cleared
	queue    []int   // enemies still to arrive
	spawnT   float64
	dying    float64 // countdown to the game-over screen
	won      bool
	overT    float64
	asked    bool // name entry already shown
	practice int  // start level from ?level=N; practice runs are not ranked

	shake, flash float64

	scores    []scoreRow
	scoresErr bool
	myRank    int
}

func newGame() *game {
	g := &game{}
	for i := 0; i < 110; i++ {
		g.bg = append(g.bg, bgStar{rand.Float64() * W, rand.Float64() * H, 0.3 + rand.Float64()*0.7, rand.Float64() * tau})
	}
	g.lvl = levels.Get(1)
	g.toTitle(false)
	return g
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func dist2(ax, ay, bx, by float64) float64 { return (ax-bx)*(ax-bx) + (ay-by)*(ay-by) }

func wrap(x, y *float64, m float64) {
	if *x < -m {
		*x += W + 2*m
	} else if *x > W+m {
		*x -= W + 2*m
	}
	if *y < -m {
		*y += H + 2*m
	} else if *y > H+m {
		*y -= H + 2*m
	}
}

// ------------------------------------------------------------ flow control

func (g *game) clearWorld() {
	g.asts, g.bullets, g.picks, g.enemies, g.shocks, g.floats = nil, nil, nil, nil, nil, nil
	g.parts = g.parts[:0]
	g.queue = nil
}

func (g *game) toTitle(refresh bool) {
	g.st = stTitle
	g.paused = false
	g.clearWorld()
	g.ship.alive = false
	g.lvl = levels.Get(1 + rand.Intn(levels.Count))
	for i := 0; i < 9; i++ {
		kind := []int{kRock, kRock, kIce, kIron, kCrystal}[rand.Intn(5)]
		g.asts = append(g.asts, newAsteroid(kind, 1+rand.Intn(3), rand.Float64()*W, rand.Float64()*H, 0.6))
	}
	snd.setThrust(false)
	snd.setMusic(musTitle, 0)
	if refresh {
		fetchScores()
	}
}

func (g *game) start() {
	g.clearWorld()
	g.st = stPlay
	g.score, g.lives, g.bombs = 0, startLives, 1
	g.mult, g.stars = 1, 0
	g.nextLife = lifeEvery
	g.won, g.asked = false, false
	g.dying, g.overT, g.myRank = 0, 0, 0
	g.spawnShip()
	if g.practice > 1 {
		g.startLevel(g.practice)
	} else {
		g.startLevel(1)
	}
}

func (g *game) startLevel(n int) {
	g.lvl = levels.Get(n)
	g.reached = n
	g.intro = 3
	g.spawned = false
	g.clearT = 0
	g.bullets = nil
	if g.lvl.Boss {
		snd.setMusic(musBoss, n)
		snd.bossWarn()
	} else {
		snd.setMusic(musLevel, n)
	}
}

func (g *game) spawnShip() {
	g.ship = ship{x: W / 2, y: H / 2, a: -math.Pi / 2, alive: true, inv: 3}
}

// edgePoint picks a point on the border of the playfield away from the ship.
func (g *game) edgePoint() (float64, float64) {
	for i := 0; ; i++ {
		var x, y float64
		if rand.Intn(2) == 0 {
			x, y = rand.Float64()*W, float64(rand.Intn(2))*H
		} else {
			x, y = float64(rand.Intn(2))*W, rand.Float64()*H
		}
		if i > 20 || dist2(x, y, g.ship.x, g.ship.y) > 300*300 {
			return x, y
		}
	}
}

func (g *game) populate() {
	g.spawned = true
	l := g.lvl
	add := func(kind, n int) {
		for i := 0; i < n; i++ {
			x, y := g.edgePoint()
			g.asts = append(g.asts, newAsteroid(kind, 3, x, y, l.Speed))
		}
	}
	add(kRock, l.Rocks)
	add(kIce, l.Ice)
	add(kIron, l.Iron)
	add(kCrystal, l.Crystal)

	g.queue = nil
	for i := 0; i < l.Drones; i++ {
		g.queue = append(g.queue, eDrone)
	}
	for i := 0; i < l.Hunters; i++ {
		g.queue = append(g.queue, eHunter)
	}
	rand.Shuffle(len(g.queue), func(i, j int) { g.queue[i], g.queue[j] = g.queue[j], g.queue[i] })
	g.spawnT = l.SpawnEvery * 0.6

	if l.Boss {
		g.enemies = append(g.enemies, &enemy{kind: eBoss, x: W / 2, y: -90, r: 58, hp: l.BossHP, maxhp: l.BossHP,
			cool: 3, cool2: 5, cl3: 7})
	}
}

func newAsteroid(kind, size int, x, y, speed float64) *asteroid {
	a := &asteroid{kind: kind, size: size, x: x, y: y, hp: 1}
	a.r = astRadius[size]
	v := (40 + rand.Float64()*40) * (1 + float64(3-size)*0.55)
	n := 9 + rand.Intn(4)
	lo, hi := 0.74, 1.12
	switch kind {
	case kIce:
		v *= 1.5
		n = 6 + rand.Intn(2)
		lo, hi = 0.6, 1.18
	case kIron:
		v *= 0.7
		a.hp = size + 1
		lo, hi = 0.86, 1.06
	case kCrystal:
		v *= 0.8
		a.size = 2
		a.r = 30
		n = 8
	}
	for i := 0; i < n; i++ {
		m := lo + rand.Float64()*(hi-lo)
		if kind == kCrystal {
			m = 1.0
			if i%2 == 1 {
				m = 0.62
			}
		}
		a.shape = append(a.shape, m)
	}
	dir := rand.Float64() * tau
	a.vx, a.vy = math.Cos(dir)*v*speed, math.Sin(dir)*v*speed
	a.a = rand.Float64() * tau
	a.spin = (rand.Float64() - 0.5) * 1.6
	return a
}

func (g *game) autoPause() {
	if g.st == stPlay && g.dying == 0 {
		g.paused = true
		snd.setThrust(false)
	}
}

// ----------------------------------------------------------------- effects

func (g *game) burst(x, y float64, n int, speed, life, size float64, color string) {
	if len(g.parts) > 450 {
		n = n/4 + 1
	}
	for i := 0; i < n; i++ {
		d := rand.Float64() * tau
		v := speed * (0.25 + rand.Float64()*0.75)
		l := life * (0.5 + rand.Float64()*0.5)
		g.parts = append(g.parts, particle{x, y, math.Cos(d) * v, math.Sin(d) * v, l, l, size * (0.6 + rand.Float64()*0.8), color})
	}
}

func (g *game) say(x, y float64, s, color string) {
	g.floats = append(g.floats, floater{x, y, 1.1, s, color})
}

func (g *game) addScore(pts int, x, y float64) {
	pts *= g.mult
	g.score += pts
	g.say(x, y, "+"+strconv.Itoa(pts), "#ffffff")
	for g.score >= g.nextLife {
		g.nextLife += lifeEvery
		if g.lives < maxLives {
			g.lives++
			g.say(g.ship.x, g.ship.y-40, "EXTRA SHIP", "#7dff9a")
			snd.extraLife()
		}
	}
}

// ---------------------------------------------------------------- damage

var astColor = [4]string{"#c9a98a", "#b9ecff", "#c5d0de", "#ff6bf0"}
var astPoints = [4]int{20, 30, 45, 150}

func (g *game) hitAsteroid(a *asteroid, dmg int) {
	if a.dead {
		return
	}
	a.hp -= dmg
	if a.hp > 0 {
		a.flash = 0.12
		g.burst(a.x, a.y, 5, 160, 0.3, 2, "#ffe9a8")
		snd.clang()
		return
	}
	a.dead = true
	if g.st != stPlay {
		return
	}
	g.addScore(astPoints[a.kind]*(4-a.size), a.x, a.y)
	g.burst(a.x, a.y, 8+a.size*6, 90+float64(a.size)*40, 0.8, 3, astColor[a.kind])
	g.shake = math.Max(g.shake, float64(a.size)*2.5)

	switch a.kind {
	case kCrystal:
		g.shocks = append(g.shocks, &shock{x: a.x, y: a.y, max: 150, dur: 0.45, dmg: 2})
		g.burst(a.x, a.y, 30, 320, 0.6, 3, "#ffffff")
		snd.boom(3)
		snd.shatter()
	case kIce:
		snd.shatter()
	default:
		snd.boom(a.size)
	}

	if a.size > 1 && a.kind != kCrystal {
		n := 2
		if a.kind == kIce {
			n = 3
		}
		for i := 0; i < n; i++ {
			c := newAsteroid(a.kind, a.size-1, a.x+(rand.Float64()-0.5)*a.r, a.y+(rand.Float64()-0.5)*a.r, g.lvl.Speed)
			c.vx += a.vx * 0.4
			c.vy += a.vy * 0.4
			g.asts = append(g.asts, c)
		}
	}

	// Loot.
	p := 0.3
	if a.size == 1 {
		p = 0.45
	}
	if a.kind == kCrystal {
		p = 1
	}
	if rand.Float64() < p {
		g.drop(pStar, a.x, a.y)
	}
	if rand.Float64() < 0.035 {
		g.drop(pTriple+rand.Intn(4), a.x, a.y)
	}
}

func (g *game) hitEnemy(e *enemy, dmg int) {
	if e.dead {
		return
	}
	e.hp -= dmg
	e.flash = 0.1
	if e.hp > 0 {
		g.burst(e.x, e.y, 4, 150, 0.3, 2, "#ffb0a0")
		snd.clang()
		return
	}
	e.dead = true
	switch e.kind {
	case eDrone:
		g.addScore(250, e.x, e.y)
		if rand.Float64() < 0.7 {
			g.drop(pTriple+rand.Intn(4), e.x, e.y)
		}
	case eHunter:
		g.addScore(400, e.x, e.y)
		if rand.Float64() < 0.45 {
			g.drop(pTriple+rand.Intn(4), e.x, e.y)
		}
		g.drop(pStar, e.x, e.y)
	case eBoss:
		g.addScore(5000*(g.lvl.Sector+1), e.x, e.y)
		for i := 0; i < 12; i++ {
			g.drop(pStar, e.x+(rand.Float64()-0.5)*120, e.y+(rand.Float64()-0.5)*120)
		}
		g.drop(pShield, e.x, e.y)
		g.burst(e.x, e.y, 90, 420, 1.6, 4, "#ffd84a")
		g.burst(e.x, e.y, 60, 260, 1.2, 4, "#ff6bf0")
		g.shake = 26
		g.flash = 0.5
		snd.boom(4)
		snd.bomb()
		// The guardian's escorts and shots die with it.
		for _, b := range g.bullets {
			if b.enemy {
				b.dead = true
			}
		}
		return
	}
	g.burst(e.x, e.y, 26, 240, 0.8, 3, "#ff8a5c")
	g.shake = math.Max(g.shake, 7)
	snd.boom(2)
}

func (g *game) drop(kind int, x, y float64) {
	d := rand.Float64() * tau
	life := 9.0
	if kind != pStar {
		life = 12
	}
	g.picks = append(g.picks, &pickup{x: x, y: y, vx: math.Cos(d) * 35, vy: math.Sin(d) * 35, life: life, kind: kind})
}

func (g *game) killShip() {
	s := &g.ship
	if !s.alive || s.inv > 0 || g.clearT > 0 && g.lvl.Num == levels.Count {
		return
	}
	if s.shield {
		s.shield = false
		s.inv = 1.2
		g.burst(s.x, s.y, 24, 260, 0.5, 3, "#7df0ff")
		snd.shieldHit()
		return
	}
	s.alive = false
	s.respawn = 2
	g.lives--
	g.mult, g.stars = 1, 0
	g.burst(s.x, s.y, 50, 300, 1.3, 3, "#39d5ff")
	g.burst(s.x, s.y, 30, 180, 1.0, 3, "#ffffff")
	g.shake = 18
	snd.die()
	snd.setThrust(false)
	if g.lives <= 0 {
		g.dying = 2.4
	}
}

func (g *game) detonateBomb() {
	g.bombs--
	g.shocks = append(g.shocks, &shock{x: g.ship.x, y: g.ship.y, max: 380, dur: 0.6, dmg: 4, friendly: true})
	g.flash = 0.35
	g.shake = 16
	snd.bomb()
}

func (g *game) applyShock(s *shock) {
	s.applied = true
	for _, a := range g.asts {
		if dist2(a.x, a.y, s.x, s.y) < (s.max+a.r)*(s.max+a.r) {
			g.hitAsteroid(a, s.dmg)
		}
	}
	for _, e := range g.enemies {
		if dist2(e.x, e.y, s.x, s.y) < (s.max+e.r)*(s.max+e.r) {
			d := s.dmg
			if e.kind == eBoss && s.friendly {
				d = 10
			}
			g.hitEnemy(e, d)
		}
	}
	if s.friendly {
		for _, b := range g.bullets {
			if b.enemy && dist2(b.x, b.y, s.x, s.y) < s.max*s.max {
				b.dead = true
			}
		}
	} else if dist2(g.ship.x, g.ship.y, s.x, s.y) < s.max*s.max*0.6 {
		g.killShip()
	}
}

// ----------------------------------------------------------------- update

func (g *game) update(dt float64) {
	g.t += dt
	for i := range g.bg {
		b := &g.bg[i]
		b.x -= b.z * 9 * dt
		if b.x < 0 {
			b.x += W
		}
	}
	if hit("KeyM") {
		snd.toggleMute()
	}

	switch g.st {
	case stTitle:
		g.updateAsteroids(dt)
		if hit("Enter", "Space", "Tap") {
			g.start()
		}
	case stPlay:
		if g.dying == 0 && hit("KeyP", "Escape") {
			g.paused = !g.paused
			snd.setThrust(false)
		} else if g.paused && hit("Tap", "Enter") {
			g.paused = false
		}
		if !g.paused {
			g.play(dt)
		}
	case stOver:
		g.overT += dt
		g.updateAsteroids(dt)
		g.updateParticles(dt)
		if g.overT > 1.6 && !g.asked {
			g.asked = true
			if g.score > 0 && g.practice <= 1 {
				title := "GAME OVER"
				if g.won {
					title = "GALAXY CLEARED!"
				}
				showEntry(title, "Score "+commas(g.score)+" - enter your name, pilot")
			}
		} else if g.asked && (g.score == 0 || g.practice > 1) && hit("Enter", "Space", "Tap") {
			g.toTitle(true)
		}
	}
}

func (g *game) play(dt float64) {
	l := g.lvl
	g.shake = math.Max(0, g.shake-dt*40)
	g.flash = math.Max(0, g.flash-dt)

	if g.intro > 0 {
		g.intro -= dt
		if g.intro <= 1.4 && !g.spawned {
			g.populate()
		}
	}

	g.updateShip(dt)
	g.updateAsteroids(dt)
	g.updateEnemies(dt)

	// Enemy arrivals.
	if g.spawned && len(g.queue) > 0 && g.ship.alive {
		if len(g.asts) == 0 && len(g.enemies) == 0 {
			g.spawnT = math.Min(g.spawnT, 1.2)
		}
		g.spawnT -= dt
		if g.spawnT <= 0 {
			g.spawnT = l.SpawnEvery
			g.spawnEnemy(g.queue[0])
			g.queue = g.queue[1:]
		}
	}

	// Bullets.
	for _, b := range g.bullets {
		b.x += b.vx * dt
		b.y += b.vy * dt
		b.life -= dt
		if b.enemy {
			if b.life <= 0 || b.x < -20 || b.x > W+20 || b.y < -20 || b.y > H+20 {
				b.dead = true
			} else if g.ship.alive && dist2(b.x, b.y, g.ship.x, g.ship.y) < 13*13 {
				b.dead = true
				g.killShip()
			}
			continue
		}
		wrap(&b.x, &b.y, 4)
		if b.life <= 0 {
			b.dead = true
			continue
		}
		for _, a := range g.asts {
			if !a.dead && dist2(b.x, b.y, a.x, a.y) < a.r*a.r {
				b.dead = true
				g.hitAsteroid(a, 1)
				break
			}
		}
		if b.dead {
			continue
		}
		for _, e := range g.enemies {
			if !e.dead && dist2(b.x, b.y, e.x, e.y) < e.r*e.r {
				b.dead = true
				g.hitEnemy(e, 1)
				break
			}
		}
	}

	// Shockwaves.
	for _, s := range g.shocks {
		s.t += dt
		if !s.applied && s.t >= s.dur*0.3 {
			g.applyShock(s)
		}
	}

	// Ship collisions.
	if s := &g.ship; s.alive {
		for _, a := range g.asts {
			if !a.dead && dist2(s.x, s.y, a.x, a.y) < (a.r*0.88+11)*(a.r*0.88+11) {
				if s.inv <= 0 {
					g.killShip()
					if a.kind != kIron {
						g.hitAsteroid(a, 1)
					}
				}
				break
			}
		}
		for _, e := range g.enemies {
			if !e.dead && dist2(s.x, s.y, e.x, e.y) < (e.r+10)*(e.r+10) && s.inv <= 0 {
				g.killShip()
				if e.kind != eBoss {
					g.hitEnemy(e, 2)
				}
				break
			}
		}
	}

	g.updatePickups(dt)
	g.updateParticles(dt)
	for i := range g.floats {
		g.floats[i].life -= dt
		g.floats[i].y -= 28 * dt
	}
	g.compact()

	// Level clear.
	if g.spawned && g.clearT == 0 && g.dying == 0 && len(g.asts) == 0 && len(g.enemies) == 0 && len(g.queue) == 0 {
		g.clearT = 3
		bonus := 250 * l.Num
		g.score += bonus
		g.say(W/2, H/2+70, "LEVEL BONUS +"+commas(bonus), "#ffd84a")
		if l.Num == levels.Count {
			snd.win()
			snd.setMusic(musQuiet, 0)
		} else {
			snd.levelUp()
		}
	}
	if g.clearT > 0 {
		g.clearT -= dt
		if g.clearT <= 0 {
			g.clearT = 0
			if l.Num == levels.Count {
				g.won = true
				g.score += g.lives * 5000
				g.gameOver()
			} else {
				g.startLevel(l.Num + 1)
			}
		}
	}

	if g.dying > 0 {
		g.dying -= dt
		if g.dying <= 0 {
			g.gameOver()
		}
	}
}

func (g *game) gameOver() {
	g.st = stOver
	g.overT = 0
	g.dying = 0
	g.asked = false
	g.bullets = nil
	snd.setThrust(false)
	if !g.won {
		snd.setMusic(musQuiet, 0)
		snd.gameOver()
	}
}

func (g *game) updateShip(dt float64) {
	s := &g.ship
	if !s.alive {
		if g.lives > 0 {
			s.respawn -= dt
			if s.respawn <= 0 {
				g.spawnShip()
			}
		}
		return
	}
	if down("ArrowLeft", "KeyA") {
		s.a -= 4.4 * dt
	}
	if down("ArrowRight", "KeyD") {
		s.a += 4.4 * dt
	}
	s.thrust = down("ArrowUp", "KeyW")
	snd.setThrust(s.thrust)
	cs, sn := math.Cos(s.a), math.Sin(s.a)
	if s.thrust {
		s.vx += cs * 430 * dt
		s.vy += sn * 430 * dt
		if rand.Float64() < dt*70 {
			d := s.a + math.Pi + (rand.Float64()-0.5)*0.5
			v := 140 + rand.Float64()*120
			g.parts = append(g.parts, particle{s.x - cs*13, s.y - sn*13, s.vx + math.Cos(d)*v, s.vy + math.Sin(d)*v, 0.4, 0.4, 3, "#ffb347"})
		}
	}
	drag := math.Exp(-0.35 * dt)
	s.vx *= drag
	s.vy *= drag
	if sp := math.Hypot(s.vx, s.vy); sp > 540 {
		s.vx, s.vy = s.vx/sp*540, s.vy/sp*540
	}
	s.x += s.vx * dt
	s.y += s.vy * dt
	wrap(&s.x, &s.y, 16)

	s.inv = math.Max(0, s.inv-dt)
	s.cool -= dt
	s.triple = math.Max(0, s.triple-dt)
	s.rapid = math.Max(0, s.rapid-dt)

	if down("Space", "KeyK", "ControlLeft") && s.cool <= 0 {
		s.cool = 0.21
		if s.rapid > 0 {
			s.cool = 0.1
		}
		angles := []float64{0}
		if s.triple > 0 {
			angles = []float64{-0.2, 0, 0.2}
		}
		for _, da := range angles {
			c2, s2 := math.Cos(s.a+da), math.Sin(s.a+da)
			g.bullets = append(g.bullets, &bullet{x: s.x + c2*18, y: s.y + s2*18, vx: s.vx*0.5 + c2*720, vy: s.vy*0.5 + s2*720, life: 0.85})
		}
		snd.shoot()
	}
	if g.bombs > 0 && hit("KeyB", "KeyX", "ShiftLeft", "ShiftRight") {
		g.detonateBomb()
	}
}

func (g *game) updateAsteroids(dt float64) {
	for _, a := range g.asts {
		a.x += a.vx * dt
		a.y += a.vy * dt
		a.a += a.spin * dt
		a.flash -= dt
		wrap(&a.x, &a.y, a.r)
	}
}

func (g *game) spawnEnemy(kind int) {
	l := g.lvl
	e := &enemy{kind: kind}
	switch kind {
	case eDrone:
		e.r, e.hp = 16, 2
		e.y = 80 + rand.Float64()*(H-160)
		e.vx = (90 + rand.Float64()*40) * l.Speed
		e.x = -20
		if rand.Intn(2) == 0 {
			e.x, e.vx = W+20, -e.vx
		}
		e.cool = 1.5
	case eHunter:
		e.r, e.hp = 15, 3
		e.x, e.y = g.edgePoint()
	}
	e.maxhp = e.hp
	g.enemies = append(g.enemies, e)
	snd.enemyShoot()
}

func (g *game) enemyFire(x, y, ang, speed float64) {
	g.bullets = append(g.bullets, &bullet{x: x, y: y, vx: math.Cos(ang) * speed, vy: math.Sin(ang) * speed, life: 5, enemy: true})
}

func (g *game) updateEnemies(dt float64) {
	l := g.lvl
	s := &g.ship
	bs := 230 + float64(l.Num)*3.5 // enemy bullet speed
	for _, e := range g.enemies {
		e.t += dt
		e.flash -= dt
		aim := math.Atan2(s.y-e.y, s.x-e.x)
		switch e.kind {
		case eDrone:
			e.vy = 75 * math.Cos(e.t*1.7)
			e.cool -= dt
			if e.cool <= 0 && s.alive {
				e.cool = math.Max(1.1, 2.6-float64(l.Num)*0.04)
				spread := math.Max(0.06, 0.5-float64(l.Num)*0.013)
				g.enemyFire(e.x, e.y, aim+(rand.Float64()-0.5)*spread, bs)
				snd.enemyShoot()
			}
		case eHunter:
			if s.alive {
				acc := 190 * l.Speed
				e.vx += math.Cos(aim) * acc * dt
				e.vy += math.Sin(aim) * acc * dt
			}
			top := 150 * l.Speed
			if sp := math.Hypot(e.vx, e.vy); sp > top {
				e.vx, e.vy = e.vx/sp*top, e.vy/sp*top
			}
			e.a = math.Atan2(e.vy, e.vx)
		case eBoss:
			tier := l.Sector
			tx := W/2 + 420*math.Sin(e.t*0.42)
			ty := 210 + 140*math.Sin(e.t*0.67)
			k := math.Min(1, dt*1.4)
			e.x += (tx - e.x) * k
			e.y += (ty - e.y) * k
			e.a += dt * 0.6
			if !s.alive {
				break
			}
			e.cool -= dt
			e.cool2 -= dt
			e.cl3 -= dt
			if e.cool <= 0 {
				e.cool = 1.5 - float64(tier)*0.15
				for i := -1; i <= 1; i++ {
					g.enemyFire(e.x, e.y, aim+float64(i)*0.22, bs)
				}
				snd.enemyShoot()
			}
			if e.cool2 <= 0 {
				e.cool2 = 4.5 - float64(tier)*0.3
				n := 10 + tier*3
				off := rand.Float64() * tau
				for i := 0; i < n; i++ {
					g.enemyFire(e.x, e.y, off+float64(i)*tau/float64(n), bs*0.7)
				}
				snd.boom(1)
			}
			if e.cl3 <= 0 {
				e.cl3 = 7 - float64(tier)*0.6
				if len(g.asts) < 9 {
					kind := kRock
					if tier >= 2 && rand.Intn(2) == 0 {
						kind = kIce
					}
					g.asts = append(g.asts, newAsteroid(kind, 2, e.x, e.y, l.Speed))
				}
			}
			continue
		}
		e.x += e.vx * dt
		e.y += e.vy * dt
		wrap(&e.x, &e.y, e.r+6)
	}
}

func (g *game) updatePickups(dt float64) {
	s := &g.ship
	for _, p := range g.picks {
		p.life -= dt
		if p.life <= 0 {
			p.dead = true
			continue
		}
		if s.alive {
			d2 := dist2(p.x, p.y, s.x, s.y)
			if d2 < 150*150 { // magnet
				d := math.Sqrt(d2) + 0.01
				p.vx += (s.x - p.x) / d * 900 * dt
				p.vy += (s.y - p.y) / d * 900 * dt
			}
			if d2 < 26*26 {
				p.dead = true
				g.collect(p)
				continue
			}
		}
		f := math.Exp(-1.2 * dt)
		p.vx *= f
		p.vy *= f
		p.x += p.vx * dt
		p.y += p.vy * dt
		wrap(&p.x, &p.y, 12)
	}
}

func (g *game) collect(p *pickup) {
	s := &g.ship
	switch p.kind {
	case pStar:
		g.stars++
		if g.stars%starsPerX == 0 && g.mult < maxMult {
			g.mult++
			g.say(s.x, s.y-34, "MULTIPLIER x"+strconv.Itoa(g.mult), "#ffd84a")
		}
		g.addScore(50, p.x, p.y)
		g.burst(p.x, p.y, 8, 120, 0.4, 2, "#ffd84a")
		snd.star(g.mult)
		return
	case pTriple:
		s.triple = 14
		g.say(s.x, s.y-34, "TRIPLE SHOT", "#7dff9a")
	case pRapid:
		s.rapid = 14
		g.say(s.x, s.y-34, "RAPID FIRE", "#ff9d5c")
	case pShield:
		s.shield = true
		g.say(s.x, s.y-34, "SHIELD", "#7df0ff")
	case pBomb:
		if g.bombs < maxBombs {
			g.bombs++
		}
		g.say(s.x, s.y-34, "NOVA BOMB", "#ff6bf0")
	}
	g.addScore(100, p.x, p.y)
	snd.powerup()
}

func (g *game) updateParticles(dt float64) {
	out := g.parts[:0]
	for _, p := range g.parts {
		p.life -= dt
		if p.life <= 0 {
			continue
		}
		p.x += p.vx * dt
		p.y += p.vy * dt
		out = append(out, p)
	}
	g.parts = out
}

func keep[T any](in []*T, dead func(*T) bool) []*T {
	out := in[:0]
	for _, v := range in {
		if !dead(v) {
			out = append(out, v)
		}
	}
	for i := len(out); i < len(in); i++ {
		in[i] = nil
	}
	return out
}

func (g *game) compact() {
	g.asts = keep(g.asts, func(a *asteroid) bool { return a.dead })
	g.bullets = keep(g.bullets, func(b *bullet) bool { return b.dead })
	g.picks = keep(g.picks, func(p *pickup) bool { return p.dead })
	g.enemies = keep(g.enemies, func(e *enemy) bool { return e.dead })
	g.shocks = keep(g.shocks, func(s *shock) bool { return s.t >= s.dur })
	fl := g.floats[:0]
	for _, f := range g.floats {
		if f.life > 0 {
			fl = append(fl, f)
		}
	}
	g.floats = fl
}

func commas(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
