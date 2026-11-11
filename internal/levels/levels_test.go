package levels

import "testing"

func TestCampaign(t *testing.T) {
	all := All()
	if len(all) != 35 {
		t.Fatalf("want 35 levels, got %d", len(all))
	}
	seen := map[string]bool{}
	bosses := 0
	for i, l := range all {
		if l.Num != i+1 {
			t.Errorf("level %d has Num %d", i+1, l.Num)
		}
		if l.Name == "" || seen[l.Name] {
			t.Errorf("level %d: empty or duplicate name %q", l.Num, l.Name)
		}
		seen[l.Name] = true
		if l.Rocks+l.Ice+l.Iron+l.Crystal < 3 {
			t.Errorf("level %d has too few asteroids", l.Num)
		}
		if l.Boss != (l.Num%7 == 0) {
			t.Errorf("level %d: unexpected Boss=%v", l.Num, l.Boss)
		}
		if l.Boss {
			bosses++
			if l.BossHP <= 0 {
				t.Errorf("level %d: boss without hit points", l.Num)
			}
		}
		if l.SpawnEvery < 5 {
			t.Errorf("level %d: spawn interval too short: %v", l.Num, l.SpawnEvery)
		}
		if i > 0 && l.Speed <= all[i-1].Speed {
			t.Errorf("level %d is not faster than the previous one", l.Num)
		}
	}
	if bosses != 5 {
		t.Errorf("want 5 bosses, got %d", bosses)
	}
	if !all[34].Boss {
		t.Error("final level must be a boss level")
	}
}

func TestGetClamps(t *testing.T) {
	if Get(0).Num != 1 || Get(99).Num != Count {
		t.Error("Get should clamp out-of-range level numbers")
	}
}
