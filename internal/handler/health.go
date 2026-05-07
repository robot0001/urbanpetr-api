package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

var healthPadding = strings.Repeat("healthcheck-padding-", 500)

func HealthHandler(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("hello guys")
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"status": "ok", "padding": healthPadding}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
