package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/config"
	"github.com/khfcre-afk/pterodactyl-billing/backend/migrations"
)

func main() {
	flag.Parse()
	cmd := flag.Arg(0)
	if cmd == "" {
		cmd = "up"
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}

	connConfig, err := cfg.Postgres.PgxConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("pgx config")
	}
	db := stdlib.OpenDB(*connConfig)
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal().Err(err).Msg("dialect")
	}

	if err := run(context.Background(), db, cmd, flag.Args()[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, db *sql.DB, cmd string, args []string) error {
	return goose.RunContext(ctx, cmd, db, ".", args...)
}
