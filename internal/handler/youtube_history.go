package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// -- response types --

type youtubeVideoResp struct {
	UUID            string         `json:"uuid"`
	ID              string         `json:"id"`
	Type            string         `json:"type"`
	URL             string         `json:"url"`
	Title           string         `json:"title"`
	Channel         *string        `json:"channel"`
	ThumbnailURL    *string        `json:"thumbnail_url"`
	Description     *string        `json:"description"`
	Duration        *durationResp  `json:"duration"`
	PublishedAt     *timestampResp `json:"published_at"`
	ViewCount       *int64         `json:"view_count"`
	LikeCount       *int64         `json:"like_count"`
	Tags            []string       `json:"tags"`
}

type youtubeHistoryItemResp struct {
	UUID      string           `json:"uuid"`
	Active    bool             `json:"active"`
	WatchedAt timestampResp    `json:"watched_at"`
	Video     youtubeVideoResp `json:"video"`
}

type timestampResp struct {
	Timestamp int64  `json:"timestamp"`
	Formatted string `json:"formatted"`
}

type durationResp struct {
	TotalSeconds int    `json:"total_seconds"`
	Formatted    string `json:"formatted"`
}

type paginationResp struct {
	PagesTotal   int `json:"pages_total"`
	ItemsTotal   int `json:"items_total"`
	Page         int `json:"page"`
	ItemsPerPage int `json:"items_per_page"`
}

type listResp struct {
	Items      []youtubeHistoryItemResp `json:"items"`
	Pagination paginationResp           `json:"pagination"`
}

// -- helpers --

func formatWatchedAt(t time.Time) timestampResp {
	return timestampResp{
		Timestamp: t.Unix(),
		Formatted: t.UTC().Format("2 Jan 2006, 15:04"),
	}
}

func formatPublishedAt(t time.Time) *timestampResp {
	r := timestampResp{
		Timestamp: t.Unix(),
		Formatted: t.UTC().Format("2 Jan 2006"),
	}
	return &r
}

func formatDuration(seconds int) *durationResp {
	m := seconds / 60
	s := seconds % 60
	return &durationResp{
		TotalSeconds: seconds,
		Formatted:    fmt.Sprintf("%d:%02d", m, s),
	}
}

func videoURL(videoID, videoType string) string {
	if videoType == "short" {
		return "https://www.youtube.com/shorts/" + videoID
	}
	return "https://www.youtube.com/watch?v=" + videoID
}

func parsePage(r *http.Request) (page, itemsPerPage int) {
	page = 1
	itemsPerPage = 50
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("items_per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			itemsPerPage = n
		}
	}
	return
}

func parseSort(r *http.Request) string {
	if r.URL.Query().Get("sort") == "asc" {
		return "ASC"
	}
	return "DESC"
}

const listQuery = `
SELECT
    h.uuid, h.active, h.watched_at,
    v.uuid, v.video_id, v.type, v.title, v.channel,
    v.thumbnail_url, v.description, v.duration_seconds,
    v.published_at, v.view_count, v.like_count, v.tags
FROM youtube_history h
JOIN youtube_video v ON v.id = h.id_youtube_video
%s
ORDER BY h.watched_at %s
LIMIT $1 OFFSET $2`

const countQuery = `
SELECT COUNT(*) FROM youtube_history h %s`

