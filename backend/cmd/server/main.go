package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"stream-analytics/backend/internal/api"
	"stream-analytics/backend/internal/collector"
	"stream-analytics/backend/internal/config"
	"stream-analytics/backend/internal/store"
)

type app struct {
	cfg  config.Config
	st   *store.Store
	root context.Context

	mu      sync.Mutex
	cancel  context.CancelFunc
	mgr     *collector.Manager
	keys    config.Keys
	queries []string
	mock    bool
}

func (a *app) rebuild(keys config.Keys) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithCancel(a.root)
	a.cancel = cancel
	a.keys = keys

	mock := a.cfg.MockMode == "on" ||
		(a.cfg.MockMode == "auto" && keys.TwitchClientID == "" && keys.YouTubeAPIKey == "")
	a.mock = mock

	mgr := &collector.Manager{}
	if mock {
		log.Printf("MOCK mode: no API keys, generating fake data (Settings tab in the UI or .env)")
		mgr.Add(collector.NewMock(a.cfg.Categories, a.st), a.cfg.PollInterval)
	} else {
		// --- Twitch collector: DISABLED ---
		// Re-enable by uncommenting the block below (and the Twitch route in
		// internal/api/server.go and the Twitch UI in App.tsx / SettingsView.tsx).
		// if keys.TwitchClientID != "" && keys.TwitchSecret != "" {
		// 	mgr.Add(collector.NewTwitch(keys.TwitchClientID, keys.TwitchSecret,
		// 		a.cfg.Categories, a.cfg.MaxPagesPerGame, a.st), a.cfg.PollInterval)
		// } else {
		// 	log.Printf("twitch collector: disabled (no keys)")
		// }
		if keys.YouTubeAPIKey != "" && len(a.queries) > 0 {
			ytInterval := a.cfg.YTPollInterval
			if len(a.queries) > 2 && ytInterval < 30*time.Minute {
				log.Printf("youtube: %d queries -> raising poll interval to 30m to respect quota",
					len(a.queries))
				ytInterval = 30 * time.Minute
			}
			mgr.Add(collector.NewYouTube(keys.YouTubeAPIKey, a.queries, a.st), ytInterval)
		} else {
			log.Printf("youtube collector: disabled (no key or no tracked games)")
		}
	}
	mgr.Start(ctx)
	a.mgr = mgr
}

func (a *app) pollNow(ctx context.Context) []string {
	a.mu.Lock()
	m := a.mgr
	a.mu.Unlock()
	if m == nil {
		return []string{"collectors not running"}
	}
	return m.PollNow(ctx)
}

func (a *app) isMock() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.mock
}

func (a *app) getQueries() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string{}, a.queries...)
}

// setQueries replaces the tracked YouTube search queries and hot-rebuilds the
// collectors so the change takes effect without a restart.
func (a *app) setQueries(qs []string) []string {
	a.mu.Lock()
	a.queries = qs
	keys := a.keys
	a.mu.Unlock()
	a.rebuild(keys)
	return a.getQueries()
}

func (a *app) keysStatus() api.KeysStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return api.KeysStatus{
		Twitch:  a.keys.TwitchClientID != "" && a.keys.TwitchSecret != "",
		YouTube: a.keys.YouTubeAPIKey != "",
		Rawg:    a.keys.RawgAPIKey != "",
		Mock:    a.mock,
	}
}

// searchGames proxies the RAWG catalog for the game picker using the current
// RAWG key.
func (a *app) searchGames(ctx context.Context, q string) ([]collector.RawgGame, error) {
	a.mu.Lock()
	key := a.keys.RawgAPIKey
	a.mu.Unlock()
	return collector.SearchGames(ctx, key, q)
}

func (a *app) setKeys(ctx context.Context, keys config.Keys, wipe bool) (map[string]string, error) {
	fieldErrs := map[string]string{}
	// --- Twitch validation: DISABLED (see rebuild) ---
	// if keys.TwitchClientID != "" || keys.TwitchSecret != "" {
	// 	if keys.TwitchClientID == "" || keys.TwitchSecret == "" {
	// 		fieldErrs["twitch"] = "both Client ID and Client Secret are required"
	// 	} else if err := collector.ValidateTwitch(ctx, keys.TwitchClientID, keys.TwitchSecret); err != nil {
	// 		fieldErrs["twitch"] = err.Error()
	// 	}
	// }
	if keys.YouTubeAPIKey != "" {
		if err := collector.ValidateYouTube(ctx, keys.YouTubeAPIKey); err != nil {
			fieldErrs["youtube"] = err.Error()
		}
	}
	if keys.RawgAPIKey != "" {
		if err := collector.ValidateRAWG(ctx, keys.RawgAPIKey); err != nil {
			fieldErrs["rawg"] = err.Error()
		}
	}
	if len(fieldErrs) > 0 {
		return fieldErrs, nil
	}
	if wipe {
		if err := a.st.WipeAll(); err != nil {
			return nil, err
		}
		log.Printf("data wiped on key switch")
	}
	a.rebuild(keys)
	log.Printf("keys updated: twitch=%v youtube=%v mock=%v",
		keys.TwitchClientID != "", keys.YouTubeAPIKey != "", a.isMock())
	return nil, nil
}

func main() {
	config.LoadDotEnv(".env")
	config.LoadDotEnv("../.env")
	cfg := config.Load()

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a := &app{cfg: cfg, st: st, root: ctx, queries: cfg.YouTubeQueries}
	a.rebuild(config.Keys{
		TwitchClientID: cfg.TwitchClientID,
		TwitchSecret:   cfg.TwitchSecret,
		YouTubeAPIKey:  cfg.YouTubeAPIKey,
		RawgAPIKey:     cfg.RawgAPIKey,
	})

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: (&api.Server{
			Store:       st,
			StaticDir:   cfg.StaticDir,
			PollNow:     a.pollNow,
			IsMock:      a.isMock,
			KeysStatus:  a.keysStatus,
			SetKeys:     a.setKeys,
			GetQueries:  a.getQueries,
			SetQueries:  a.setQueries,
			SearchGames: a.searchGames,
		}).Handler(),
	}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	log.Printf("listening on http://localhost:%d", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
