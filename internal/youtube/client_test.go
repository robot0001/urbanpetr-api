package youtube

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func stubServer(t *testing.T, items []apiItem) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(apiResponse{Items: items})
	}))
}

func clientForServer(srv *httptest.Server) *Client {
	c := New("fake-key")
	c.baseURL = srv.URL
	return c
}

func TestEnrich_BasicVideo(t *testing.T) {
	srv := stubServer(t, []apiItem{{
		ID: "abc123",
		Snippet: apiSnippet{
			PublishedAt: "2021-06-07T12:05:45Z",
			Description: "A great video",
			Thumbnails:  map[string]apiThumb{"maxres": {URL: "https://i.ytimg.com/vi/abc123/maxresdefault.jpg"}},
			Tags:        []string{"go", "programming"},
		},
		Content: apiContent{Duration: "PT20M53S"},
		Stats:   apiStats{ViewCount: "482301", LikeCount: "9823"},
	}})
	defer srv.Close()

	result, err := clientForServer(srv).Enrich(t.Context(), []string{"abc123"})
	if err != nil {
		t.Fatal(err)
	}
	d, ok := result["abc123"]
	if !ok {
		t.Fatal("abc123 not in result")
	}
	if d.ThumbnailURL != "https://i.ytimg.com/vi/abc123/maxresdefault.jpg" {
		t.Errorf("ThumbnailURL = %q", d.ThumbnailURL)
	}
	if d.DurationSeconds != 1253 {
		t.Errorf("DurationSeconds = %d, want 1253 (20m53s)", d.DurationSeconds)
	}
	if d.ViewCount != 482301 {
		t.Errorf("ViewCount = %d", d.ViewCount)
	}
	if d.LikeCount != 9823 {
		t.Errorf("LikeCount = %d", d.LikeCount)
	}
	if len(d.Tags) != 2 || d.Tags[0] != "go" {
		t.Errorf("Tags = %v", d.Tags)
	}
	if d.PublishedAt.Year() != 2021 {
		t.Errorf("PublishedAt = %v", d.PublishedAt)
	}
}

func TestEnrich_ThumbnailFallback(t *testing.T) {
	srv := stubServer(t, []apiItem{{
		ID: "xyz",
		Snippet: apiSnippet{
			Thumbnails: map[string]apiThumb{
				"medium":  {URL: "https://example.com/mq.jpg"},
				"default": {URL: "https://example.com/default.jpg"},
			},
		},
		Content: apiContent{Duration: "PT1S"},
	}})
	defer srv.Close()

	result, err := clientForServer(srv).Enrich(t.Context(), []string{"xyz"})
	if err != nil {
		t.Fatal(err)
	}
	if result["xyz"].ThumbnailURL != "https://example.com/mq.jpg" {
		t.Errorf("expected medium fallback, got %q", result["xyz"].ThumbnailURL)
	}
}

func TestEnrich_VideoNotFound(t *testing.T) {
	srv := stubServer(t, []apiItem{})
	defer srv.Close()

	result, err := clientForServer(srv).Enrich(t.Context(), []string{"nonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result["nonexistent"]; ok {
		t.Error("expected missing key for unknown video")
	}
}

func TestEnrich_EmptyInput(t *testing.T) {
	c := New("fake-key")
	result, err := c.Enrich(t.Context(), []string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestEnrich_TooManyIDs(t *testing.T) {
	ids := make([]string, 51)
	_, err := New("fake-key").Enrich(t.Context(), ids)
	if err == nil {
		t.Error("expected error for >50 IDs")
	}
}

func TestEnrich_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := clientForServer(srv).Enrich(t.Context(), []string{"abc"})
	if err == nil {
		t.Error("expected error for non-200 response")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		iso  string
		want int
	}{
		{"PT20M53S", 1253},
		{"PT4M13S", 253},
		{"PT1H", 3600},
		{"PT1H30M", 5400},
		{"PT1H30M45S", 5445},
		{"PT59S", 59},
		{"PT0S", 0},
		{"", 0},
		{"invalid", 0},
	}
	for _, tc := range tests {
		if got := parseDuration(tc.iso); got != tc.want {
			t.Errorf("parseDuration(%q) = %d, want %d", tc.iso, got, tc.want)
		}
	}
}
