package handler

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robot0001/urbanpetr-api/internal/ingest"
)

func IngestYoutubeHistory(log *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing file field"})
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			log.Error("ingest: read file", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		videos, err := ingest.ParseEntries(data, time.Time{})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid file format"})
			return
		}

		total, created, err := ingest.Ingest(r.Context(), db, videos)
		if err != nil {
			log.Error("ingest: db", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]int{"total": total, "created": created})
	}
}
