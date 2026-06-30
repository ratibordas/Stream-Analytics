package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Keys struct {
	TwitchClientID string `json:"twitch_client_id"`
	TwitchSecret   string `json:"twitch_client_secret"`
	YouTubeAPIKey  string `json:"youtube_api_key"`
}

type Config struct {
	Port            int
	DBPath          string
	TwitchClientID  string
	TwitchSecret    string
	YouTubeAPIKey   string
	Mock            bool
	MockMode        string
	Categories      []string
	YouTubeQueries  []string
	PollInterval    time.Duration
	YTPollInterval  time.Duration
	StaticDir       string
	MaxPagesPerGame int
}

func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getint(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func getlist(k, def string) []string {
	raw := getenv(k, def)
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func Load() Config {
	c := Config{
		Port:            getint("PORT", 8080),
		DBPath:          getenv("DB_PATH", "./data/analytics.db"),
		TwitchClientID:  os.Getenv("TWITCH_CLIENT_ID"),
		TwitchSecret:    os.Getenv("TWITCH_CLIENT_SECRET"),
		YouTubeAPIKey:   os.Getenv("YOUTUBE_API_KEY"),
		Categories:      getlist("CATEGORIES", "Just Chatting,League of Legends,Counter-Strike,Dota 2,Minecraft,Grand Theft Auto V,Valorant,Fortnite"),
		YouTubeQueries:  getlist("YOUTUBE_QUERIES", "Minecraft,Fortnite,VALORANT,League of Legends,Grand Theft Auto V,Counter-Strike 2,Dota 2,Call of Duty Warzone,Apex Legends,Roblox"),
		PollInterval:    time.Duration(getint("POLL_INTERVAL_SEC", 900)) * time.Second,
		YTPollInterval:  time.Duration(getint("YT_POLL_INTERVAL_SEC", 14400)) * time.Second,
		StaticDir:       getenv("STATIC_DIR", "../frontend/dist"),
		MaxPagesPerGame: getint("MAX_PAGES_PER_GAME", 2),
	}
	switch strings.ToLower(getenv("MOCK", "auto")) {
	case "1", "true", "yes", "on":
		c.MockMode = "on"
	case "0", "false", "no", "off":
		c.MockMode = "off"
	default:
		c.MockMode = "auto"
	}
	c.Mock = c.MockMode == "on" ||
		(c.MockMode == "auto" && c.TwitchClientID == "" && c.YouTubeAPIKey == "")
	return c
}
