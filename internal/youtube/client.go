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

const defaultBaseURL = "https://www.googleapis.com"

// Client calls the YouTube Data API v3. Construct with New.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// New returns a Client using the given API key.
func New(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// VideoDetails holds the enrichment data returned for a single video.
type VideoDetails struct {
	ThumbnailURL    string
	Description     string
	DurationSeconds int
	PublishedAt     time.Time
	ViewCount       int64
	LikeCount       int64
	Tags            []string
}

// Enrich fetches metadata for up to 50 video IDs and returns a map keyed
// by video ID. Videos absent from the API response are missing from the map.
func (c *Client) Enrich(ctx context.Context, videoIDs []string) (map[string]VideoDetails, error) {
	if len(videoIDs) == 0 {
		return map[string]VideoDetails{}, nil
	}
	if len(videoIDs) > 50 {
		return nil, fmt.Errorf("youtube: max 50 IDs per call, got %d", len(videoIDs))
	}

	u, err := url.Parse(c.baseURL + "/youtube/v3/videos")
	if err != nil {
		return nil, fmt.Errorf("youtube: build URL: %w", err)
	}
	q := u.Query()
	q.Set("part", "snippet,contentDetails,statistics")
	q.Set("id", strings.Join(videoIDs, ","))
	q.Set("key", c.apiKey)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("youtube: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube: API returned %d", resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("youtube: decode response: %w", err)
	}

	result := make(map[string]VideoDetails, len(apiResp.Items))
	for _, item := range apiResp.Items {
		result[item.ID] = item.toDetails()
	}
	return result, nil
}

// -- internal API types --

type apiResponse struct {
	Items []apiItem `json:"items"`
}

type apiItem struct {
	ID      string     `json:"id"`
	Snippet apiSnippet `json:"snippet"`
	Content apiContent `json:"contentDetails"`
	Stats   apiStats   `json:"statistics"`
}

type apiSnippet struct {
	PublishedAt string              `json:"publishedAt"`
	Description string              `json:"description"`
	Thumbnails  map[string]apiThumb `json:"thumbnails"`
	Tags        []string            `json:"tags"`
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

func (item apiItem) toDetails() VideoDetails {
	d := VideoDetails{
		Description:     item.Snippet.Description,
		Tags:            item.Snippet.Tags,
		DurationSeconds: parseDuration(item.Content.Duration),
	}

	// prefer maxres, fall back through high → medium → default
	for _, key := range []string{"maxres", "high", "medium", "default"} {
		if t, ok := item.Snippet.Thumbnails[key]; ok && t.URL != "" {
			d.ThumbnailURL = t.URL
			break
		}
	}

	if t, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt); err == nil {
		d.PublishedAt = t
	}
	if n, err := strconv.ParseInt(item.Stats.ViewCount, 10, 64); err == nil {
		d.ViewCount = n
	}
	if n, err := strconv.ParseInt(item.Stats.LikeCount, 10, 64); err == nil {
		d.LikeCount = n
	}

	return d
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
