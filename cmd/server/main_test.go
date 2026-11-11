package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"stars_n_asteroids/internal/scores"
)

func TestAPI(t *testing.T) {
	store, _ := scores.Open("")
	fsys := fstest.MapFS{"index.html": {Data: []byte("<html>hi</html>")}, "game.wasm": {Data: []byte("\x00asm")}}
	srv := httptest.NewServer(newMux(store, fsys))
	defer srv.Close()

	post := func(body string) (int, scoreResp) {
		res, err := http.Post(srv.URL+"/api/scores", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out scoreResp
		json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}

	if code, out := post(`{"name":"ace","score":1200,"level":4}`); code != 201 || out.Rank != 1 || len(out.Scores) != 1 {
		t.Errorf("post: %d %+v", code, out)
	}
	if code, out := post(`{"name":"zed","score":9000,"level":9,"difficulty":"hard"}`); code != 201 || out.Rank != 1 || out.Scores[1].Name != "ace" || out.Scores[0].Difficulty != "hard" {
		t.Errorf("post: %d %+v", code, out)
	}
	for _, bad := range []string{`{`, `{"name":"","score":5,"level":1}`, `{"name":"x","score":-1,"level":1}`, `{"name":"x","score":5,"level":77}`, `{"name":"x","score":5,"level":1,"difficulty":"nope"}`} {
		if code, _ := post(bad); code != 400 {
			t.Errorf("post %s: got %d, want 400", bad, code)
		}
	}

	res, _ := http.Get(srv.URL + "/api/scores")
	var list scoreResp
	json.NewDecoder(res.Body).Decode(&list)
	res.Body.Close()
	if len(list.Scores) != 2 || list.Scores[0].Name != "zed" {
		t.Errorf("get: %+v", list)
	}

	res, _ = http.Get(srv.URL + "/")
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Errorf("index: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	res.Body.Close()
	res, _ = http.Get(srv.URL + "/game.wasm")
	if res.Header.Get("Content-Type") != "application/wasm" {
		t.Errorf("wasm content type: %s", res.Header.Get("Content-Type"))
	}
	res.Body.Close()
	res, _ = http.Get(srv.URL + "/../../etc/passwd")
	if res.StatusCode != 404 {
		t.Errorf("traversal: %d", res.StatusCode)
	}
	res.Body.Close()
}
