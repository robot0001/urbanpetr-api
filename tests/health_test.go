package tests

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/robot0001/urbanpetr-api/internal/handler"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("app", "urbanpetr-api", "env", "test")
}

func TestHealthHandler(t *testing.T) {
	r := handler.NewRouter(testLogger(), nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func preflight(t *testing.T, origin string) *httptest.ResponseRecorder {
	t.Helper()
	r := handler.NewRouter(testLogger(), nil, nil)
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHealthHandlerCORSAllowed(t *testing.T) {
	origins := []string{
		"https://urbanpetr.com",
		"https://stage1.urbanpetr.com",
	}
	for _, origin := range origins {
		t.Run(origin, func(t *testing.T) {
			w := preflight(t, origin)
			if w.Header().Get("Access-Control-Allow-Origin") != origin {
				t.Fatalf("expected CORS header for %s, got %q", origin, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestHealthHandlerCORSBlocked(t *testing.T) {
	origins := []string{
		"https://example.com",
		"http://urbanpetr.com",            // wrong scheme
		"https://urbanpetr.com.evil.com",  // subdomain spoofing
	}
	for _, origin := range origins {
		t.Run(origin, func(t *testing.T) {
			w := preflight(t, origin)
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Fatalf("expected no CORS header for %s, got %q", origin, got)
			}
		})
	}
}
