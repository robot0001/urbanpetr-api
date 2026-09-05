package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robot0001/urbanpetr-api/internal/auth"
	"github.com/robot0001/urbanpetr-api/internal/youtube"
)

func NewRouter(log *slog.Logger, db *pgxpool.Pool, yt *youtube.Client, jwtMiddleware *auth.JWTMiddleware) *chi.Mux {
	origins := []string{"https://urbanpetr.com", "https://*.urbanpetr.com"}
	if os.Getenv("ENVIRONMENT") == "local" {
		origins = append(origins,
			"http://localhost:3000",
			"http://localhost:*",
			"https://urbanpetr.home",
			"https://admin.urbanpetr.home",
		)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Compress(5))
	r.Use(joinACRH)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         300,
	}))
	r.Use(originSecretMiddleware(os.Getenv("ORIGIN_SECRET")))
	r.Use(requestLogger(log))
	r.Use(recoverer(log))
	r.Get("/health", HealthHandler(log))
	r.Get("/v1/history/youtube", ListActiveYoutubeHistory(log, db))

	r.Group(func(r chi.Router) {
		r.Use(jwtMiddleware.Require("urbanpetr_admin"))
		r.Get("/v1/history/youtube/all", ListAllYoutubeHistory(log, db))
		r.Get("/v1/history/youtube/{uuid}", GetYoutubeHistory(log, db))
		r.Post("/v1/history/youtube/ingest", IngestYoutubeHistory(log, db, yt))
		r.Post("/v1/history/youtube/{uuid}/activate", ActivateYoutubeHistory(log, db))
		r.Post("/v1/history/youtube/{uuid}/deactivate", DeactivateYoutubeHistory(log, db))
		r.Patch("/v1/history/youtube/{uuid}", UpdateYoutubeHistoryDetails(log, db))
		r.Post("/v1/history/youtube/{uuid}/enrich", EnrichYoutubeVideo(log, db, yt))
	})

	return r
}

func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"latency_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

func originSecretMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret != "" && r.Header.Get("X-Origin-Secret") != secret {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// joinACRH re-joins Access-Control-Request-Headers values before the CORS
// handler reads them. The aws-lambda-go-api-proxy adapter splits the
// comma-separated header list into multiple Go header values because
// Access-Control-Request-Headers is not in its singleton-header allowlist.
// go-chi/cors uses Header.Get() which returns only the first value, so
// without this fix preflight requests with two or more requested headers
// (e.g. "authorization, content-type") result in only the first being
// reflected in Access-Control-Allow-Headers.
func joinACRH(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vals := r.Header["Access-Control-Request-Headers"]; len(vals) > 1 {
			r.Header.Set("Access-Control-Request-Headers", strings.Join(vals, ", "))
		}
		next.ServeHTTP(w, r)
	})
}

func recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic",
						"error", rec,
						"method", r.Method,
						"path", r.URL.Path,
						"request_id", middleware.GetReqID(r.Context()),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
