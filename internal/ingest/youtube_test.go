package ingest

import (
	"encoding/json"
	"testing"
	"time"
)

func buildJSON(entries []map[string]any) []byte {
	b, _ := json.Marshal(entries)
	return b
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		wantID   string
		wantType string
		wantOK   bool
	}{
		{
			name:     "standard watch URL",
			rawURL:   "https://www.youtube.com/watch?v=yJANVlPeb8Q",
			wantID:   "yJANVlPeb8Q",
			wantType: "video",
			wantOK:   true,
		},
		{
			name:     "shorts URL",
			rawURL:   "https://www.youtube.com/shorts/abc123XYZ",
			wantID:   "abc123XYZ",
			wantType: "short",
			wantOK:   true,
		},
		{
			name:   "missing v param",
			rawURL: "https://www.youtube.com/watch",
			wantOK: false,
		},
		{
			name:   "empty URL",
			rawURL: "",
			wantOK: false,
		},
		{
			name:   "unrelated URL",
			rawURL: "https://www.youtube.com/channel/UCy9dgKMoBbFJ4B9E560mSUQ",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, vtype, ok := ExtractVideoID(tc.rawURL)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if id != tc.wantID {
				t.Errorf("id = %q, want %q", id, tc.wantID)
			}
			if vtype != tc.wantType {
				t.Errorf("type = %q, want %q", vtype, tc.wantType)
			}
		})
	}
}

func TestParseEntries_BasicVideo(t *testing.T) {
	data := buildJSON([]map[string]any{{
		"header":    "YouTube",
		"title":     "Watched Nejsilnější automobilový zážitek mého života",
		"titleUrl":  "https://www.youtube.com/watch?v=yJANVlPeb8Q",
		"subtitles": []map[string]any{{"name": "Autokult CZ", "url": "https://youtube.com/channel/UC123"}},
		"time":      "2026-05-10T21:28:07.307Z",
	}})

	videos, err := ParseEntries(data, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 1 {
		t.Fatalf("got %d videos, want 1", len(videos))
	}
	v := videos[0]
	if v.VideoID != "yJANVlPeb8Q" {
		t.Errorf("VideoID = %q", v.VideoID)
	}
	if v.Type != "video" {
		t.Errorf("Type = %q", v.Type)
	}
	if v.Title != "Nejsilnější automobilový zážitek mého života" {
		t.Errorf("Title = %q (Watched prefix not stripped?)", v.Title)
	}
	if v.Channel == nil || *v.Channel != "Autokult CZ" {
		t.Errorf("Channel = %v", v.Channel)
	}
	if !v.WatchedAt.Equal(mustTime("2026-05-10T21:28:07.307Z")) {
		t.Errorf("WatchedAt = %v", v.WatchedAt)
	}
}

func TestParseEntries_Short(t *testing.T) {
	data := buildJSON([]map[string]any{{
		"header":    "YouTube",
		"title":     "Watched Some Short",
		"titleUrl":  "https://www.youtube.com/shorts/abc123",
		"subtitles": []map[string]any{{"name": "Creator", "url": "https://youtube.com/channel/UC456"}},
		"time":      "2026-05-10T10:00:00.000Z",
	}})

	videos, err := ParseEntries(data, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 1 {
		t.Fatalf("got %d videos, want 1", len(videos))
	}
	if videos[0].Type != "short" {
		t.Errorf("Type = %q, want short", videos[0].Type)
	}
	if videos[0].VideoID != "abc123" {
		t.Errorf("VideoID = %q", videos[0].VideoID)
	}
}

func TestParseEntries_SkipMissingTitleURL(t *testing.T) {
	data := buildJSON([]map[string]any{
		{
			"header": "YouTube",
			"title":  "Watched Deleted Video",
			// no titleUrl — deleted video
			"time": "2026-05-10T10:00:00.000Z",
		},
		{
			"header":   "YouTube",
			"title":    "Watched Real Video",
			"titleUrl": "https://www.youtube.com/watch?v=abc123",
			"time":     "2026-05-10T11:00:00.000Z",
		},
	})

	videos, err := ParseEntries(data, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 1 {
		t.Fatalf("got %d videos, want 1 (deleted should be skipped)", len(videos))
	}
	if videos[0].VideoID != "abc123" {
		t.Errorf("wrong video kept: %q", videos[0].VideoID)
	}
}

func TestParseEntries_NilChannelWhenNoSubtitles(t *testing.T) {
	data := buildJSON([]map[string]any{{
		"header":   "YouTube",
		"title":    "Watched Unavailable Channel Video",
		"titleUrl": "https://www.youtube.com/watch?v=xyz789",
		"time":     "2026-05-10T10:00:00.000Z",
	}})

	videos, err := ParseEntries(data, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 1 {
		t.Fatalf("got %d videos, want 1", len(videos))
	}
	if videos[0].Channel != nil {
		t.Errorf("Channel = %v, want nil", *videos[0].Channel)
	}
}

func TestParseEntries_DaysFilter(t *testing.T) {
	now := time.Now().UTC()
	data := buildJSON([]map[string]any{
		{
			"header":   "YouTube",
			"title":    "Watched Old Video",
			"titleUrl": "https://www.youtube.com/watch?v=old111",
			"time":     now.AddDate(0, 0, -10).Format(time.RFC3339Nano),
		},
		{
			"header":   "YouTube",
			"title":    "Watched Recent Video",
			"titleUrl": "https://www.youtube.com/watch?v=recent2",
			"time":     now.AddDate(0, 0, -2).Format(time.RFC3339Nano),
		},
	})

	cutoff := now.AddDate(0, 0, -5)
	videos, err := ParseEntries(data, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 1 {
		t.Fatalf("got %d videos, want 1 (old video should be filtered)", len(videos))
	}
	if videos[0].VideoID != "recent2" {
		t.Errorf("wrong video kept: %q", videos[0].VideoID)
	}
}

func TestParseEntries_TitleWithSpecialChars(t *testing.T) {
	data := buildJSON([]map[string]any{{
		"header":    "YouTube",
		"title":     "Watched This Could Be The BEST Monitor of 2026... (for Mac & PC)",
		"titleUrl":  "https://www.youtube.com/watch?v=xoGkilE00MY",
		"subtitles": []map[string]any{{"name": "Created Tech", "url": "https://youtube.com/channel/UC789"}},
		"time":      "2026-05-10T21:16:51.862Z",
	}})

	videos, err := ParseEntries(data, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 1 {
		t.Fatalf("got %d videos, want 1", len(videos))
	}
	want := "This Could Be The BEST Monitor of 2026... (for Mac & PC)"
	if videos[0].Title != want {
		t.Errorf("Title = %q, want %q", videos[0].Title, want)
	}
}

func TestParseEntries_EmptyInput(t *testing.T) {
	videos, err := ParseEntries([]byte("[]"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 0 {
		t.Errorf("got %d videos, want 0", len(videos))
	}
}

func TestParseEntries_InvalidJSON(t *testing.T) {
	_, err := ParseEntries([]byte("{not valid json"), time.Time{})
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}
