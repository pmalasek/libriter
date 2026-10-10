package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sceniterEnv je router se Sceniterem nad dočasnou videoknihovnou.
type sceniterEnv struct {
	t      *testing.T
	router http.Handler
	token  string
}

func (e *sceniterEnv) do(method, path, token string, body any) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *sceniterEnv) json(method, path string, body any, out any) int {
	e.t.Helper()
	rec := e.do(method, path, e.token, body)
	if out != nil && rec.Code < 300 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			e.t.Fatalf("%s %s: %v\n%s", method, path, err, rec.Body.String())
		}
	}
	return rec.Code
}

func makeVideo(t *testing.T, path string, args ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	base := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=12",
		"-f", "lavfi", "-i", "sine=duration=12"}
	out, err := exec.Command("ffmpeg", append(append(base, args...), "-shortest", path)...).CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg nevyrobí testovací video: %v %s", err, out)
	}
}

func TestSceniterEndToEnd(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s není k dispozici", tool)
		}
	}
	videoRoot := t.TempDir()
	makeVideo(t, filepath.Join(videoRoot, "Filmy", "Pelíšky (1999).mp4"), "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac")
	makeVideo(t, filepath.Join(videoRoot, "Filmy", "Amadeus-1984-CZ-Dab.mkv"), "-c:v", "libx264", "-preset", "ultrafast", "-g", "25", "-c:a", "ac3")
	makeVideo(t, filepath.Join(videoRoot, "Seriály", "Docent", "S01", "Docent - 01x01 - Lesní vražda.mkv"), "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac")
	if err := os.WriteFile(filepath.Join(videoRoot, "Filmy", "Pelíšky (1999).cs.srt"),
		[]byte("1\n00:00:01,000 --> 00:00:02,000\nAhoj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIDEO_ROOT", videoRoot)
	t.Setenv("TRANSCODE_ROOT", t.TempDir())
	t.Setenv("SCENITER_IMAGE_ROOT", t.TempDir())
	t.Setenv("TMDB_API_KEY", "")

	r, deps := testRouterDeps(t)
	e := &sceniterEnv{t: t, router: r}

	// Knihovny zakládá admin: Filmy jako filmy, Seriály jako seriály.
	if _, err := deps.Users.Create(context.Background(), "Správce", "admin@example.com", "", "tajneheslo", "admin"); err != nil {
		t.Fatal(err)
	}
	var adminLogin struct{ Token string }
	if rec := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "admin@example.com", "password": "tajneheslo"}); json.Unmarshal(rec.Body.Bytes(), &adminLogin) != nil || adminLogin.Token == "" {
		t.Fatalf("admin login: %d %s", rec.Code, rec.Body.String())
	}
	e.token = adminLogin.Token
	var candidates struct{ Dirs []string }
	if code := e.json("GET", "/api/v1/sceniter/admin/libraries/candidates", nil, &candidates); code != 200 ||
		strings.Join(candidates.Dirs, ",") != "Filmy,Seriály" {
		t.Fatalf("kandidáti: %d %v", code, candidates.Dirs)
	}
	for _, lib := range []map[string]string{
		{"dir": "Filmy", "kind": "movie", "name": "Filmy", "language": "cs"},
		{"dir": "Seriály", "kind": "show", "name": "Seriály", "language": "cs"},
	} {
		if code := e.json("POST", "/api/v1/sceniter/admin/libraries", lib, nil); code != http.StatusCreated {
			t.Fatalf("knihovna %s: %d", lib["dir"], code)
		}
	}
	if code := e.json("POST", "/api/v1/sceniter/admin/libraries", map[string]string{"dir": "../etc", "kind": "movie"}, nil); code != http.StatusBadRequest {
		t.Fatalf("složka mimo VIDEO_ROOT: %d", code)
	}
	if code := e.json("POST", "/api/v1/sceniter/admin/libraries", map[string]string{"dir": "Filmy", "kind": "movie", "provider": "neni"}, nil); code != http.StatusBadRequest {
		t.Fatalf("neznámý zdroj: %d", code)
	}
	e.token = ""

	// Účet je společný: uživatel z jádra se přihlásí stejně jako do Libriteru.
	if rec := e.do("POST", "/api/v1/auth/register", "", map[string]string{
		"display_name": "Divák", "email": "divak@example.com", "password": "tajneheslo",
	}); rec.Code != http.StatusCreated {
		t.Fatalf("registrace: %d %s", rec.Code, rec.Body.String())
	}
	var login struct{ Token string }
	rec := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "divak@example.com", "password": "tajneheslo"})
	if err := json.Unmarshal(rec.Body.Bytes(), &login); err != nil || login.Token == "" {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	e.token = login.Token

	var libs []struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	if code := e.json("GET", "/api/v1/sceniter/libraries", nil, &libs); code != 200 || len(libs) != 2 || libs[1].Kind != "show" {
		t.Fatalf("knihovny: %d %+v", code, libs)
	}

	if rec := e.do("GET", "/api/v1/sceniter/movies", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bez tokenu: %d", rec.Code)
	}

	var cfg map[string]any
	if code := e.json("GET", "/api/v1/sceniter/config", nil, &cfg); code != 200 || cfg["enabled"] != true {
		t.Fatalf("config: %d %v", code, cfg)
	}

	// Počáteční průchod knihovnou běží na pozadí.
	var movies []struct {
		ID      string  `json:"id"`
		Title   string  `json:"title"`
		MediaID *string `json:"media_id"`
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		e.json("GET", "/api/v1/sceniter/movies", nil, &movies)
		if len(movies) == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if len(movies) != 2 {
		t.Fatalf("filmy: %+v", movies)
	}
	byTitle := map[string]string{}
	mediaOf := map[string]string{}
	for _, m := range movies {
		byTitle[m.Title], mediaOf[m.Title] = m.ID, *m.MediaID
	}

	var detail struct {
		Files []struct {
			Container string `json:"container"`
			Streams   []any  `json:"streams"`
		} `json:"files"`
	}
	if code := e.json("GET", "/api/v1/sceniter/movies/"+byTitle["Pelíšky"], nil, &detail); code != 200 ||
		len(detail.Files) != 1 || detail.Files[0].Container != "mp4" || len(detail.Files[0].Streams) != 2 {
		t.Fatalf("detail: %d %+v", code, detail)
	}

	type playback struct {
		Mode      string `json:"mode"`
		URL       string `json:"url"`
		HLS       bool   `json:"hls"`
		Subtitles []struct {
			Key string `json:"key"`
			URL string `json:"url"`
		} `json:"subtitles"`
	}

	// MP4 s H.264 a AAC hraje prohlížeč přímo; Range funguje.
	var pb playback
	e.json("GET", "/api/v1/sceniter/media/"+mediaOf["Pelíšky"]+"/playback?profile=web", nil, &pb)
	if pb.Mode != "direct" || pb.HLS || len(pb.Subtitles) != 1 {
		t.Fatalf("playback mp4: %+v", pb)
	}
	req := httptest.NewRequest("GET", pb.URL, nil)
	req.Header.Set("Range", "bytes=0-99")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusPartialContent || rr.Body.Len() != 100 || rr.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("stream: %d %d %q", rr.Code, rr.Body.Len(), rr.Header().Get("Content-Type"))
	}
	if rec := e.do("GET", pb.Subtitles[0].URL, "", nil); rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), "WEBVTT") {
		t.Fatalf("titulky: %d %q", rec.Code, rec.Body.String())
	}
	// Přihlašovací token adresu streamu neotevře, jen streamovací.
	if rec := e.do("GET", strings.Split(pb.URL, "?")[0]+"?t="+e.token, "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("stream s přihlašovacím tokenem: %d", rec.Code)
	}

	// MKV s AC3 (CZ dabing) v Chrome: převod zvuku přes HLS.
	e.json("GET", "/api/v1/sceniter/media/"+mediaOf["Amadeus"]+"/playback?profile=web", nil, &pb)
	if pb.Mode != "transcode_audio" || !pb.HLS {
		t.Fatalf("playback mkv: %+v", pb)
	}
	rec = e.do("GET", pb.URL, "", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "#EXT-X-STREAM-INF") {
		t.Fatalf("master: %d %s", rec.Code, rec.Body.String())
	}
	var media string
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(line, "/api/") {
			media = line
		}
	}
	rec = e.do("GET", media, "", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "seg_00000.m4s?t=") {
		t.Fatalf("media playlist: %d %s", rec.Code, rec.Body.String())
	}
	segURL := strings.Replace(strings.Split(media, "?")[0], "index.m3u8", "seg_00000.m4s", 1) + "?" + strings.Split(media, "?")[1]
	if rec := e.do("GET", segURL, "", nil); rec.Code != 200 || rec.Body.Len() == 0 {
		t.Fatalf("segment: %d", rec.Code)
	}

	// Rozkoukanost → Pokračovat ve sledování; watchlist; hodnocení.
	movieID := byTitle["Amadeus"]
	if code := e.json("PUT", "/api/v1/sceniter/progress", map[string]any{
		"movie_id": movieID, "media_id": mediaOf["Amadeus"], "position_sec": 5, "duration_sec": 600, "watched_seconds": 5,
	}, nil); code != http.StatusNoContent {
		t.Fatalf("progress: %d", code)
	}
	old := time.Now().Add(-time.Hour)
	if code := e.json("PUT", "/api/v1/sceniter/progress", map[string]any{
		"movie_id": movieID, "position_sec": 1, "recorded_at": old,
	}, nil); code != http.StatusConflict {
		t.Fatalf("starší pozice: %d, chci 409", code)
	}
	var home struct {
		Continue []struct {
			Kind  string `json:"kind"`
			Movie struct {
				ID string `json:"id"`
			} `json:"movie"`
		} `json:"continue"`
		Libraries []struct {
			Movies   []any `json:"movies"`
			Episodes []any `json:"episodes"`
		} `json:"libraries"`
	}
	e.json("GET", "/api/v1/sceniter/home", nil, &home)
	if len(home.Continue) != 1 || home.Continue[0].Movie.ID != movieID || len(home.Libraries) != 2 ||
		len(home.Libraries[0].Movies) != 2 || len(home.Libraries[1].Episodes) != 1 {
		t.Fatalf("home: %+v", home)
	}
	if code := e.json("PUT", "/api/v1/sceniter/watchlist/movie/"+byTitle["Pelíšky"], nil, nil); code != http.StatusNoContent {
		t.Fatalf("watchlist: %d", code)
	}
	if code := e.json("PUT", "/api/v1/sceniter/ratings/movie/"+byTitle["Pelíšky"], map[string]any{"rating": 9}, nil); code != http.StatusNoContent {
		t.Fatalf("rating: %d", code)
	}
	var state struct {
		User struct {
			InWatchlist bool `json:"in_watchlist"`
			Rating      int  `json:"rating"`
		} `json:"user"`
	}
	e.json("GET", "/api/v1/sceniter/movies/"+byTitle["Pelíšky"], nil, &state)
	if !state.User.InWatchlist || state.User.Rating != 9 {
		t.Fatalf("stav uživatele: %+v", state)
	}

	// Administrace knihovny je jen pro admina; čtenář dostane 403.
	if rec := e.do("POST", "/api/v1/sceniter/admin/scanner/rescan", e.token, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("rescan jako reader: %d", rec.Code)
	}
	// Ruční párování s TMDB je až pro editora.
	if rec := e.do("GET", "/api/v1/sceniter/metadata/search?q=x", e.token, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("hledání metadat jako reader: %d", rec.Code)
	}
}
