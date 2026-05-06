package main

import (
	"context"
	"log/slog"
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

	mode := os.Getenv("LAMBDA_HANDLER_MODE")
	switch mode {
	case "migrate":
		lambda.Start(func(ctx context.Context) error {
			return migrate.Run(ctx)
		})
	case "seed":
		lambda.Start(func(ctx context.Context) error {
			return seed.Run(ctx)
		})
	default:
		r := handler.NewRouter(log)
		adapter := chiadapter.NewV2(r)
		lambda.Start(adapter.ProxyWithContextV2)
	}
}
