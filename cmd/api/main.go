package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robot0001/urbanpetr-api/internal/config"
	"github.com/robot0001/urbanpetr-api/internal/handler"
	"github.com/robot0001/urbanpetr-api/internal/migrate"
	"github.com/robot0001/urbanpetr-api/internal/seed"
	"github.com/robot0001/urbanpetr-api/internal/youtube"
)

func main() {
	appName := os.Getenv("APP_NAME")
	if appName == "" {
		appName = "urbanpetr-api"
	}
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "unknown"
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).
		With("app", appName, "env", env)

	localHTTP := os.Getenv("LOCAL_HTTP_MODE") == "true"
	mode := os.Getenv("LAMBDA_HANDLER_MODE")

	switch mode {
	case "migrate":
		if localHTTP {
			if err := migrate.Run(context.Background()); err != nil {
				log.Error("migrate failed", "error", err)
				os.Exit(1)
			}
			return
		}
		lambda.Start(func(ctx context.Context) error {
			return migrate.Run(ctx)
		})
	case "seed":
		if localHTTP {
			if err := seed.Run(context.Background()); err != nil {
				log.Error("seed failed", "error", err)
				os.Exit(1)
			}
			return
		}
		lambda.Start(func(ctx context.Context) error {
			return seed.Run(ctx)
		})
	default:
		db, err := initPool(context.Background(), log, localHTTP)
		if err != nil {
			log.Error("db pool init failed", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		yt := initYoutubeClient(context.Background(), log, localHTTP)
		r := handler.NewRouter(log, db, yt)
		if localHTTP {
			log.Info("starting HTTP server", "addr", ":8080")
			if err := http.ListenAndServe(":8080", r); err != nil {
				log.Error("server failed", "error", err)
				os.Exit(1)
			}
			return
		}
		adapter := chiadapter.NewV2(r)
		lambda.Start(adapter.ProxyWithContextV2)
	}
}

func initYoutubeClient(ctx context.Context, log *slog.Logger, local bool) *youtube.Client {
	var apiKey string
	if local {
		apiKey = os.Getenv("YOUTUBE_API_KEY")
	} else {
		cfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			log.Warn("youtube: load aws config failed", "error", err)
			return nil
		}
		sm := secretsmanager.NewFromConfig(cfg)
		key, err := config.GetStringSecret(ctx, sm, os.Getenv("YOUTUBE_API_KEY_SECRET_ARN"))
		if err != nil {
			log.Warn("youtube: api key unavailable, enrichment disabled", "error", err)
			return nil
		}
		apiKey = key
	}
	if apiKey == "" {
		log.Warn("youtube: YOUTUBE_API_KEY not set, enrichment disabled")
		return nil
	}
	return youtube.New(apiKey)
}

func initPool(ctx context.Context, log *slog.Logger, local bool) (*pgxpool.Pool, error) {
	var dsn string
	if local {
		creds, err := config.GetLocalCredentials()
		if err != nil {
			return nil, err
		}
		dsn = creds.LocalDSN()
	} else {
		cfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, err
		}
		sm := secretsmanager.NewFromConfig(cfg)
		creds, err := config.GetSecret(ctx, sm, os.Getenv("DB_SECRET_ARN"))
		if err != nil {
			return nil, err
		}
		dsn = creds.DSN("")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	log.Info("db pool ready")
	return pool, nil
}