func scanRows(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]youtubeHistoryItemResp, error) {
	var items []youtubeHistoryItemResp
	for rows.Next() {
		var (
			huuid     string
			active    bool
			watchedAt time.Time
			vuuid     string
			videoID   string
			vtype     string
			title     string
			channel   *string
			thumbURL  *string
			desc      *string
			durSec    *int
			pubAt     *time.Time
			viewCnt   *int64
			likeCnt   *int64
			tags      []string
		)
		if err := rows.Scan(
			&huuid, &active, &watchedAt,
			&vuuid, &videoID, &vtype, &title, &channel,
			&thumbURL, &desc, &durSec,
			&pubAt, &viewCnt, &likeCnt, &tags,
		); err != nil {
			return nil, err
		}

		var dur *durationResp
		if durSec != nil {
			dur = formatDuration(*durSec)
		}
		var pub *timestampResp
		if pubAt != nil {
			pub = formatPublishedAt(*pubAt)
		}

		items = append(items, youtubeHistoryItemResp{
			UUID:      huuid,
			Active:    active,
			WatchedAt: formatWatchedAt(watchedAt),
			Video: youtubeVideoResp{
				UUID:         vuuid,
				ID:           videoID,
				Type:         vtype,
				URL:          videoURL(videoID, vtype),
				Title:        title,
				Channel:      channel,
				ThumbnailURL: thumbURL,
				Description:  desc,
				Duration:     dur,
				PublishedAt:  pub,
				ViewCount:    viewCnt,
				LikeCount:    likeCnt,
				Tags:         tags,
			},
		})
	}
	return items, rows.Err()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func listHandler(log *slog.Logger, db *pgxpool.Pool, where string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, ipp := parsePage(r)
		sort := parseSort(r)
		offset := (page - 1) * ipp

		var total int
		cq := fmt.Sprintf(countQuery, where)
		if err := db.QueryRow(r.Context(), cq).Scan(&total); err != nil {
			log.Error("count youtube_history", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		lq := fmt.Sprintf(listQuery, where, sort)
		rows, err := db.Query(r.Context(), lq, ipp, offset)
		if err != nil {
			log.Error("query youtube_history", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		items, err := scanRows(rows)
		if err != nil {
			log.Error("scan youtube_history", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []youtubeHistoryItemResp{}
		}

		pages := int(math.Ceil(float64(total) / float64(ipp)))
		writeJSON(w, http.StatusOK, listResp{
			Items: items,
			Pagination: paginationResp{
				PagesTotal:   pages,
				ItemsTotal:   total,
				Page:         page,
				ItemsPerPage: ipp,
			},
		})
	}
}

// -- handlers --

func ListActiveYoutubeHistory(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return listHandler(log, db, "WHERE h.active = TRUE")
}

func ListAllYoutubeHistory(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return listHandler(log, db, "")
}

func GetYoutubeHistory(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := chi.URLParam(r, "uuid")

		const q = `
SELECT
    h.uuid, h.active, h.watched_at,
    v.uuid, v.video_id, v.type, v.title, v.channel,
    v.thumbnail_url, v.description, v.duration_seconds,
    v.published_at, v.view_count, v.like_count, v.tags
FROM youtube_history h
JOIN youtube_video v ON v.id = h.id_youtube_video
WHERE h.uuid = $1`

		var (
			huuid     string
			active    bool
			watchedAt time.Time
			vuuid     string
			videoID   string
			vtype     string
			title     string
			channel   *string
			thumbURL  *string
			desc      *string
			durSec    *int
			pubAt     *time.Time
			viewCnt   *int64
			likeCnt   *int64
			tags      []string
		)
		err := db.QueryRow(r.Context(), q, uuid).Scan(
			&huuid, &active, &watchedAt,
			&vuuid, &videoID, &vtype, &title, &channel,
			&thumbURL, &desc, &durSec,
			&pubAt, &viewCnt, &likeCnt, &tags,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
			log.Error("get youtube_history", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		var dur *durationResp
		if durSec != nil {
			dur = formatDuration(*durSec)
		}
		var pub *timestampResp
		if pubAt != nil {
			pub = formatPublishedAt(*pubAt)
		}

		writeJSON(w, http.StatusOK, youtubeHistoryItemResp{
			UUID:      huuid,
			Active:    active,
			WatchedAt: formatWatchedAt(watchedAt),
			Video: youtubeVideoResp{
				UUID:         vuuid,
				ID:           videoID,
				Type:         vtype,
				URL:          videoURL(videoID, vtype),
				Title:        title,
				Channel:      channel,
				ThumbnailURL: thumbURL,
				Description:  desc,
				Duration:     dur,
				PublishedAt:  pub,
				ViewCount:    viewCnt,
				LikeCount:    likeCnt,
				Tags:         tags,
			},
		})
	}
}

func ActivateYoutubeHistory(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return setActive(log, db, true)
}

func DeactivateYoutubeHistory(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return setActive(log, db, false)
}

func setActive(log *slog.Logger, db *pgxpool.Pool, active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := chi.URLParam(r, "uuid")

		tag, err := db.Exec(r.Context(),
			`UPDATE youtube_history SET active = $1 WHERE uuid = $2`,
			active, uuid,
		)
		if err != nil {
			log.Error("update youtube_history active", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"uuid": uuid, "active": active})
	}
}
