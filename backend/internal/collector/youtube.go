package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"stream-analytics/backend/internal/store"
)

type YouTube struct {
	APIKey  string
	Queries []string
	Store   *store.Store
	http    *http.Client
}

func NewYouTube(apiKey string, queries []string, st *store.Store) *YouTube {
	return &YouTube{APIKey: apiKey, Queries: queries, Store: st,
		http: &http.Client{Timeout: 20 * time.Second}}
}

func (y *YouTube) Name() string { return "youtube" }

func (y *YouTube) Poll(ctx context.Context) error {
	ts := time.Now().Unix()
	var items []store.StreamUpsert
	for _, q := range y.Queries {
		got, err := y.fetchLive(ctx, q)
		if err != nil {
			log.Printf("youtube: %q: %v", q, err)
			continue
		}
		items = append(items, got...)
	}
	if len(items) == 0 {
		return fmt.Errorf("no streams fetched (YouTube quota exhausted?)")
	}
	if err := y.Store.SavePoll("youtube", ts, items); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	log.Printf("youtube: saved %d streams for %d queries", len(items), len(y.Queries))
	return nil
}

func (y *YouTube) fetchLive(ctx context.Context, query string) ([]store.StreamUpsert, error) {
	sq := url.Values{
		"part": {"snippet"}, "eventType": {"live"}, "type": {"video"},
		"maxResults": {"50"}, "order": {"viewCount"}, "q": {query}, "key": {y.APIKey},
	}
	var sr struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
		} `json:"items"`
	}
	if err := y.get(ctx, "https://www.googleapis.com/youtube/v3/search?"+sq.Encode(), &sr); err != nil {
		return nil, err
	}
	if len(sr.Items) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(sr.Items))
	for _, it := range sr.Items {
		if it.ID.VideoID != "" {
			ids = append(ids, it.ID.VideoID)
		}
	}

	vq := url.Values{
		"part": {"snippet,liveStreamingDetails"},
		"id":   {strings.Join(ids, ",")}, "key": {y.APIKey},
	}
	var vr struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title        string   `json:"title"`
				ChannelID    string   `json:"channelId"`
				ChannelTitle string   `json:"channelTitle"`
				CategoryID   string   `json:"categoryId"`
				Language     string   `json:"defaultAudioLanguage"`
				Tags         []string `json:"tags"`
				Thumbnails   struct {
					Medium struct {
						URL string `json:"url"`
					} `json:"medium"`
				} `json:"thumbnails"`
			} `json:"snippet"`
			Live struct {
				ConcurrentViewers string `json:"concurrentViewers"`
				ActualStartTime   string `json:"actualStartTime"`
			} `json:"liveStreamingDetails"`
		} `json:"items"`
	}
	if err := y.get(ctx, "https://www.googleapis.com/youtube/v3/videos?"+vq.Encode(), &vr); err != nil {
		return nil, err
	}

	chSet := map[string]bool{}
	for _, v := range vr.Items {
		chSet[v.Snippet.ChannelID] = true
	}
	chIDs := make([]string, 0, len(chSet))
	for id := range chSet {
		chIDs = append(chIDs, id)
	}
	subs := map[string]string{}
	avatars := map[string]string{}
	if len(chIDs) > 0 {
		cq := url.Values{"part": {"snippet,statistics"}, "id": {strings.Join(chIDs, ",")}, "key": {y.APIKey}}
		var cr struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					Thumbnails struct {
						Default struct {
							URL string `json:"url"`
						} `json:"default"`
					} `json:"thumbnails"`
				} `json:"snippet"`
				Statistics struct {
					SubscriberCount string `json:"subscriberCount"`
				} `json:"statistics"`
			} `json:"items"`
		}
		if err := y.get(ctx, "https://www.googleapis.com/youtube/v3/channels?"+cq.Encode(), &cr); err == nil {
			for _, c := range cr.Items {
				subs[c.ID] = c.Statistics.SubscriberCount
				avatars[c.ID] = c.Snippet.Thumbnails.Default.URL
			}
		}
	}

	var out []store.StreamUpsert
	for _, v := range vr.Items {
		viewers, _ := strconv.Atoi(v.Live.ConcurrentViewers)
		started, _ := time.Parse(time.RFC3339, v.Live.ActualStartTime)
		extra, _ := json.Marshal(map[string]any{
			"category_id": v.Snippet.CategoryID,
			"channel_id":  v.Snippet.ChannelID,
		})
		subscribers := int64(-1)
		if n, err := strconv.ParseInt(subs[v.Snippet.ChannelID], 10, 64); err == nil {
			subscribers = n
		}
		out = append(out, store.StreamUpsert{
			ID: v.ID, Streamer: v.Snippet.ChannelTitle, Title: v.Snippet.Title,
			Game:      query,
			Language:  v.Snippet.Language,
			URL:       "https://youtube.com/watch?v=" + v.ID,
			Tags:      strings.Join(v.Snippet.Tags, ", "),
			Extra:     string(extra),
			AvatarURL: avatars[v.Snippet.ChannelID],
			Thumbnail: v.Snippet.Thumbnails.Medium.URL,
			StartedAt: started.Unix(), Viewers: viewers,
			Subscribers: subscribers,
		})
	}
	return out, nil
}

func (y *YouTube) get(ctx context.Context, u string, dst any) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := y.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("youtube: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
