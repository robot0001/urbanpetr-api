package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type DBCredentials struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DBName   string `json:"dbname"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// DSN returns a postgres:// DSN, overriding the stored dbname if dbname is non-empty.
func (c *DBCredentials) DSN(dbname string) string {
	return c.dsn("postgres", dbname)
}

// MigrateDSN returns a pgx5:// DSN for use with golang-migrate.
func (c *DBCredentials) MigrateDSN(dbname string) string {
	return c.dsn("pgx5", dbname)
}

func (c *DBCredentials) dsn(scheme, dbname string) string {
	db := c.DBName
	if dbname != "" {
		db = dbname
	}
	u := url.URL{
		Scheme: scheme,
		User:   url.UserPassword(c.Username, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   db,
	}
	return u.String()
}

func GetSecret(ctx context.Context, sm *secretsmanager.Client, arn string) (*DBCredentials, error) {
	out, err := sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(arn),
	})
	if err != nil {
		return nil, fmt.Errorf("get secret %s: %w", arn, err)
	}
	var creds DBCredentials
	if err := json.Unmarshal([]byte(aws.ToString(out.SecretString)), &creds); err != nil {
		return nil, fmt.Errorf("parse secret %s: %w", arn, err)
	}
	return &creds, nil
}
