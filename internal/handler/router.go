package handler

import (
	"log/slog"
	"net/http"
	"os"
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
		origins = append(origins, "http://localhost:3000", "http://localhost:*")
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Compress(5))
	r.Use(requestLogger(log))
	r.Use(recoverer(log))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         300,
	}))
	r.Get("/health", HealthHandler(log))
	r.Get("/v1/history/youtube", ListActiveYoutubeHistory(log, db))

	r.Group(func(r chi.Router) {
		r.Use(jwtMiddleware.Require("urbanpetr_admin"))
		r.Get("/v1/history/youtube/all",                ListAllYoutubeHistory(log, db))
		r.Get("/v1/history/youtube/{uuid}",             GetYoutubeHistory(log, db))
		r.Post("/v1/history/youtube/{uuid}/activate",   ActivateYoutubeHistory(log, db))
		r.Post("/v1/history/youtube/{uuid}/deactivate", DeactivateYoutubeHistory(log, db))
		r.Post("/v1/history/youtube/{uuid}/enrich",     EnrichYoutubeVideo(log, db, yt))
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
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
