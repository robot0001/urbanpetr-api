package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/robot0001/urbanpetr-api/internal/handler"
	"github.com/robot0001/urbanpetr-api/internal/migrate"
	"github.com/robot0001/urbanpetr-api/internal/seed"
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
		r := handler.NewRouter(log)
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
