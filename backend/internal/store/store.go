package store

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS streams (
  platform        TEXT NOT NULL,
  id              TEXT NOT NULL,
  streamer        TEXT NOT NULL DEFAULT '',
  title           TEXT NOT NULL DEFAULT '',
  game            TEXT NOT NULL DEFAULT '',
  category        TEXT NOT NULL DEFAULT '',
  language        TEXT NOT NULL DEFAULT '',
  url             TEXT NOT NULL DEFAULT '',
  tags            TEXT NOT NULL DEFAULT '',
  extra           TEXT NOT NULL DEFAULT '{}',
  avatar_url      TEXT NOT NULL DEFAULT '',
  started_at      INTEGER NOT NULL DEFAULT 0,
  last_seen_at    INTEGER NOT NULL DEFAULT 0,
  current_viewers INTEGER NOT NULL DEFAULT 0,
  is_live         INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (platform, id)
);
CREATE TABLE IF NOT EXISTS group_images (
  platform  TEXT NOT NULL,
  dim       TEXT NOT NULL,
  name      TEXT NOT NULL,
  image_url TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (platform, dim, name)
);
CREATE TABLE IF NOT EXISTS snapshots (
  platform  TEXT NOT NULL,
  stream_id TEXT NOT NULL,
  streamer  TEXT NOT NULL DEFAULT '',
  ts        INTEGER NOT NULL,
  viewers   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_snap ON snapshots(platform, stream_id, ts);
CREATE INDEX IF NOT EXISTS idx_snap_ts ON snapshots(ts);
CREATE INDEX IF NOT EXISTS idx_snap_streamer ON snapshots(platform, streamer, ts);
CREATE TABLE IF NOT EXISTS group_snapshots (
  platform      TEXT NOT NULL,
  dim           TEXT NOT NULL,
  name          TEXT NOT NULL,
  ts            INTEGER NOT NULL,
  total_viewers INTEGER NOT NULL,
  stream_count  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_group ON group_snapshots(platform, dim, name, ts);
CREATE TABLE IF NOT EXISTS channel_stats (
  platform    TEXT NOT NULL,
  streamer    TEXT NOT NULL,
  ts          INTEGER NOT NULL,
  subscribers INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chan ON channel_stats(platform, streamer, ts);
`

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	db.Exec(`ALTER TABLE streams ADD COLUMN avatar_url TEXT NOT NULL DEFAULT ''`)
	db.Exec(`ALTER TABLE streams ADD COLUMN category TEXT NOT NULL DEFAULT ''`)
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) WipeAll() error {
	_, err := s.db.Exec(`DELETE FROM snapshots; DELETE FROM streams;
		DELETE FROM group_snapshots; DELETE FROM group_images; DELETE FROM channel_stats;`)
	return err
}

type StreamUpsert struct {
	Platform, ID, Streamer, Title, Game, Language, URL, Tags, Extra string
	AvatarURL                                                       string
	Thumbnail                                                       string
	StartedAt                                                       int64
	Viewers                                                         int
	Subscribers                                                     int64
}

func (s *Store) UpsertGroupImages(platform, dim string, images map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for name, url := range images {
		if _, err := tx.Exec(`INSERT INTO group_images(platform,dim,name,image_url) VALUES(?,?,?,?)
		  ON CONFLICT(platform,dim,name) DO UPDATE SET image_url=excluded.image_url`,
			platform, dim, name, url); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SavePoll(platform string, ts int64, items []StreamUpsert) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	up, err := tx.Prepare(`INSERT INTO streams
	  (platform,id,streamer,title,game,category,language,url,tags,extra,avatar_url,started_at,last_seen_at,current_viewers,is_live)
	  VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)
	  ON CONFLICT(platform,id) DO UPDATE SET
	    streamer=excluded.streamer, title=excluded.title, game=excluded.game, category=excluded.category,
	    language=excluded.language, url=excluded.url, tags=excluded.tags, extra=excluded.extra,
	    avatar_url=excluded.avatar_url,
	    started_at=excluded.started_at, last_seen_at=excluded.last_seen_at,
	    current_viewers=excluded.current_viewers, is_live=1`)
	if err != nil {
		return err
	}
	snap, err := tx.Prepare(`INSERT INTO snapshots(platform,stream_id,streamer,ts,viewers) VALUES(?,?,?,?,?)`)
	if err != nil {
		return err
	}
	chst, err := tx.Prepare(`INSERT INTO channel_stats(platform,streamer,ts,subscribers) VALUES(?,?,?,?)`)
	if err != nil {
		return err
	}

	games := map[string]*groupAgg{}
	seenSubs := map[string]bool{}
	for _, it := range items {
		if _, err := up.Exec(platform, it.ID, it.Streamer, it.Title, it.Game, "", it.Language,
			it.URL, it.Tags, it.Extra, it.AvatarURL, it.StartedAt, ts, it.Viewers); err != nil {
			return err
		}
		if _, err := snap.Exec(platform, it.ID, it.Streamer, ts, it.Viewers); err != nil {
			return err
		}
		if it.Subscribers >= 0 && !seenSubs[it.Streamer] {
			seenSubs[it.Streamer] = true
			if _, err := chst.Exec(platform, it.Streamer, ts, it.Subscribers); err != nil {
				return err
			}
		}
		addGroup(games, it.Game, it)
	}
	for name, a := range games {
		if _, err := tx.Exec(`INSERT INTO group_snapshots(platform,dim,name,ts,total_viewers,stream_count) VALUES(?,'game',?,?,?,?)`,
			platform, name, ts, a.viewers, a.count); err != nil {
			return err
		}
		if a.image != "" {
			if _, err := tx.Exec(`INSERT INTO group_images(platform,dim,name,image_url) VALUES(?,'game',?,?)
			  ON CONFLICT(platform,dim,name) DO UPDATE SET image_url=excluded.image_url`,
				platform, name, a.image); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`UPDATE streams SET is_live=0 WHERE platform=? AND last_seen_at < ?`, platform, ts); err != nil {
		return err
	}
	return tx.Commit()
}

type groupAgg struct {
	viewers, count, top int
	image               string
}

// addGroup accumulates a stream into a dimension bucket, tracking the
// thumbnail of the highest-viewer stream as the bucket's image.
func addGroup(m map[string]*groupAgg, name string, it StreamUpsert) {
	if name == "" {
		name = "(uncategorized)"
	}
	a := m[name]
	if a == nil {
		a = &groupAgg{}
		m[name] = a
	}
	a.viewers += it.Viewers
	a.count++
	if it.Viewers >= a.top {
		a.top = it.Viewers
		if it.Thumbnail != "" {
			a.image = it.Thumbnail
		}
	}
}

type StreamRow struct {
	Platform          string   `json:"platform"`
	ID                string   `json:"id"`
	Streamer          string   `json:"streamer"`
	Title             string   `json:"title"`
	Game              string   `json:"game"`
	Language          string   `json:"language"`
	URL               string   `json:"url"`
	Tags              string   `json:"tags"`
	Extra             string   `json:"extra"`
	AvatarURL         string   `json:"avatar_url"`
	GameImage         string   `json:"game_image_url"`
	StartedAt         int64    `json:"started_at"`
	LastSeenAt        int64    `json:"last_seen_at"`
	Viewers           int      `json:"current_viewers"`
	IsLive            bool     `json:"is_live"`
	AvgPeriod         float64  `json:"avg_viewers_period"`
	PeakPeriod        int      `json:"peak_viewers_period"`
	Samples           int      `json:"samples_period"`
	TrendPeriodPct    *float64 `json:"trend_period_pct"`
	TrendMonthPct     *float64 `json:"trend_month_pct"`
	SubsTrendMonthPct *float64 `json:"subs_trend_month_pct"`
	Subscribers       *int64   `json:"subscribers"`
}

type StreamFilter struct {
	Platform   string
	Game       string
	Streamer   string
	MinViewers int
	MaxViewers int
	From, To   int64
	OnlyLive   bool
	SortBy     string
	Order      string
	Limit      int
	Offset     int
}

func pctChange(a, b sql.NullFloat64) *float64 {
	if !a.Valid || !b.Valid || a.Float64 <= 0 {
		return nil
	}
	v := (b.Float64 - a.Float64) / a.Float64 * 100
	return &v
}

func (s *Store) ListStreams(f StreamFilter) ([]StreamRow, int, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	now := time.Now().Unix()
	monthAgo := now - 30*86400
	midMonth := now - 15*86400
	midPeriod := (f.From + f.To) / 2

	where := []string{"s.platform = ?"}
	wargs := []any{f.Platform}
	if f.Game != "" {
		where = append(where, "s.game LIKE ?")
		wargs = append(wargs, "%"+f.Game+"%")
	}
	if f.Streamer != "" {
		where = append(where, "s.streamer LIKE ?")
		wargs = append(wargs, "%"+f.Streamer+"%")
	}
	if f.MinViewers > 0 {
		where = append(where, "s.current_viewers >= ?")
		wargs = append(wargs, f.MinViewers)
	}
	if f.MaxViewers > 0 {
		where = append(where, "s.current_viewers <= ?")
		wargs = append(wargs, f.MaxViewers)
	}
	if f.OnlyLive {
		where = append(where, "s.is_live = 1")
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM streams s WHERE "+whereSQL, wargs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := "DESC"
	if strings.EqualFold(f.Order, "asc") {
		order = "ASC"
	}
	sortExpr := "s.current_viewers"
	switch f.SortBy {
	case "period":
		sortExpr = "COALESCE(p.avg_v, 0)"
	case "trend":
		sortExpr = "CASE WHEN p.h1 > 0 THEN (p.h2 - p.h1) / p.h1 END"
	}

	q := `SELECT s.platform,s.id,s.streamer,s.title,s.game,s.language,s.url,s.tags,s.extra,
	  s.avatar_url, COALESCE(gi_g.image_url,''),
	  s.started_at,s.last_seen_at,s.current_viewers,s.is_live,
	  COALESCE(p.avg_v,0), COALESCE(p.peak_v,0), COALESCE(p.cnt,0),
	  p.h1, p.h2, m.mh1, m.mh2, c1.s_first, c2.s_last
	FROM streams s
	LEFT JOIN group_images gi_g ON gi_g.platform = s.platform AND gi_g.dim = 'game' AND gi_g.name = s.game
	LEFT JOIN (
	  SELECT platform, stream_id, AVG(viewers) avg_v, MAX(viewers) peak_v, COUNT(*) cnt,
	    AVG(CASE WHEN ts < ? THEN viewers END) h1,
	    AVG(CASE WHEN ts >= ? THEN viewers END) h2
	  FROM snapshots WHERE ts BETWEEN ? AND ?
	  GROUP BY platform, stream_id
	) p ON p.platform = s.platform AND p.stream_id = s.id
	LEFT JOIN (
	  SELECT platform, streamer,
	    AVG(CASE WHEN ts < ? THEN viewers END) mh1,
	    AVG(CASE WHEN ts >= ? THEN viewers END) mh2
	  FROM snapshots WHERE ts >= ?
	  GROUP BY platform, streamer
	) m ON m.platform = s.platform AND m.streamer = s.streamer
	LEFT JOIN (
	  SELECT platform, streamer, subscribers AS s_first, MIN(ts) mt
	  FROM channel_stats WHERE ts >= ? GROUP BY platform, streamer
	) c1 ON c1.platform = s.platform AND c1.streamer = s.streamer
	LEFT JOIN (
	  SELECT platform, streamer, subscribers AS s_last, MAX(ts) mt
	  FROM channel_stats WHERE ts >= ? GROUP BY platform, streamer
	) c2 ON c2.platform = s.platform AND c2.streamer = s.streamer
	WHERE ` + whereSQL +
		fmt.Sprintf(" ORDER BY (%s IS NULL), %s %s, s.id LIMIT ? OFFSET ?", sortExpr, sortExpr, order)

	args := []any{midPeriod, midPeriod, f.From, f.To, midMonth, midMonth, monthAgo, monthAgo, monthAgo}
	args = append(args, wargs...)
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []StreamRow{}
	for rows.Next() {
		var r StreamRow
		var live int
		var h1, h2, mh1, mh2 sql.NullFloat64
		var sFirst, sLast sql.NullInt64
		if err := rows.Scan(&r.Platform, &r.ID, &r.Streamer, &r.Title, &r.Game, &r.Language,
			&r.URL, &r.Tags, &r.Extra, &r.AvatarURL, &r.GameImage,
			&r.StartedAt, &r.LastSeenAt, &r.Viewers, &live,
			&r.AvgPeriod, &r.PeakPeriod, &r.Samples,
			&h1, &h2, &mh1, &mh2, &sFirst, &sLast); err != nil {
			return nil, 0, err
		}
		r.IsLive = live == 1
		r.TrendPeriodPct = pctChange(h1, h2)
		r.TrendMonthPct = pctChange(mh1, mh2)
		if sFirst.Valid && sLast.Valid && sFirst.Int64 > 0 {
			v := float64(sLast.Int64-sFirst.Int64) / float64(sFirst.Int64) * 100
			r.SubsTrendMonthPct = &v
		}
		if sLast.Valid {
			r.Subscribers = &sLast.Int64
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

type CategoryRow struct {
	Category     string  `json:"category"`
	ImageURL     string  `json:"image_url"`
	AvgViewers   float64 `json:"avg_viewers"`
	PeakViewers  int     `json:"peak_viewers"`
	AvgChannels  float64 `json:"avg_channels"`
	ViewersPerCh float64 `json:"viewers_per_channel"`
	TrendPct     float64 `json:"trend_pct"`
	StabilityCV  float64 `json:"stability_cv_pct"`
	Samples      int     `json:"samples"`
}

func (s *Store) ListGames(platform string, from, to int64) ([]CategoryRow, error) {
	mid := (from + to) / 2
	rows, err := s.db.Query(`SELECT gs.name, COALESCE(gi.image_url,''),
	  AVG(gs.total_viewers), MAX(gs.total_viewers), AVG(gs.stream_count),
	  AVG(CAST(gs.total_viewers AS REAL)/MAX(gs.stream_count,1)),
	  AVG(CASE WHEN gs.ts < ?  THEN gs.total_viewers END),
	  AVG(CASE WHEN gs.ts >= ? THEN gs.total_viewers END),
	  AVG(CAST(gs.total_viewers AS REAL)*gs.total_viewers),
	  COUNT(*)
	FROM group_snapshots gs
	LEFT JOIN group_images gi ON gi.platform = gs.platform AND gi.dim = gs.dim AND gi.name = gs.name
	WHERE gs.platform = ? AND gs.dim = 'game' AND gs.ts BETWEEN ? AND ?
	GROUP BY gs.name
	ORDER BY 3 DESC`, mid, mid, platform, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CategoryRow{}
	for rows.Next() {
		var r CategoryRow
		var h1, h2 sql.NullFloat64
		var avgSq float64
		if err := rows.Scan(&r.Category, &r.ImageURL, &r.AvgViewers, &r.PeakViewers, &r.AvgChannels,
			&r.ViewersPerCh, &h1, &h2, &avgSq, &r.Samples); err != nil {
			return nil, err
		}
		if h1.Valid && h2.Valid && h1.Float64 > 0 {
			r.TrendPct = (h2.Float64 - h1.Float64) / h1.Float64 * 100
		}
		if r.AvgViewers > 0 {
			variance := avgSq - r.AvgViewers*r.AvgViewers
			if variance > 0 {
				r.StabilityCV = math.Sqrt(variance) / r.AvgViewers * 100
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type Meta struct {
	Mock          bool  `json:"mock"`
	FirstSnapshot int64 `json:"first_snapshot"`
	LastSnapshot  int64 `json:"last_snapshot"`
	SnapshotCount int64 `json:"snapshot_count"`
	ServerTime    int64 `json:"server_time"`
}

func (s *Store) GetMeta(mock bool) (Meta, error) {
	m := Meta{Mock: mock, ServerTime: time.Now().Unix()}
	err := s.db.QueryRow(`SELECT COALESCE(MIN(ts),0), COALESCE(MAX(ts),0), COUNT(*) FROM snapshots`).
		Scan(&m.FirstSnapshot, &m.LastSnapshot, &m.SnapshotCount)
	return m, err
}
