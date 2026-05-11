package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type VideoDetails struct {
	ThumbnailURL    string
	Description     string
	DurationSeconds int
	PublishedAt     time.Time
	ViewCount       int64
	LikeCount       int64
	Tags            []string
}

type apiResponse struct {
	Items []apiItem `json:"items"`
}

type apiItem struct {
	ID      string      `json:"id"`
	Snippet apiSnippet  `json:"snippet"`
	Content apiContent  `json:"contentDetails"`
	Stats   apiStats    `json:"statistics"`
}

type apiSnippet struct {
	PublishedAt string            `json:"publishedAt"`
	Description string            `json:"description"`
	Thumbnails  map[string]apiThumb `json:"thumbnails"`
	Tags        []string          `json:"tags"`
}

type apiThumb struct {
	URL string `json:"url"`
}

type apiContent struct {
	Duration string `json:"duration"` // ISO 8601, e.g. PT4M13S
}

type apiStats struct {
	ViewCount string `json:"viewCount"`
	LikeCount string `json:"likeCount"`
}

// Enrich fetches metadata for up to 50 video IDs per call from the YouTube
// Data API v3 and returns a map keyed by video ID. Videos not found in the
// response are absent from the map (not an error).
func Enrich(ctx context.Context, apiKey string, videoIDs []string) (map[string]VideoDetails, error) {
	if len(videoIDs) == 0 {
		return map[string]VideoDetails{}, nil
	}
	if len(videoIDs) > 50 {
		return nil, fmt.Errorf("enrich: max 50 IDs per call, got %d", len(videoIDs))
	}

	u := &url.URL{
		Scheme: "https",
		Host:   "www.googleapis.com",
		Path:   "/youtube/v3/videos",
	}
	q := u.Query()
	q.Set("part", "snippet,contentDetails,statistics")
	q.Set("id", strings.Join(videoIDs, ","))
	q.Set("key", apiKey)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("enrich: build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("enrich: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("enrich: API returned %d", resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("enrich: decode response: %w", err)
	}

	result := make(map[string]VideoDetails, len(apiResp.Items))
	for _, item := range apiResp.Items {
		d := VideoDetails{
			Description: item.Snippet.Description,
			Tags:        item.Snippet.Tags,
		}

		// thumbnail — prefer maxres, fall back through hq → mq → default
		for _, key := range []string{"maxres", "high", "medium", "default"} {
			if t, ok := item.Snippet.Thumbnails[key]; ok && t.URL != "" {
				d.ThumbnailURL = t.URL
				break
			}
		}

		if t, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt); err == nil {
			d.PublishedAt = t
		}

		d.DurationSeconds = parseDuration(item.Content.Duration)

		if n, err := strconv.ParseInt(item.Stats.ViewCount, 10, 64); err == nil {
			d.ViewCount = n
		}
		if n, err := strconv.ParseInt(item.Stats.LikeCount, 10, 64); err == nil {
			d.LikeCount = n
		}

		result[item.ID] = d
	}
	return result, nil
}

// iso8601Re matches PT[nH][nM][nS] — all components optional.
var iso8601Re = regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)

func parseDuration(iso string) int {
	m := iso8601Re.FindStringSubmatch(iso)
	if m == nil {
		return 0
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	s, _ := strconv.Atoi(m[3])
	return h*3600 + min*60 + s
}
