// Package scores implements the persistent high-score table.
package scores

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	MaxName  = 12
	MaxScore = 50_000_000
	MaxLevel = 35
	keep     = 100 // entries retained on disk
)

// Difficulties lists the accepted difficulty identifiers, easiest first.
var Difficulties = []string{"supereasy", "easy", "normal", "hard", "superhard", "insane"}

// Entry is one row of the high-score table.
type Entry struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
	Level int    `json:"level"`
	// Difficulty is one of Difficulties; empty in old files means "normal".
	Difficulty string    `json:"difficulty,omitempty"`
	Date       time.Time `json:"date"`
}

// Store is a concurrency-safe high-score table, optionally backed by a JSON file.
type Store struct {
	mu      sync.Mutex
	path    string
	entries []Entry // sorted, best first
}

// Open loads the table from path. A missing file yields an empty table.
// An empty path gives an in-memory table.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	if path == "" {
		return s, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.entries); err != nil {
		return nil, err
	}
	s.sort()
	return s, nil
}

func (s *Store) sort() {
	sort.SliceStable(s.entries, func(i, j int) bool {
		a, b := s.entries[i], s.entries[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.Date.Before(b.Date)
	})
}

// CleanName normalises a player name: printable characters only, collapsed
// whitespace, at most MaxName runes.
func CleanName(name string) (string, error) {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsSpace(r) {
			r = ' '
		}
		if unicode.IsPrint(r) && r != '<' && r != '>' {
			b.WriteRune(r)
		}
	}
	rs := []rune(strings.Join(strings.Fields(b.String()), " "))
	if len(rs) > MaxName {
		rs = rs[:MaxName]
	}
	out := strings.TrimSpace(string(rs))
	if out == "" {
		return "", errors.New("name is required")
	}
	return out, nil
}

// Add validates and inserts a score. It returns the 1-based rank of the new
// entry (0 if it did not make the retained table). An empty difficulty means
// "normal".
func (s *Store) Add(name string, score, level int, difficulty string) (int, error) {
	if difficulty == "" {
		difficulty = "normal"
	}
	if !slices.Contains(Difficulties, difficulty) {
		return 0, errors.New("unknown difficulty")
	}
	name, err := CleanName(name)
	if err != nil {
		return 0, err
	}
	if score <= 0 || score > MaxScore {
		return 0, errors.New("score out of range")
	}
	if level < 1 || level > MaxLevel {
		return 0, errors.New("level out of range")
	}
	e := Entry{Name: name, Score: score, Level: level, Difficulty: difficulty, Date: time.Now().UTC()}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	s.sort()
	if len(s.entries) > keep {
		s.entries = s.entries[:keep]
	}
	rank := 0
	for i, x := range s.entries {
		if x == e {
			rank = i + 1
			break
		}
	}
	return rank, s.save()
}

// Top returns up to n best entries.
func (s *Store) Top(n int) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n > len(s.entries) {
		n = len(s.entries)
	}
	return append([]Entry{}, s.entries[:n]...)
}

func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(s.entries, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
