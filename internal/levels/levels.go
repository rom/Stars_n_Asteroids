// Package levels defines the 35 levels of Stars n Asteroids.
//
// The campaign is split into five sectors of seven levels. The last level of
// every sector is a boss fight.
package levels

// Count is the number of levels in the campaign.
const Count = 35

// PerSector is the number of levels in each sector.
const PerSector = 7

// Level describes what the player faces on a single level.
type Level struct {
	Num        int     // 1-based level number
	Name       string  // unique level name
	Sector     int     // 0-based sector index
	SectorName string  // name of the sector
	Rocks      int     // plain asteroids: split in two
	Ice        int     // fast asteroids: shatter into three shards
	Iron       int     // armoured asteroids: take several hits
	Crystal    int     // volatile asteroids: explode with a shockwave
	Speed      float64 // speed multiplier for everything hostile
	Drones     int     // shooting drones that arrive during the level
	Hunters    int     // ramming hunters that arrive during the level
	SpawnEvery float64 // seconds between enemy arrivals
	Boss       bool    // sector guardian present
	BossHP     int     // guardian hit points
	Hue        float64 // background colour hue (degrees)
}

var sectorNames = [Count / PerSector]string{
	"Kuiper Drift", "Cinder Belt", "Glass Nebula", "Iron Graveyard", "Event Horizon",
}

var sectorHues = [Count / PerSector]float64{215, 18, 165, 275, 340}

var names = [Count]string{
	// Kuiper Drift
	"First Light", "Loose Gravel", "Cold Shoulder", "Spark Plug", "Tumbling Dice", "Heavy Weather", "Warden of the Drift",
	// Cinder Belt
	"Ember Rain", "Slag Heap", "Hot Pursuit", "Kiln", "Ash and Echo", "Furnace Door", "The Cinder Queen",
	// Glass Nebula
	"Shard Garden", "Prism Break", "Thin Ice", "Chandelier", "Hall of Mirrors", "Splinter Storm", "The Glass Eye",
	// Iron Graveyard
	"Rust Tide", "Anvil Chorus", "Scrapyard Waltz", "Hull Breach", "Magnet Row", "Dead Reckoning", "The Foundry Core",
	// Event Horizon
	"Long Fall", "Tidal Lock", "Red Shift", "No Return", "Spaghetti Line", "Last Orbit", "Singularity",
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Get returns level n (1..Count). Out-of-range values are clamped.
func Get(n int) Level {
	n = clamp(n, 1, Count)
	s := (n - 1) / PerSector // sector
	p := (n - 1) % PerSector // position within sector

	l := Level{
		Num:        n,
		Name:       names[n-1],
		Sector:     s,
		SectorName: sectorNames[s],
		Speed:      1 + 0.025*float64(n-1),
		Hue:        sectorHues[s] + float64(p)*5,
		Boss:       p == PerSector-1,
	}

	l.Rocks = 4 + s/2 + p/3
	if n >= 3 {
		l.Ice = clamp((n-1)/6+p%2, 1, 3)
	}
	if n >= 4 {
		l.Crystal = clamp((n+2)/9+(p+1)%2, 1, 3)
	}
	if n >= 6 {
		l.Iron = clamp((n-2)/7+p%2, 1, 3)
	}
	if n >= 2 {
		l.Drones = 1 + n/6
	}
	if n >= 10 {
		l.Hunters = (n - 3) / 7
	}
	l.SpawnEvery = 16 - 0.25*float64(n)

	if l.Boss {
		// The guardian is the main event: thin out the rubble around it.
		l.Rocks = 2 + s/2
		l.Ice /= 2
		l.Iron /= 2
		l.Crystal = 1
		l.Drones = s
		l.Hunters = s / 2
		l.BossHP = 40 + 25*s
	}
	return l
}

// All returns every level in order.
func All() []Level {
	out := make([]Level, Count)
	for i := range out {
		out[i] = Get(i + 1)
	}
	return out
}
