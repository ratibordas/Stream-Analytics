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

	mu     sync.Mutex
	cancel context.CancelFunc
	mgr    *collector.Manager
	keys   config.Keys
	mock   bool
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
		if keys.YouTubeAPIKey != "" {
			ytInterval := a.cfg.YTPollInterval
			if len(a.cfg.YouTubeQueries) > 2 && ytInterval < 30*time.Minute {
				log.Printf("youtube: %d queries -> raising poll interval to 30m to respect quota",
					len(a.cfg.YouTubeQueries))
				ytInterval = 30 * time.Minute
			}
			mgr.Add(collector.NewYouTube(keys.YouTubeAPIKey, a.cfg.YouTubeQueries, a.st), ytInterval)
		} else {
			log.Printf("youtube collector: disabled (no key)")
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

func (a *app) keysStatus() api.KeysStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return api.KeysStatus{
		Twitch:  a.keys.TwitchClientID != "" && a.keys.TwitchSecret != "",
		YouTube: a.keys.YouTubeAPIKey != "",
		Mock:    a.mock,
	}
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

	a := &app{cfg: cfg, st: st, root: ctx}
	a.rebuild(config.Keys{
		TwitchClientID: cfg.TwitchClientID,
		TwitchSecret:   cfg.TwitchSecret,
		YouTubeAPIKey:  cfg.YouTubeAPIKey,
	})

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: (&api.Server{
			Store:      st,
			StaticDir:  cfg.StaticDir,
			PollNow:    a.pollNow,
			IsMock:     a.isMock,
			KeysStatus: a.keysStatus,
			SetKeys:    a.setKeys,
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
