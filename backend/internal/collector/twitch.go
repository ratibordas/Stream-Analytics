package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"stream-analytics/backend/internal/store"
)

type Twitch struct {
	ClientID, Secret string
	Categories       []string
	MaxPages         int
	Store            *store.Store

	http     *http.Client
	mu       sync.Mutex
	token    string
	tokenExp time.Time
	gameIDs  map[string]string
	avatars  map[string]string
}

func NewTwitch(clientID, secret string, categories []string, maxPages int, st *store.Store) *Twitch {
	return &Twitch{
		ClientID: clientID, Secret: secret, Categories: categories,
		MaxPages: maxPages, Store: st,
		http:    &http.Client{Timeout: 15 * time.Second},
		gameIDs: map[string]string{},
		avatars: map[string]string{},
	}
}

func (t *Twitch) Name() string { return "twitch" }

func (t *Twitch) Poll(ctx context.Context) error {
	if err := t.ensureGameIDs(ctx); err != nil {
		return fmt.Errorf("resolve games: %w", err)
	}
	ts := time.Now().Unix()
	var items []store.StreamUpsert
	var uids []string
	for name, id := range t.gameIDs {
		got, gotUIDs, err := t.fetchStreams(ctx, id, name)
		if err != nil {
			log.Printf("twitch: fetch %q: %v", name, err)
			continue
		}
		items = append(items, got...)
		uids = append(uids, gotUIDs...)
	}
	if len(items) == 0 {
		return fmt.Errorf("no streams fetched")
	}
	t.resolveAvatars(ctx, uids)
	t.mu.Lock()
	for i := range items {
		items[i].AvatarURL = t.avatars[uids[i]]
	}
	t.mu.Unlock()
	if err := t.Store.SavePoll("twitch", ts, items); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	log.Printf("twitch: saved %d streams across %d categories", len(items), len(t.gameIDs))
	return nil
}

func (t *Twitch) resolveAvatars(ctx context.Context, uids []string) {
	t.mu.Lock()
	var missing []string
	seen := map[string]bool{}
	for _, id := range uids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if _, ok := t.avatars[id]; !ok {
			missing = append(missing, id)
		}
	}
	t.mu.Unlock()
	for i := 0; i < len(missing); i += 100 {
		end := i + 100
		if end > len(missing) {
			end = len(missing)
		}
		q := url.Values{}
		for _, id := range missing[i:end] {
			q.Add("id", id)
		}
		var resp struct {
			Data []struct {
				ID              string `json:"id"`
				ProfileImageURL string `json:"profile_image_url"`
			} `json:"data"`
		}
		if err := t.get(ctx, "https://api.twitch.tv/helix/users?"+q.Encode(), &resp); err != nil {
			log.Printf("twitch: users: %v", err)
			return
		}
		t.mu.Lock()
		for _, u := range resp.Data {
			t.avatars[u.ID] = u.ProfileImageURL
		}
		t.mu.Unlock()
	}
}

type twitchStream struct {
	ID          string   `json:"id"`
	UserID      string   `json:"user_id"`
	UserLogin   string   `json:"user_login"`
	UserName    string   `json:"user_name"`
	GameName    string   `json:"game_name"`
	Title       string   `json:"title"`
	ViewerCount int      `json:"viewer_count"`
	StartedAt   string   `json:"started_at"`
	Language    string   `json:"language"`
	IsMature    bool     `json:"is_mature"`
	Tags        []string `json:"tags"`
}

func (t *Twitch) fetchStreams(ctx context.Context, gameID, gameName string) ([]store.StreamUpsert, []string, error) {
	var out []store.StreamUpsert
	var uids []string
	cursor := ""
	for page := 0; page < t.MaxPages; page++ {
		q := url.Values{"game_id": {gameID}, "first": {"100"}}
		if cursor != "" {
			q.Set("after", cursor)
		}
		var resp struct {
			Data       []twitchStream          `json:"data"`
			Pagination struct{ Cursor string } `json:"pagination"`
		}
		if err := t.get(ctx, "https://api.twitch.tv/helix/streams?"+q.Encode(), &resp); err != nil {
			return out, uids, err
		}
		for _, s := range resp.Data {
			started, _ := time.Parse(time.RFC3339, s.StartedAt)
			extra, _ := json.Marshal(map[string]any{
				"is_mature": s.IsMature,
				"user_name": s.UserName,
			})
			out = append(out, store.StreamUpsert{
				ID: s.ID, Streamer: s.UserName, Title: s.Title,
				Game: gameName, Language: s.Language,
				URL:  "https://twitch.tv/" + s.UserLogin,
				Tags: strings.Join(s.Tags, ", "), Extra: string(extra),
				StartedAt: started.Unix(), Viewers: s.ViewerCount,
				Subscribers: -1,
			})
			uids = append(uids, s.UserID)
		}
		cursor = resp.Pagination.Cursor
		if cursor == "" || len(resp.Data) == 0 {
			break
		}
	}
	return out, uids, nil
}

func (t *Twitch) ensureGameIDs(ctx context.Context) error {
	t.mu.Lock()
	resolved := len(t.gameIDs)
	t.mu.Unlock()
	if resolved == len(t.Categories) {
		return nil
	}
	q := url.Values{}
	for _, name := range t.Categories {
		q.Add("name", name)
	}
	var resp struct {
		Data []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			BoxArtURL string `json:"box_art_url"`
		} `json:"data"`
	}
	if err := t.get(ctx, "https://api.twitch.tv/helix/games?"+q.Encode(), &resp); err != nil {
		return err
	}
	images := map[string]string{}
	t.mu.Lock()
	for _, g := range resp.Data {
		t.gameIDs[g.Name] = g.ID
		if g.BoxArtURL != "" {
			images[g.Name] = strings.Replace(g.BoxArtURL, "{width}x{height}", "144x192", 1)
		}
	}
	n := len(t.gameIDs)
	t.mu.Unlock()
	if len(images) > 0 {
		if err := t.Store.UpsertCategoryImages("twitch", images); err != nil {
			log.Printf("twitch: save category images: %v", err)
		}
	}
	if n == 0 {
		return fmt.Errorf("no categories resolved (check CATEGORIES names)")
	}
	return nil
}

func (t *Twitch) get(ctx context.Context, u string, dst any) error {
	tok, err := t.getToken(ctx)
	if err != nil {
		return err
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("Client-Id", t.ClientID)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := t.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		t.mu.Lock()
		t.token = ""
		t.mu.Unlock()
		return fmt.Errorf("twitch 401 (token refreshed, will retry next cycle)")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("twitch %s: %s", u, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (t *Twitch) getToken(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token != "" && time.Now().Before(t.tokenExp) {
		return t.token, nil
	}
	form := url.Values{
		"client_id":     {t.ClientID},
		"client_secret": {t.Secret},
		"grant_type":    {"client_credentials"},
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://id.twitch.tv/oauth2/token",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("twitch oauth failed: %s", resp.Status)
	}
	t.token = tr.AccessToken
	t.tokenExp = time.Now().Add(time.Duration(tr.ExpiresIn-60) * time.Second)
	return t.token, nil
}
