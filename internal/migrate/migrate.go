package migrate

import (
	"context"
	"fmt"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/robot0001/urbanpetr-api/internal/config"
)

// Run provisions DB users then applies pending SQL migrations using migrator credentials.
// When USE_ENV_CREDENTIALS=true (local Docker), it skips provisioning and reads credentials
// from env vars directly.
func Run(ctx context.Context) error {
	if os.Getenv("USE_ENV_CREDENTIALS") == "true" {
		return runLocal()
	}
	return runAWS(ctx)
}

func runLocal() error {
	creds, err := config.GetLocalCredentials()
	if err != nil {
		return fmt.Errorf("local credentials: %w", err)
	}
	m, err := migrate.New("file://migrations", creds.LocalMigrateDSN())
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func runAWS(ctx context.Context) error {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}
	sm := secretsmanager.NewFromConfig(cfg)

	master, err := config.GetSecret(ctx, sm, os.Getenv("DB_MASTER_SECRET_ARN"))
	if err != nil {
		return fmt.Errorf("get master secret: %w", err)
	}

	if err := provision(ctx, sm, master); err != nil {
		return fmt.Errorf("provision: %w", err)
	}

	migrator, err := config.GetSecret(ctx, sm, os.Getenv("DB_MIGRATOR_SECRET_ARN"))
	if err != nil {
		return fmt.Errorf("get migrator secret: %w", err)
	}

	m, err := migrate.New("file://migrations", migrator.MigrateDSN(appDB))
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
