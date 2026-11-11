// Command server is the web back-end of Stars n Asteroids. It serves the
// browser client and the high-score API.
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"stars_n_asteroids/internal/scores"
	"stars_n_asteroids/internal/version"
	"stars_n_asteroids/web"
)

const topN = 10

type scoreReq struct {
	Name       string `json:"name"`
	Score      int    `json:"score"`
	Level      int    `json:"level"`
	Difficulty string `json:"difficulty"`
}

type scoreResp struct {
	Rank   int            `json:"rank,omitempty"`
	Scores []scores.Entry `json:"scores"`
	Error  string         `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func scoresHandler(store *scores.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, scoreResp{Scores: store.Top(topN)})
		case http.MethodPost:
			var req scoreReq
			body := http.MaxBytesReader(w, r.Body, 1024)
			if err := json.NewDecoder(body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, scoreResp{Scores: store.Top(topN), Error: "invalid JSON"})
				return
			}
			rank, err := store.Add(req.Name, req.Score, req.Level, req.Difficulty)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, scoreResp{Scores: store.Top(topN), Error: err.Error()})
				return
			}
			writeJSON(w, http.StatusCreated, scoreResp{Rank: rank, Scores: store.Top(topN)})
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// staticHandler serves files from fsys, gzip-compressing them once on demand
// (the WebAssembly binary shrinks to roughly a third).
func staticHandler(fsys fs.FS) http.HandlerFunc {
	type cached struct{ raw, gz []byte }
	var mu sync.Mutex
	cache := map[string]*cached{}

	load := func(name string) (*cached, error) {
		mu.Lock()
		defer mu.Unlock()
		if c, ok := cache[name]; ok {
			return c, nil
		}
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		zw.Write(raw)
		zw.Close()
		c := &cached{raw: raw, gz: buf.Bytes()}
		cache[name] = c
		return c, nil
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		c, err := load(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ctype := mime.TypeByExtension(path.Ext(name))
		if strings.HasSuffix(name, ".wasm") {
			ctype = "application/wasm"
		}
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Vary", "Accept-Encoding")
		body := c.raw
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			body = c.gz
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(body))
	}
}

func newMux(store *scores.Store, static fs.FS) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/scores", scoresHandler(store))
	mux.HandleFunc("/", staticHandler(static))
	return mux
}

func main() {
	defPort := 8080
	if p := os.Getenv("PORT"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			log.Fatalf("invalid PORT environment variable %q", p)
		}
		defPort = n
	}
	port := flag.Int("port", defPort, "TCP port to listen on (default from $PORT, else 8080)")
	host := flag.String("host", "", "interface to listen on (default: all interfaces)")
	data := flag.String("data", "data/scores.json", "high-score file")
	dir := flag.String("web", "", "serve client files from this directory instead of the embedded copy (development)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("stars_n_asteroids", version.Version)
		return
	}

	store, err := scores.Open(*data)
	if err != nil {
		log.Fatalf("opening scores: %v", err)
	}
	static := web.Static()
	if *dir != "" {
		static = os.DirFS(*dir)
	}
	if _, err := fs.Stat(static, "game.wasm"); err != nil {
		log.Print("warning: game.wasm not found - run `make wasm` and rebuild")
	}

	if *port < 1 || *port > 65535 {
		log.Fatalf("invalid port %d: must be between 1 and 65535", *port)
	}
	addr := net.JoinHostPort(*host, strconv.Itoa(*port))
	shown := *host
	if shown == "" {
		shown = "localhost"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           newMux(store, static),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("Stars n Asteroids %s listening on http://%s", version.Version, net.JoinHostPort(shown, strconv.Itoa(*port)))
	log.Fatal(srv.ListenAndServe())
}
