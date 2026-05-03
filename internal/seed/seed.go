package seed

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/jackc/pgx/v5"
	"github.com/robot0001/urbanpetr-api/internal/config"
)

//go:embed sql/initial_data.sql
var seedSQL string

// Run connects to the app database and executes the embedded seed SQL.
// Credentials are read from DB_SECRET_ARN; the target database is encoded
// in the secret (set by provision.go, respecting DB_NAME at provision time).
func Run(ctx context.Context) error {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}
	sm := secretsmanager.NewFromConfig(cfg)

	creds, err := config.GetSecret(ctx, sm, os.Getenv("DB_SECRET_ARN"))
	if err != nil {
		return fmt.Errorf("get db secret: %w", err)
	}

	conn, err := pgx.Connect(ctx, creds.DSN(""))
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, seedSQL); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	return nil
}
