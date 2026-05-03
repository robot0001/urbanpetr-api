package main

import (
	"context"
	"log"
	"os"

	"github.com/robot0001/urbanpetr-api/internal/migrate"
)

func main() {
	mode := os.Getenv("LAMBDA_HANDLER_MODE")
	switch mode {
	case "migrate":
		if err := migrate.Run(context.Background()); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
	default:
		// HTTP/Lambda mode — implemented in step 5
		log.Fatal("LAMBDA_HANDLER_MODE not set to a recognised value")
	}
}
