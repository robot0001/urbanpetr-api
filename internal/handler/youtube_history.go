package handler

import (
	"context"
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
	"github.com/robot0001/urbanpetr-api/internal/youtube"
)

// -- response types --

type youtubeVideoResp struct {
	UUID         string         `json:"uuid"`
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	URL          string         `json:"url"`
	Title        string         `json:"title"`
	Channel      *string        `json:"channel"`
	ChannelURL   *string        `json:"channel_url"`
	ThumbnailURL *string        `json:"thumbnail_url"`
	Description  *string        `json:"description"`
	Duration     *durationResp  `json:"duration"`
	PublishedAt  *timestampResp `json:"published_at"`
	ViewCount    *int64         `json:"view_count"`
	LikeCount    *int64         `json:"like_count"`
	Tags         []string       `json:"tags"`
}

type youtubeHistoryItemResp struct {
	UUID       string           `json:"uuid"`
	Active     bool             `json:"active"`
	WatchedAt  timestampResp    `json:"watched_at"`
	Comment    *string          `json:"comment"`
	CustomTags []string         `json:"custom_tags"`
	Video      youtubeVideoResp `json:"video"`
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
    h.uuid, h.active, h.watched_at, h.comment, h.custom_tags,
    v.uuid, v.video_id, v.type, v.title, v.channel, v.channel_url,
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
			huuid      string
			active     bool
			watchedAt  time.Time
			comment    *string
			customTags []string
			vuuid      string
			videoID    string
			vtype      string
			title      string
			channel    *string
			channelURL *string
			thumbURL   *string
			desc       *string
			durSec     *int
			pubAt      *time.Time
			viewCnt    *int64
			likeCnt    *int64
			tags       []string
		)
		if err := rows.Scan(
			&huuid, &active, &watchedAt, &comment, &customTags,
			&vuuid, &videoID, &vtype, &title, &channel, &channelURL,
			&thumbURL, &desc, &durSec,
			&pubAt, &viewCnt, &likeCnt, &tags,
		); err != nil {
			return nil, err
		}

		if customTags == nil {
			customTags = []string{}
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
			UUID:       huuid,
			Active:     active,
			WatchedAt:  formatWatchedAt(watchedAt),
			Comment:    comment,
			CustomTags: customTags,
			Video: youtubeVideoResp{
				UUID:         vuuid,
				ID:           videoID,
				Type:         vtype,
				URL:          videoURL(videoID, vtype),
				Title:        title,
				Channel:      channel,
				ChannelURL:   channelURL,
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
	_ = json.NewEncoder(w).Encode(v)
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

func parseVideoType(r *http.Request) *string {
	vt := r.URL.Query().Get("type")
	if vt == "video" || vt == "short" {
		return &vt
	}
	return nil
}

func ListAllYoutubeHistory(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, ipp := parsePage(r)
		sortDir := parseSort(r)
		offset := (page - 1) * ipp
		vtype := parseVideoType(r)

		var total int
		if vtype != nil {
			if err := db.QueryRow(r.Context(), `
				SELECT COUNT(*) FROM youtube_history h
				JOIN youtube_video v ON v.id = h.id_youtube_video
				WHERE v.type = $1::youtube_video_type`,
				*vtype,
			).Scan(&total); err != nil {
				log.Error("count youtube_history", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		} else {
			if err := db.QueryRow(r.Context(), `SELECT COUNT(*) FROM youtube_history h`).Scan(&total); err != nil {
				log.Error("count youtube_history", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		whereClause := ""
		listArgs := []any{ipp, offset}
		if vtype != nil {
			whereClause = "WHERE v.type = $3::youtube_video_type"
			listArgs = append(listArgs, *vtype)
		}
		lq := fmt.Sprintf(`
SELECT
    h.uuid, h.active, h.watched_at, h.comment, h.custom_tags,
    v.uuid, v.video_id, v.type, v.title, v.channel, v.channel_url,
    v.thumbnail_url, v.description, v.duration_seconds,
    v.published_at, v.view_count, v.like_count, v.tags
FROM youtube_history h
JOIN youtube_video v ON v.id = h.id_youtube_video
%s
ORDER BY h.watched_at %s
LIMIT $1 OFFSET $2`, whereClause, sortDir)

		rows, err := db.Query(r.Context(), lq, listArgs...)
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

func GetYoutubeHistory(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := chi.URLParam(r, "uuid")

		const q = `
SELECT
    h.uuid, h.active, h.watched_at, h.comment, h.custom_tags,
    v.uuid, v.video_id, v.type, v.title, v.channel, v.channel_url,
    v.thumbnail_url, v.description, v.duration_seconds,
    v.published_at, v.view_count, v.like_count, v.tags
FROM youtube_history h
JOIN youtube_video v ON v.id = h.id_youtube_video
WHERE h.uuid = $1`

		var (
			huuid      string
			active     bool
			watchedAt  time.Time
			comment    *string
			customTags []string
			vuuid      string
			videoID    string
			vtype      string
			title      string
			channel    *string
			channelURL *string
			thumbURL   *string
			desc       *string
			durSec     *int
			pubAt      *time.Time
			viewCnt    *int64
			likeCnt    *int64
			tags       []string
		)
		err := db.QueryRow(r.Context(), q, uuid).Scan(
			&huuid, &active, &watchedAt, &comment, &customTags,
			&vuuid, &videoID, &vtype, &title, &channel, &channelURL,
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

		if customTags == nil {
			customTags = []string{}
		}

		writeJSON(w, http.StatusOK, youtubeHistoryItemResp{
			UUID:       huuid,
			Active:     active,
			WatchedAt:  formatWatchedAt(watchedAt),
			Comment:    comment,
			CustomTags: customTags,
			Video: youtubeVideoResp{
				UUID:         vuuid,
				ID:           videoID,
				Type:         vtype,
				URL:          videoURL(videoID, vtype),
				Title:        title,
				Channel:      channel,
				ChannelURL:   channelURL,
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

const autoEnrichLimit = 500

func applyEnrichment(ctx context.Context, db *pgxpool.Pool, dbID int64, currentType string, d youtube.VideoDetails) error {
	var thumbnailURL *string
	if d.ThumbnailURL != "" {
		thumbnailURL = &d.ThumbnailURL
	}
	var description *string
	if d.Description != "" {
		description = &d.Description
	}
	var durationSeconds *int
	if d.DurationSeconds > 0 {
		durationSeconds = &d.DurationSeconds
	}
	var publishedAt *time.Time
	if !d.PublishedAt.IsZero() {
		publishedAt = &d.PublishedAt
	}
	var viewCount *int64
	if d.ViewCount > 0 {
		viewCount = &d.ViewCount
	}
	var likeCount *int64
	if d.LikeCount > 0 {
		likeCount = &d.LikeCount
	}
	var tags []string
	if len(d.Tags) > 0 {
		tags = d.Tags
	}

	newType := currentType
	if currentType == "short" && d.DurationSeconds >= 150 {
		newType = "video"
	}
	if currentType == "video" && d.DurationSeconds > 0 && d.DurationSeconds < 150 {
		newType = "short"
	}

	_, err := db.Exec(ctx, `
		UPDATE youtube_video SET
			thumbnail_url    = $1,
			description      = $2,
			duration_seconds = $3,
			published_at     = $4,
			view_count       = $5,
			like_count       = $6,
			tags             = $7,
			type             = $8::youtube_video_type,
			enriched_at      = NOW()
		WHERE id = $9
	`, thumbnailURL, description, durationSeconds, publishedAt, viewCount, likeCount, tags, newType, dbID)
	return err
}

func enrichLatest(ctx context.Context, log *slog.Logger, db *pgxpool.Pool, yt *youtube.Client) int {
	if yt == nil {
		return 0
	}

	rows, err := db.Query(ctx, `
		WITH latest AS (
			SELECT v.id, v.video_id, v.type::text AS type,
			       MAX(h.watched_at) AS last_watched
			FROM youtube_history h
			JOIN youtube_video v ON v.id = h.id_youtube_video
			WHERE v.enriched_at IS NULL
			GROUP BY v.id, v.video_id, v.type
			ORDER BY last_watched DESC
			LIMIT $1
		)
		SELECT id, video_id, type FROM latest
	`, autoEnrichLimit)
	if err != nil {
		log.Error("enrichLatest: query", "error", err)
		return 0
	}

	type videoRow struct {
		dbID        int64
		videoID     string
		currentType string
	}
	var videos []videoRow
	for rows.Next() {
		var v videoRow
		if err := rows.Scan(&v.dbID, &v.videoID, &v.currentType); err != nil {
			rows.Close()
			log.Error("enrichLatest: scan", "error", err)
			return 0
		}
		videos = append(videos, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Error("enrichLatest: rows", "error", err)
		return 0
	}

	enriched := 0
	for i := 0; i < len(videos); i += 50 {
		chunk := videos[i:min(i+50, len(videos))]

		ids := make([]string, len(chunk))
		for j, v := range chunk {
			ids[j] = v.videoID
		}

		details, err := yt.Enrich(ctx, ids)
		if err != nil {
			log.Error("enrichLatest: youtube API", "error", err)
			continue
		}

		for _, v := range chunk {
			d, ok := details[v.videoID]
			if !ok {
				continue
			}
			if err := applyEnrichment(ctx, db, v.dbID, v.currentType, d); err != nil {
				log.Error("enrichLatest: update", "error", err, "video_id", v.videoID)
				continue
			}
			enriched++
		}
	}

	return enriched
}

func EnrichYoutubeVideo(log *slog.Logger, db *pgxpool.Pool, yt *youtube.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if yt == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "enrichment not configured"})
			return
		}

		uuid := chi.URLParam(r, "uuid")

		var targetDBID int64
		var targetVideoID, targetCurrentType string
		err := db.QueryRow(r.Context(), `
			SELECT v.id, v.video_id, v.type::text
			FROM youtube_history h
			JOIN youtube_video v ON v.id = h.id_youtube_video
			WHERE h.uuid = $1
		`, uuid).Scan(&targetDBID, &targetVideoID, &targetCurrentType)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
			log.Error("enrich: lookup video_id", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Fetch up to 50 unenriched videos of the same type at id <= target (descending),
		// so the batch covers the target and the videos just before it in insertion order.
		// If fewer than 50 are found, fill remaining slots from higher IDs.
		type videoRow struct {
			dbID        int64
			videoID     string
			currentType string
		}
		batchRows, err := db.Query(r.Context(), `
			SELECT id, video_id, type::text
			FROM youtube_video
			WHERE id <= $1
			  AND enriched_at IS NULL
			  AND type = $2::youtube_video_type
			ORDER BY id DESC
			LIMIT 50
		`, targetDBID, targetCurrentType)
		if err != nil {
			log.Error("enrich: batch query", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		var batch []videoRow
		for batchRows.Next() {
			var v videoRow
			if err := batchRows.Scan(&v.dbID, &v.videoID, &v.currentType); err != nil {
				batchRows.Close()
				log.Error("enrich: batch scan", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			batch = append(batch, v)
		}
		batchRows.Close()
		if err := batchRows.Err(); err != nil {
			log.Error("enrich: batch rows", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Fill remaining slots with unenriched videos above the target.
		if len(batch) < 50 {
			fillRows, err := db.Query(r.Context(), `
				SELECT id, video_id, type::text
				FROM youtube_video
				WHERE id > $1
				  AND enriched_at IS NULL
				  AND type = $2::youtube_video_type
				ORDER BY id
				LIMIT $3
			`, targetDBID, targetCurrentType, 50-len(batch))
			if err != nil {
				log.Error("enrich: batch fill query", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			for fillRows.Next() {
				var v videoRow
				if err := fillRows.Scan(&v.dbID, &v.videoID, &v.currentType); err != nil {
					fillRows.Close()
					log.Error("enrich: batch fill scan", "error", err)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				batch = append(batch, v)
			}
			fillRows.Close()
			if err := fillRows.Err(); err != nil {
				log.Error("enrich: batch fill rows", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		// Target may be absent when already enriched; always include it.
		targetInBatch := false
		for _, v := range batch {
			if v.dbID == targetDBID {
				targetInBatch = true
				break
			}
		}
		if !targetInBatch {
			batch = append([]videoRow{{targetDBID, targetVideoID, targetCurrentType}}, batch...)
			if len(batch) > 50 {
				batch = batch[:50]
			}
		}

		ids := make([]string, len(batch))
		for i, v := range batch {
			ids[i] = v.videoID
		}

		details, err := yt.Enrich(r.Context(), ids)
		if err != nil {
			log.Error("enrich: youtube API", "error", err, "video_id", targetVideoID)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		for _, v := range batch {
			d, ok := details[v.videoID]
			if !ok {
				// Video absent from YouTube response (deleted/private). Mark it so it no
				// longer occupies batch slots — the button still shows if thumbnail is
				// still missing, but only on an explicit re-click for that specific item.
				if v.dbID != targetDBID {
					if _, err := db.Exec(r.Context(),
						`UPDATE youtube_video SET enriched_at = NOW() WHERE id = $1`,
						v.dbID,
					); err != nil {
						log.Error("enrich: mark absent", "error", err, "video_id", v.videoID)
					}
				}
				continue
			}
			if err := applyEnrichment(r.Context(), db, v.dbID, v.currentType, d); err != nil {
				log.Error("enrich: db update", "error", err, "video_id", v.videoID)
			}
		}

		if _, ok := details[targetVideoID]; !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "video not found on YouTube"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "enriched", "video_id": targetVideoID})
	}
}

type activateBody struct {
	Comment    *string  `json:"comment"`
	CustomTags []string `json:"custom_tags"`
	VideoType  *string  `json:"video_type"`
}

func ActivateYoutubeHistory(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := chi.URLParam(r, "uuid")

		var body activateBody
		if r.ContentLength > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
				return
			}
		}
		if body.CustomTags == nil {
			body.CustomTags = []string{}
		}

		tag, err := db.Exec(r.Context(),
			`UPDATE youtube_history SET active = TRUE, comment = $1, custom_tags = $2 WHERE uuid = $3`,
			body.Comment, body.CustomTags, uuid,
		)
		if err != nil {
			log.Error("activate youtube_history", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"uuid": uuid, "active": true})
	}
}

func DeactivateYoutubeHistory(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return setActive(log, db, false)
}

func UpdateYoutubeHistoryDetails(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := chi.URLParam(r, "uuid")

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body activateBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if body.CustomTags == nil {
			body.CustomTags = []string{}
		}

		if body.VideoType != nil {
			vt := *body.VideoType
			if vt != "video" && vt != "short" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid video_type"})
				return
			}
			_, err := db.Exec(r.Context(), `
				UPDATE youtube_video SET type = $1::youtube_video_type
				WHERE id = (SELECT id_youtube_video FROM youtube_history WHERE uuid = $2)
			`, vt, uuid)
			if err != nil {
				log.Error("update video type", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		tag, err := db.Exec(r.Context(),
			`UPDATE youtube_history SET comment = $1, custom_tags = $2 WHERE uuid = $3`,
			body.Comment, body.CustomTags, uuid,
		)
		if err != nil {
			log.Error("update youtube_history details", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"uuid": uuid})
	}
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
