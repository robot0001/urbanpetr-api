package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robot0001/urbanpetr-api/internal/config"
	"github.com/robot0001/urbanpetr-api/internal/ingest"
)

func initPool(ctx context.Context) (*pgxpool.Pool, error) {
	creds, err := config.GetLocalCredentials()
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, creds.LocalDSN())
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func main() {
	days := flag.Int("days", 0, "only import last N days (0 = all)")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: ingest [--days=N] <path/to/watch-history.json>")
		os.Exit(1)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	data, err := os.ReadFile(args[0])
	if err != nil {
		log.Error("read file", "error", err)
		os.Exit(1)
	}

	var cutoff time.Time
	if *days > 0 {
		cutoff = time.Now().UTC().AddDate(0, 0, -*days)
	}

	videos, err := ingest.ParseEntries(data, cutoff)
	if err != nil {
		log.Error("parse entries", "error", err)
		os.Exit(1)
	}
	log.Info("parsed entries", "count", len(videos))

	db, err := initPool(ctx)
	if err != nil {
		log.Error("db pool", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	total, created, err := ingest.Ingest(ctx, db, videos)
	if err != nil {
		log.Error("ingest", "error", err)
		os.Exit(1)
	}
	log.Info("ingest complete", "created", created, "skipped_duplicates", total-created, "total", total)
}
