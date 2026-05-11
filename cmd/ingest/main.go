package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robot0001/urbanpetr-api/internal/config"
)

type takeoutEntry struct {
	Title    string `json:"title"`
	TitleURL string `json:"titleUrl"`
	Subtitles []struct {
		Name string `json:"name"`
	} `json:"subtitles"`
	Time string `json:"time"`
}

type videoInfo struct {
	videoID string
	vtype   string
	title   string
	channel *string
	watchedAt time.Time
}

func parseEntries(data []byte, cutoff time.Time) ([]videoInfo, error) {
	var entries []takeoutEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	var videos []videoInfo
	for _, e := range entries {
		if e.TitleURL == "" {
			continue // deleted video — no ID to extract
		}

		watchedAt, err := time.Parse(time.RFC3339Nano, e.Time)
		if err != nil {
			continue
		}
		if !cutoff.IsZero() && watchedAt.Before(cutoff) {
			continue
		}

		videoID, vtype, ok := extractVideoID(e.TitleURL)
		if !ok {
			continue
		}

		title := strings.TrimPrefix(e.Title, "Watched ")

		var channel *string
		if len(e.Subtitles) > 0 && e.Subtitles[0].Name != "" {
			ch := e.Subtitles[0].Name
			channel = &ch
		}

		videos = append(videos, videoInfo{
			videoID:   videoID,
			vtype:     vtype,
			title:     title,
			channel:   channel,
			watchedAt: watchedAt,
		})
	}
	return videos, nil
}

func extractVideoID(rawURL string) (id, vtype string, ok bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", false
	}
	if strings.Contains(u.Path, "/shorts/") {
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(parts) >= 2 && parts[1] != "" {
			return parts[1], "short", true
		}
		return "", "", false
	}
	v := u.Query().Get("v")
	if v == "" {
		return "", "", false
	}
	return v, "video", true
}

func ingest(ctx context.Context, db *pgxpool.Pool, videos []videoInfo, log *slog.Logger) error {
	inserted, skipped := 0, 0
	for _, v := range videos {
		var videoRowID int64
		err := db.QueryRow(ctx, `
			INSERT INTO youtube_video (video_id, type, title, channel)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (video_id) DO UPDATE SET
				title   = EXCLUDED.title,
				channel = COALESCE(EXCLUDED.channel, youtube_video.channel)
			RETURNING id`,
			v.videoID, v.vtype, v.title, v.channel,
		).Scan(&videoRowID)
		if err != nil {
			return fmt.Errorf("upsert video %s: %w", v.videoID, err)
		}

		tag, err := db.Exec(ctx, `
			INSERT INTO youtube_history (id_youtube_video, watched_at)
			VALUES ($1, $2)
			ON CONFLICT (id_youtube_video, watched_at) DO NOTHING`,
			videoRowID, v.watchedAt,
		)
		if err != nil {
			return fmt.Errorf("insert history %s @ %s: %w", v.videoID, v.watchedAt, err)
		}
		if tag.RowsAffected() > 0 {
			inserted++
		} else {
			skipped++
		}
	}
	log.Info("ingest complete", "inserted", inserted, "skipped_duplicates", skipped, "total", len(videos))
	return nil
}

func initPool(ctx context.Context) (*pgxpool.Pool, error) {
	creds, err := config.GetLocalCredentials()
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, creds.LocalDSN())
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func main() {
	days := flag.Int("days", 0, "only import last N days (0 = all)")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: ingest [--days=N] <path/to/watch-history.json>")
		os.Exit(1)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	data, err := os.ReadFile(args[0])
	if err != nil {
		log.Error("read file", "error", err)
		os.Exit(1)
	}

	var cutoff time.Time
	if *days > 0 {
		cutoff = time.Now().UTC().AddDate(0, 0, -*days)
	}

	videos, err := parseEntries(data, cutoff)
	if err != nil {
		log.Error("parse entries", "error", err)
		os.Exit(1)
	}
	log.Info("parsed entries", "count", len(videos))

	db, err := initPool(ctx)
	if err != nil {
		log.Error("db pool", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := ingest(ctx, db, videos, log); err != nil {
		log.Error("ingest", "error", err)
		os.Exit(1)
	}
}

