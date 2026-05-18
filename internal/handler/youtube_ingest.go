package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robot0001/urbanpetr-api/internal/ingest"
	"github.com/robot0001/urbanpetr-api/internal/youtube"
)

func IngestYoutubeHistory(log *slog.Logger, db *pgxpool.Pool, yt *youtube.Client) http.HandlerFunc {
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

		zipData, err := io.ReadAll(file)
		if err != nil {
			log.Error("ingest: read file", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		jsonData, err := extractJSONFromZip(zipData)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		videos, err := ingest.ParseEntries(jsonData, time.Time{})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid watch history JSON"})
			return
		}

		total, created, err := ingest.Ingest(r.Context(), db, videos)
		if err != nil {
			log.Error("ingest: db", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		enriched := enrichLatest(r.Context(), log, db, yt)
		writeJSON(w, http.StatusOK, map[string]int{"total": total, "created": created, "enriched": enriched})
	}
}

func extractJSONFromZip(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a valid zip file")
	}

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".json") {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("could not open %s in zip", f.Name)
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}

	return nil, fmt.Errorf("no JSON file found in zip")
}
