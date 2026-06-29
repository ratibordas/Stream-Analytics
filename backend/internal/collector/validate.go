package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func ValidateTwitch(ctx context.Context, clientID, secret string) error {
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {secret},
		"grant_type":    {"client_credentials"},
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://id.twitch.tv/oauth2/token",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("Twitch unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return nil
	}
	var e struct {
		Message string `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&e)
	if e.Message == "" {
		e.Message = resp.Status
	}
	return fmt.Errorf("Twitch rejected the keys: %s", e.Message)
}

func ValidateYouTube(ctx context.Context, apiKey string) error {
	q := url.Values{"part": {"id"}, "id": {"dQw4w9WgXcQ"}, "key": {apiKey}}
	req, _ := http.NewRequestWithContext(ctx, "GET",
		"https://www.googleapis.com/youtube/v3/videos?"+q.Encode(), nil)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("YouTube unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return nil
	}
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&e)
	msg := e.Error.Message
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("YouTube rejected the key: %s", msg)
}
