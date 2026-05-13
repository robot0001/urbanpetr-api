package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type entry struct {
	Title    string `json:"title"`
	TitleURL string `json:"titleUrl"`
	Subtitles []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"subtitles"`
	Time string `json:"time"`
}

type VideoInfo struct {
	VideoID    string
	Type       string
	Title      string
	Channel    *string
	ChannelURL *string
	WatchedAt  time.Time
}

func ParseEntries(data []byte, cutoff time.Time) ([]VideoInfo, error) {
	var entries []entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	var videos []VideoInfo
	for _, e := range entries {
		if e.TitleURL == "" {
			continue
		}

		watchedAt, err := time.Parse(time.RFC3339Nano, e.Time)
		if err != nil {
			continue
		}
		if !cutoff.IsZero() && watchedAt.Before(cutoff) {
			continue
		}

		videoID, vtype, ok := ExtractVideoID(e.TitleURL)
		if !ok {
			continue
		}

		title := strings.TrimPrefix(e.Title, "Watched ")

		var channel *string
		var channelURL *string
		if len(e.Subtitles) > 0 && e.Subtitles[0].Name != "" {
			ch := e.Subtitles[0].Name
			channel = &ch
			if e.Subtitles[0].URL != "" {
				cu := e.Subtitles[0].URL
				channelURL = &cu
			}
		}

		videos = append(videos, VideoInfo{
			VideoID:    videoID,
			Type:       vtype,
			Title:      title,
			Channel:    channel,
			ChannelURL: channelURL,
			WatchedAt:  watchedAt,
		})
	}
	return videos, nil
}

func ExtractVideoID(rawURL string) (id, vtype string, ok bool) {
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

// Ingest upserts videos and inserts new history rows. Returns total parsed
// entries and how many new history rows were created.
func Ingest(ctx context.Context, db *pgxpool.Pool, videos []VideoInfo) (total, created int, err error) {
	total = len(videos)
	for _, v := range videos {
		var videoRowID int64
		err = db.QueryRow(ctx, `
			INSERT INTO youtube_video (video_id, type, title, channel, channel_url)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (video_id) DO UPDATE SET
				title       = EXCLUDED.title,
				channel     = COALESCE(EXCLUDED.channel, youtube_video.channel),
				channel_url = COALESCE(EXCLUDED.channel_url, youtube_video.channel_url)
			RETURNING id`,
			v.VideoID, v.Type, v.Title, v.Channel, v.ChannelURL,
		).Scan(&videoRowID)
		if err != nil {
			return total, created, fmt.Errorf("upsert video %s: %w", v.VideoID, err)
		}

		tag, execErr := db.Exec(ctx, `
			INSERT INTO youtube_history (id_youtube_video, watched_at)
			VALUES ($1, $2)
			ON CONFLICT (id_youtube_video, watched_at) DO NOTHING`,
			videoRowID, v.WatchedAt,
		)
		if execErr != nil {
			return total, created, fmt.Errorf("insert history %s @ %s: %w", v.VideoID, v.WatchedAt, execErr)
		}
		if tag.RowsAffected() > 0 {
			created++
		}
	}
	return total, created, nil
}
