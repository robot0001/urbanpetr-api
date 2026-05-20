package ingest

import (
	"encoding/json"
	"fmt"
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

func TestMarkShortsFromTiming(t *testing.T) {
	base := mustTime("2026-05-10T10:00:00.000Z")

	tests := []struct {
		name      string
		gaps      []time.Duration // gaps[i] = WatchedAt[i] - WatchedAt[i-1]; gaps[0] unused
		urlTypes  []string        // type detected from URL
		wantTypes []string
	}{
		{
			name:      "single video — no previous to compare",
			gaps:      []time.Duration{0},
			urlTypes:  []string{"video"},
			wantTypes: []string{"video"},
		},
		{
			name:      "gap under 150s marks as short",
			gaps:      []time.Duration{0, 149 * time.Second},
			urlTypes:  []string{"video", "video"},
			wantTypes: []string{"video", "short"},
		},
		{
			name:      "gap exactly 150s is not a short",
			gaps:      []time.Duration{0, 150 * time.Second},
			urlTypes:  []string{"video", "video"},
			wantTypes: []string{"video", "video"},
		},
		{
			name:      "gap over 150s is not a short",
			gaps:      []time.Duration{0, 5 * time.Minute},
			urlTypes:  []string{"video", "video"},
			wantTypes: []string{"video", "video"},
		},
		{
			name:      "already-short from URL is unchanged",
			gaps:      []time.Duration{0, 30 * time.Second},
			urlTypes:  []string{"video", "short"},
			wantTypes: []string{"video", "short"},
		},
		{
			name:      "chain: middle video suppressed by exit rule, last one qualifies",
			gaps:      []time.Duration{0, 10 * time.Second, 15 * time.Second, 5 * time.Minute},
			urlTypes:  []string{"video", "video", "video", "video"},
			wantTypes: []string{"video", "video", "short", "video"},
		},
		{
			name:      "exit gap exactly 160s is not suppressed",
			gaps:      []time.Duration{0, 149 * time.Second, 160 * time.Second},
			urlTypes:  []string{"video", "video", "video"},
			wantTypes: []string{"video", "short", "video"},
		},
		{
			name:      "exit gap under 160s suppresses short",
			gaps:      []time.Duration{0, 149 * time.Second, 159 * time.Second},
			urlTypes:  []string{"video", "video", "video"},
			wantTypes: []string{"video", "video", "video"},
		},
		{
			name:      "last video in list has no exit gap — qualifies as short",
			gaps:      []time.Duration{0, 30 * time.Second},
			urlTypes:  []string{"video", "video"},
			wantTypes: []string{"video", "short"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			videos := make([]VideoInfo, len(tc.urlTypes))
			ts := base
			for i := range videos {
				if i > 0 {
					ts = ts.Add(tc.gaps[i])
				}
				videos[i] = VideoInfo{
					VideoID:   fmt.Sprintf("vid%d", i),
					Type:      tc.urlTypes[i],
					WatchedAt: ts,
				}
			}
			markShortsFromTiming(videos)
			for i, v := range videos {
				if v.Type != tc.wantTypes[i] {
					t.Errorf("videos[%d].Type = %q, want %q", i, v.Type, tc.wantTypes[i])
				}
			}
		})
	}
}

func TestParseEntries_TimingShortDetection(t *testing.T) {
	base := mustTime("2026-05-10T10:00:00.000Z")
	data := buildJSON([]map[string]any{
		{
			"title":    "Watched Long Video",
			"titleUrl": "https://www.youtube.com/watch?v=longvid1",
			"time":     base.Format(time.RFC3339Nano),
		},
		{
			// started 30s later — short via timing
			"title":    "Watched Quick Video",
			"titleUrl": "https://www.youtube.com/watch?v=quickvid",
			"time":     base.Add(30 * time.Second).Format(time.RFC3339Nano),
		},
		{
			// started 10 minutes later — regular video
			"title":    "Watched Later Video",
			"titleUrl": "https://www.youtube.com/watch?v=latervid",
			"time":     base.Add(10 * time.Minute).Format(time.RFC3339Nano),
		},
	})

	videos, err := ParseEntries(data, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 3 {
		t.Fatalf("got %d videos, want 3", len(videos))
	}
	// output is sorted ascending
	if videos[0].Type != "video" {
		t.Errorf("videos[0].Type = %q, want video", videos[0].Type)
	}
	if videos[1].Type != "short" {
		t.Errorf("videos[1].Type = %q, want short (< 150s gap)", videos[1].Type)
	}
	if videos[2].Type != "video" {
		t.Errorf("videos[2].Type = %q, want video (10m gap)", videos[2].Type)
	}
}

func TestParseEntries_HashtagShort(t *testing.T) {
	tests := []struct {
		title    string
		wantType string
	}{
		{"My video #shorts", "short"},
		{"My video #short", "short"},
		{"My video #Shorts", "short"},
		{"My video #SHORT", "short"},
		{"Normal video title", "video"},
	}
	for _, tc := range tests {
		data := buildJSON([]map[string]any{{
			"title":    "Watched " + tc.title,
			"titleUrl": "https://www.youtube.com/watch?v=abc123",
			"time":     "2026-05-10T10:00:00.000Z",
		}})
		videos, err := ParseEntries(data, time.Time{})
		if err != nil {
			t.Fatalf("%q: %v", tc.title, err)
		}
		if len(videos) != 1 {
			t.Fatalf("%q: got %d videos, want 1", tc.title, len(videos))
		}
		if videos[0].Type != tc.wantType {
			t.Errorf("%q: Type = %q, want %q", tc.title, videos[0].Type, tc.wantType)
		}
	}
}
