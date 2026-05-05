package migrate

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/jackc/pgx/v5"
	"github.com/robot0001/urbanpetr-api/internal/config"
)

const (
	userMigrator = "urbanpetr_api_migrator"
	userApp      = "urbanpetr_api_app"
	userReadonly = "readonly"
)

// appDB is the target database name. Overridable via DB_NAME for staging PR envs.
var appDB = envOrDefault("DB_NAME", "urbanpetr_api")

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// provision bootstraps the app database, users, and privileges using master credentials.
// Safe to call on every deploy — all operations are idempotent.
func provision(ctx context.Context, sm *secretsmanager.Client, master *config.DBCredentials) error {
	clusterConn, err := pgx.Connect(ctx, master.DSN("postgres"))
	if err != nil {
		return fmt.Errorf("connect to cluster: %w", err)
	}
	defer clusterConn.Close(ctx)

	if err := createDatabaseIfNotExists(ctx, clusterConn, appDB); err != nil {
		return fmt.Errorf("create database: %w", err)
	}

	users := []struct {
		arnEnv   string
		username string
		dbname   string
	}{
		{"DB_READONLY_SECRET_ARN", userReadonly, appDB},
		{"DB_MIGRATOR_SECRET_ARN", userMigrator, appDB},
		{"DB_SECRET_ARN", userApp, appDB},
	}

	passwords := make(map[string]string, len(users))
	for _, u := range users {
		password, err := ensureUserSecret(ctx, sm, os.Getenv(u.arnEnv), u.username, u.dbname, master)
		if err != nil {
			return fmt.Errorf("ensure secret for %s: %w", u.username, err)
		}
		passwords[u.username] = password
		if err := ensureUser(ctx, clusterConn, u.username, password); err != nil {
			return fmt.Errorf("ensure pg user %s: %w", u.username, err)
		}
		if err := grantConnect(ctx, clusterConn, appDB, u.username); err != nil {
			return fmt.Errorf("grant connect for %s: %w", u.username, err)
		}
	}

	appConn, err := pgx.Connect(ctx, master.DSN(appDB))
	if err != nil {
		return fmt.Errorf("connect to app db: %w", err)
	}
	defer appConn.Close(ctx)

	if err := applyExplicitGrants(ctx, appConn); err != nil {
		return err
	}

	// ALTER DEFAULT PRIVILEGES must run as the migrator user: RDS master is not
	// a superuser and cannot alter default privileges for another role.
	migratorCreds := config.DBCredentials{
		Host: master.Host, Port: master.Port,
		DBName: appDB, Username: userMigrator, Password: passwords[userMigrator],
	}
	migratorConn, err := pgx.Connect(ctx, migratorCreds.DSN(""))
	if err != nil {
		return fmt.Errorf("connect as migrator: %w", err)
	}
	defer migratorConn.Close(ctx)

	return applyDefaultPrivileges(ctx, migratorConn)
}

func createDatabaseIfNotExists(ctx context.Context, conn *pgx.Conn, dbname string) error {
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbname).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{dbname}.Sanitize()))
	return err
}

// ensureUserSecret reads the password from Secrets Manager if it already has a value,
// otherwise generates a new one and stores it. Returns the password.
func ensureUserSecret(ctx context.Context, sm *secretsmanager.Client, arn, username, dbname string, master *config.DBCredentials) (string, error) {
	out, err := sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(arn)})
	if err == nil && out.SecretString != nil {
		var existing config.DBCredentials
		if jsonErr := json.Unmarshal([]byte(*out.SecretString), &existing); jsonErr == nil && existing.Password != "" {
			return existing.Password, nil
		}
	}

	var rnf *types.ResourceNotFoundException
	if err != nil && !errors.As(err, &rnf) {
		return "", fmt.Errorf("get secret %s: %w", arn, err)
	}

	password, err := generatePassword()
	if err != nil {
		return "", err
	}

	value, _ := json.Marshal(config.DBCredentials{
		Host:     master.Host,
		Port:     master.Port,
		DBName:   dbname,
		Username: username,
		Password: password,
	})
	if _, err := sm.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(arn),
		SecretString: aws.String(string(value)),
	}); err != nil {
		return "", fmt.Errorf("store secret %s: %w", arn, err)
	}

	return password, nil
}

// ensureUser creates the PostgreSQL login user if it does not exist, or updates
// the password if it does. Base64url passwords are safe to interpolate here
// (A-Z a-z 0-9 - _ =); no SQL-special characters are produced.
func ensureUser(ctx context.Context, conn *pgx.Conn, username, password string) error {
	q := pgx.Identifier{username}.Sanitize()
	sql := fmt.Sprintf(`DO $$ BEGIN
  CREATE USER %s WITH PASSWORD '%s' CONNECTION LIMIT -1;
EXCEPTION WHEN duplicate_object THEN
  ALTER USER %s WITH PASSWORD '%s';
END $$;`, q, password, q, password)
	_, err := conn.Exec(ctx, sql)
	return err
}

func grantConnect(ctx context.Context, conn *pgx.Conn, dbname, username string) error {
	sql := fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s",
		pgx.Identifier{dbname}.Sanitize(),
		pgx.Identifier{username}.Sanitize())
	_, err := conn.Exec(ctx, sql)
	return err
}

// applyExplicitGrants runs as master: grants schema usage and privileges on
// already-existing objects to app/readonly/migrator users.
func applyExplicitGrants(ctx context.Context, conn *pgx.Conn) error {
	m := pgx.Identifier{userMigrator}.Sanitize()
	a := pgx.Identifier{userApp}.Sanitize()
	r := pgx.Identifier{userReadonly}.Sanitize()

	stmts := []string{
		fmt.Sprintf("GRANT CREATE, USAGE ON SCHEMA public TO %s", m),
		fmt.Sprintf("GRANT ALL ON ALL TABLES IN SCHEMA public TO %s", m),
		fmt.Sprintf("GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO %s", m),
		fmt.Sprintf("GRANT ALL ON ALL FUNCTIONS IN SCHEMA public TO %s", m),

		fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s", a),
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %s", a),
		fmt.Sprintf("GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO %s", a),
		fmt.Sprintf("GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO %s", a),

		fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s", r),
		fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s", r),
		fmt.Sprintf("GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO %s", r),
	}

	for _, stmt := range stmts {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("grant failed (%s): %w", stmt, err)
		}
	}
	return nil
}

// applyDefaultPrivileges runs as the migrator user so that objects it creates
// in future migrations are automatically accessible to app/readonly users.
// FOR ROLE is omitted — it defaults to the current user (migrator).
func applyDefaultPrivileges(ctx context.Context, conn *pgx.Conn) error {
	a := pgx.Identifier{userApp}.Sanitize()
	r := pgx.Identifier{userReadonly}.Sanitize()

	stmts := []string{
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s", a),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE ON SEQUENCES TO %s", a),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT EXECUTE ON FUNCTIONS TO %s", a),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO %s", r),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON SEQUENCES TO %s", r),
	}

	for _, stmt := range stmts {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("grant failed (%s): %w", stmt, err)
		}
	}
	return nil
}

func generatePassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
