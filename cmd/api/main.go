package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/robot0001/urbanpetr-api/internal/handler"
	"github.com/robot0001/urbanpetr-api/internal/migrate"
	"github.com/robot0001/urbanpetr-api/internal/seed"
)

func main() {
	mode := os.Getenv("LAMBDA_HANDLER_MODE")
	switch mode {
	case "migrate":
		if err := migrate.Run(context.Background()); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
	case "seed":
		if err := seed.Run(context.Background()); err != nil {
			log.Fatalf("seed failed: %v", err)
		}
	default:
		r := handler.NewRouter()
		adapter := chiadapter.New(r)
		lambda.Start(adapter.ProxyWithContext)
	}
}
