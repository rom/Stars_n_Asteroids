package scores

import (
	"path/filepath"
	"testing"
)

func TestAddRankAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "scores.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		score int
		rank  int
	}{{"ann", 500, 1}, {"bob", 900, 1}, {"cy", 700, 2}, {"dee", 500, 4}} {
		rank, err := s.Add(c.name, c.score, 3)
		if err != nil || rank != c.rank {
			t.Errorf("Add(%s,%d) = %d, %v; want rank %d", c.name, c.score, rank, err, c.rank)
		}
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	top := s2.Top(3)
	if len(top) != 3 || top[0].Name != "bob" || top[1].Name != "cy" || top[2].Name != "ann" {
		t.Errorf("unexpected reloaded table: %+v", top)
	}
}

func TestValidation(t *testing.T) {
	s, _ := Open("")
	bad := []struct {
		name         string
		score, level int
	}{{"", 10, 1}, {"  \n ", 10, 1}, {"x", 0, 1}, {"x", -5, 1}, {"x", MaxScore + 1, 1}, {"x", 10, 0}, {"x", 10, 36}}
	for _, c := range bad {
		if _, err := s.Add(c.name, c.score, c.level); err == nil {
			t.Errorf("Add(%q,%d,%d) should fail", c.name, c.score, c.level)
		}
	}
	if len(s.Top(10)) != 0 {
		t.Error("invalid entries were stored")
	}
}

func TestCleanName(t *testing.T) {
	got, err := CleanName("  <b>Ace\x00\tPilot of the galaxy ")
	if err != nil || got != "bAce Pilot o" {
		t.Errorf("CleanName = %q, %v", got, err)
	}
}

func TestTrim(t *testing.T) {
	s, _ := Open("")
	for i := 1; i <= keep+20; i++ {
		s.Add("p", i, 1)
	}
	if n := len(s.Top(1000)); n != keep {
		t.Errorf("kept %d entries, want %d", n, keep)
	}
	if rank, _ := s.Add("low", 1, 1); rank != 0 {
		t.Errorf("score below the table should get rank 0, got %d", rank)
	}
}
