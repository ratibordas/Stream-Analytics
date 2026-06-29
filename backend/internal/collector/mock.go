package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"stream-analytics/backend/internal/store"
)

type Mock struct {
	Categories []string
	Store      *store.Store
	streams    map[string][]*mockStream
	t0         time.Time
	once       sync.Once
}

type mockStream struct {
	id, streamer, title, game, lang string
	base                            float64
	phase                           float64
	monthDrift                      float64
	subs0                           int
	subsRate                        float64
}

func NewMock(categories []string, st *store.Store) *Mock {
	m := &Mock{
		Categories: categories, Store: st,
		streams: map[string][]*mockStream{},
		t0:      time.Now().Add(-30 * 24 * time.Hour),
	}
	for _, platform := range []string{"twitch", "youtube"} {
		var list []*mockStream
		for ci, cat := range categories {
			n := 6 + ci%5
			for i := 0; i < n; i++ {
				seed := fnv64(fmt.Sprintf("%s|%s|%d", platform, cat, i))
				rng := rand.New(rand.NewSource(int64(seed)))
				name := mockNames[seed%uint64(len(mockNames))] + fmt.Sprintf("_%d", i)
				list = append(list, &mockStream{
					id:         fmt.Sprintf("%s-%d-%d", platform, ci, i),
					streamer:   name,
					title:      fmt.Sprintf("%s — %s #%d", mockTitles[rng.Intn(len(mockTitles))], cat, i+1),
					game:       cat,
					lang:       []string{"ru", "en", "de", "es"}[rng.Intn(4)],
					base:       math.Pow(10, 1.5+rng.Float64()*3),
					phase:      rng.Float64() * 2 * math.Pi,
					monthDrift: (rng.Float64() - 0.5),
					subs0:      rng.Intn(900_000) + 1000,
					subsRate:   (rng.Float64() - 0.35) * 3000,
				})
			}
		}
		m.streams[platform] = list
	}
	return m
}

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Poll(_ context.Context) error {
	m.once.Do(func() {
		meta, err := m.Store.GetMeta(true)
		if err != nil || meta.SnapshotCount > 0 {
			return
		}
		log.Printf("mock: backfilling 30 days of history…")
		now := time.Now().Truncate(time.Minute)
		for ts := now.Add(-30 * 24 * time.Hour); ts.Before(now); ts = ts.Add(30 * time.Minute) {
			m.emit(ts)
		}
		log.Printf("mock: backfill done")
	})
	m.emit(time.Now())
	return nil
}

func (m *Mock) emit(t time.Time) {
	monthFrac := t.Sub(m.t0).Hours() / 24 / 30
	for platform, list := range m.streams {
		var items []store.StreamUpsert
		dayFrac := float64(t.Hour()*3600+t.Minute()*60) / 86400
		for _, s := range list {
			wave := math.Sin(2*math.Pi*dayFrac + s.phase)
			if wave < -0.65 {
				continue
			}
			noise := (rand.Float64() - 0.5) * 0.2
			drift := 1 + s.monthDrift*(monthFrac-0.5)*2
			v := int(s.base * drift * (1 + 0.45*wave + noise))
			if v < 0 {
				v = 0
			}
			subs := s.subs0 + int(s.subsRate*t.Sub(m.t0).Hours()/24)
			if subs < 0 {
				subs = 0
			}
			extra, _ := json.Marshal(map[string]any{"mock": true})
			items = append(items, store.StreamUpsert{
				ID: s.id, Streamer: s.streamer, Title: s.title, Game: s.game,
				Language: s.lang,
				URL:      "https://" + platform + ".example/" + s.streamer,
				Tags:     "mock", Extra: string(extra),
				StartedAt: t.Add(-3 * time.Hour).Unix(), Viewers: v,
				Subscribers: int64(subs),
			})
		}
		if err := m.Store.SavePoll(platform, t.Unix(), items); err != nil {
			log.Printf("mock: save %s: %v", platform, err)
		}
	}
}

func fnv64(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

var mockNames = []string{
	"shadow", "nova", "pixel", "drift", "echo", "blaze", "frost", "viper",
	"luna", "atlas", "zephyr", "raven", "comet", "onyx", "saber", "quark",
}

var mockTitles = []string{
	"Ranked grind", "Chill stream", "Speedrun practice", "Community games",
	"Road to the top", "Casual run", "Late night session", "Tournament prep",
}
