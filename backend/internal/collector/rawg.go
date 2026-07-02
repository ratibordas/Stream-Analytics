package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// RawgGame is a single entry from the RAWG game catalog.
type RawgGame struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

// SearchGames queries the RAWG catalog. An empty query returns popular games.
func SearchGames(ctx context.Context, apiKey, q string) ([]RawgGame, error) {
	if apiKey == "" {
		return nil, nil // no key -> empty catalog, not an error (picker still allows free-text)
	}
	v := url.Values{"key": {apiKey}, "page_size": {"24"}}
	if q != "" {
		v.Set("search", q)
	} else {
		v.Set("ordering", "-added")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.rawg.io/api/games?"+v.Encode(), nil)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("rawg: %s", resp.Status)
	}
	var body struct {
		Results []struct {
			Name            string `json:"name"`
			BackgroundImage string `json:"background_image"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]RawgGame, 0, len(body.Results))
	for _, g := range body.Results {
		out = append(out, RawgGame{Name: g.Name, Image: g.BackgroundImage})
	}
	return out, nil
}

func ValidateRAWG(ctx context.Context, apiKey string) error {
	v := url.Values{"key": {apiKey}, "page_size": {"1"}}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.rawg.io/api/games?"+v.Encode(), nil)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("RAWG unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return nil
	}
	if resp.StatusCode == 401 {
		return fmt.Errorf("RAWG rejected the key")
	}
	return fmt.Errorf("RAWG: %s", resp.Status)
}
