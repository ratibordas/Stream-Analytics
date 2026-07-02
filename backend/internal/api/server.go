package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"stream-analytics/backend/internal/collector"
	"stream-analytics/backend/internal/config"
	"stream-analytics/backend/internal/store"
)

type KeysStatus struct {
	Twitch  bool `json:"twitch_configured"`
	YouTube bool `json:"youtube_configured"`
	Rawg    bool `json:"rawg_configured"`
	Mock    bool `json:"mock"`
}

type Server struct {
	Store      *store.Store
	StaticDir  string
	PollNow    func(ctx context.Context) []string
	IsMock     func() bool
	KeysStatus func() KeysStatus
	SetKeys    func(ctx context.Context, k config.Keys, wipe bool) (map[string]string, error)

	GetQueries  func() []string
	SetQueries  func(qs []string) []string
	SearchGames func(ctx context.Context, q string) ([]collector.RawgGame, error)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("POST /api/poll", s.handlePoll)
	mux.HandleFunc("GET /api/keys/status", s.handleKeysStatus)
	mux.HandleFunc("POST /api/keys", s.handleSetKeys)
	mux.HandleFunc("GET /api/queries", s.handleGetQueries)
	mux.HandleFunc("POST /api/queries", s.handleSetQueries)
	mux.HandleFunc("GET /api/games/search", s.handleGameSearch)
	mux.HandleFunc("GET /api/{platform}/streams", s.handleStreams)
	mux.HandleFunc("GET /api/{platform}/games", s.handleGames)

	if st, err := os.Stat(s.StaticDir); err == nil && st.IsDir() {
		fs := http.FileServer(http.Dir(s.StaticDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Join(s.StaticDir, filepath.Clean(r.URL.Path))
			if _, err := os.Stat(path); err != nil {
				http.ServeFile(w, r, filepath.Join(s.StaticDir, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		})
	}
	return cors(mux)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Twitch is disabled; only youtube is served. To re-enable Twitch add
// `|| p == "twitch"` back here.
func platformOK(p string) bool { return p == "youtube" }

func parsePeriod(r *http.Request) (int64, int64) {
	now := time.Now().Unix()
	from, to := now-86400, now
	if v, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64); err == nil && v > 0 {
		from = v
	}
	if v, err := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64); err == nil && v > 0 {
		to = v
	}
	if from > to {
		from, to = to, from
	}
	return from, to
}

func (s *Server) handlePoll(w http.ResponseWriter, r *http.Request) {
	if s.PollNow == nil {
		writeErr(w, 503, "collectors not running")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	errs := s.PollNow(ctx)
	if errs == nil {
		errs = []string{}
	}
	writeJSON(w, map[string]any{"ok": len(errs) == 0, "errors": errs})
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	m, err := s.Store.GetMeta(s.IsMock())
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, m)
}

func (s *Server) handleKeysStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.KeysStatus())
}

func (s *Server) handleSetKeys(w http.ResponseWriter, r *http.Request) {
	var body struct {
		config.Keys
		WipeData bool `json:"wipe_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	fieldErrs, err := s.SetKeys(ctx, body.Keys, body.WipeData)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if fieldErrs == nil {
		fieldErrs = map[string]string{}
	}
	writeJSON(w, map[string]any{
		"ok":     len(fieldErrs) == 0,
		"errors": fieldErrs,
		"status": s.KeysStatus(),
	})
}

func (s *Server) handleStreams(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")
	if !platformOK(platform) {
		writeErr(w, 404, "unknown platform: "+platform)
		return
	}
	q := r.URL.Query()
	from, to := parsePeriod(r)
	atoi := func(k string) int { v, _ := strconv.Atoi(q.Get(k)); return v }

	f := store.StreamFilter{
		Platform:   platform,
		Game:       q.Get("game"),
		Streamer:   q.Get("streamer"),
		MinViewers: atoi("min_viewers"),
		MaxViewers: atoi("max_viewers"),
		From:       from, To: to,
		OnlyLive: q.Get("live") != "0",
		SortBy:   q.Get("sort"),
		Order:    q.Get("order"),
		Limit:    atoi("limit"),
		Offset:   atoi("offset"),
	}
	items, total, err := s.Store.ListStreams(f)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"items": items, "total": total, "from": from, "to": to})
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")
	if !platformOK(platform) {
		writeErr(w, 404, "unknown platform: "+platform)
		return
	}
	from, to := parsePeriod(r)
	items, err := s.Store.ListGames(platform, from, to)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"items": items, "from": from, "to": to})
}

func (s *Server) handleGetQueries(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"queries": s.GetQueries()})
}

func (s *Server) handleSetQueries(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Queries []string `json:"queries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	clean := make([]string, 0, len(body.Queries))
	seen := map[string]bool{}
	for _, q := range body.Queries {
		q = strings.TrimSpace(q)
		if q == "" || seen[q] {
			continue
		}
		seen[q] = true
		clean = append(clean, q)
	}
	writeJSON(w, map[string]any{"queries": s.SetQueries(clean)})
}

func (s *Server) handleGameSearch(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	games, err := s.SearchGames(ctx, r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	if games == nil {
		games = []collector.RawgGame{}
	}
	writeJSON(w, map[string]any{"items": games})
}
