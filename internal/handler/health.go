package handler

import (
	"encoding/json"
	"net/http"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok", "env": "staging-test"}); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
