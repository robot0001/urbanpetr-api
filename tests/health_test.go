package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/robot0001/urbanpetr-api/internal/handler"
)

func TestHealthHandler(t *testing.T) {
	r := handler.NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHealthHandlerCORSPreflight(t *testing.T) {
	r := handler.NewRouter()
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://urbanpetr.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 preflight, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "https://urbanpetr.com" {
		t.Fatal("missing CORS allow-origin header")
	}
}
