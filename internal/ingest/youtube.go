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

const batchSize = 1000

// Ingest upserts videos and inserts new history rows in batches of 1000.
// Returns total parsed entries and how many new history rows were created.
func Ingest(ctx context.Context, db *pgxpool.Pool, videos []VideoInfo) (total, created int, err error) {
	total = len(videos)
	for i := 0; i < len(videos); i += batchSize {
		end := min(i+batchSize, len(videos))
		n, batchErr := ingestBatch(ctx, db, videos[i:end])
		if batchErr != nil {
			return total, created, batchErr
		}
		created += n
	}
	return total, created, nil
}

func ingestBatch(ctx context.Context, db *pgxpool.Pool, batch []VideoInfo) (int, error) {
	// Deduplicate by video_id for the upsert — same video watched multiple times in a
	// batch would cause "ON CONFLICT DO UPDATE command cannot affect row a second time".
	seen := make(map[string]bool, len(batch))
	var vidIDs, types, titles []string
	var channels, channelURLs []*string
	for _, v := range batch {
		if seen[v.VideoID] {
			continue
		}
		seen[v.VideoID] = true
		vidIDs      = append(vidIDs, v.VideoID)
		types       = append(types, v.Type)
		titles      = append(titles, v.Title)
		channels    = append(channels, v.Channel)
		channelURLs = append(channelURLs, v.ChannelURL)
	}

	// Bulk upsert into youtube_video; DO UPDATE ensures RETURNING covers conflicts too
	rows, err := db.Query(ctx, `
		INSERT INTO youtube_video (video_id, type, title, channel, channel_url)
		SELECT v, t::youtube_video_type, ti, ch, cu
		FROM UNNEST($1::text[], $2::text[], $3::text[], $4::text[], $5::text[])
		    AS x(v, t, ti, ch, cu)
		ON CONFLICT (video_id) DO UPDATE SET
		    title       = EXCLUDED.title,
		    channel     = COALESCE(EXCLUDED.channel, youtube_video.channel),
		    channel_url = COALESCE(EXCLUDED.channel_url, youtube_video.channel_url)
		RETURNING id, video_id`,
		vidIDs, types, titles, channels, channelURLs,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk upsert videos: %w", err)
	}

	idByVidID := make(map[string]int64, len(vidIDs))
	for rows.Next() {
		var id int64
		var vid string
		if scanErr := rows.Scan(&id, &vid); scanErr != nil {
			rows.Close()
			return 0, fmt.Errorf("scan video upsert row: %w", scanErr)
		}
		idByVidID[vid] = id
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, fmt.Errorf("read video upsert: %w", err)
	}

	sz := len(batch)
	intIDs     := make([]int64,     sz)
	watchedAts := make([]time.Time, sz)
	for i, v := range batch {
		intIDs[i]     = idByVidID[v.VideoID]
		watchedAts[i] = v.WatchedAt
	}

	// One query to find already-existing history rows; covered by youtube_history_video_watched_idx
	existRows, err := db.Query(ctx, `
		SELECT h.id_youtube_video, h.watched_at
		FROM youtube_history h
		JOIN UNNEST($1::bigint[], $2::timestamptz[]) AS t(vid_id, wat)
		    ON h.id_youtube_video = t.vid_id AND h.watched_at = t.wat`,
		intIDs, watchedAts,
	)
	if err != nil {
		return 0, fmt.Errorf("check existing history: %w", err)
	}

	type histKey struct {
		id int64
		wa time.Time
	}
	existing := make(map[histKey]struct{})
	for existRows.Next() {
		var k histKey
		if scanErr := existRows.Scan(&k.id, &k.wa); scanErr != nil {
			existRows.Close()
			return 0, fmt.Errorf("scan existing history: %w", scanErr)
		}
		existing[k] = struct{}{}
	}
	existRows.Close()
	if err = existRows.Err(); err != nil {
		return 0, fmt.Errorf("read existing history: %w", err)
	}

	var newIDs []int64
	var newWAs []time.Time
	for i, v := range batch {
		if _, ok := existing[histKey{intIDs[i], v.WatchedAt}]; !ok {
			newIDs = append(newIDs, intIDs[i])
			newWAs = append(newWAs, v.WatchedAt)
		}
	}
	if len(newIDs) == 0 {
		return 0, nil
	}

	// Bulk insert new history rows; ON CONFLICT DO NOTHING guards against duplicates within a batch
	tag, err := db.Exec(ctx, `
		INSERT INTO youtube_history (id_youtube_video, watched_at)
		SELECT * FROM UNNEST($1::bigint[], $2::timestamptz[]) AS t(id_youtube_video, watched_at)
		ON CONFLICT (id_youtube_video, watched_at) DO NOTHING`,
		newIDs, newWAs,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk insert history: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
