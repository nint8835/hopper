package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/golang-migrate/migrate/v4"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"

	"github.com/nint8835/hopper/pkg/config"
	"github.com/nint8835/hopper/pkg/database/migrations"
	"github.com/nint8835/hopper/pkg/feeds"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh all feeds once and exit. Useful for testing or for impatient operators.",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		checkErr(err, "Failed to load config")

		db, err := sql.Open("sqlite", cfg.DatabasePath+"?_pragma=foreign_keys(1)")
		checkErr(err, "Failed to connect to database")
		defer db.Close()

		migrationRunner, err := migrations.New(db)
		checkErr(err, "Failed to create migration runner")
		err = migrationRunner.Up()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			checkErr(err, "Failed to run migrations")
		}

		session, err := discordgo.New(fmt.Sprintf("Bot %s", cfg.DiscordToken))
		checkErr(err, "Failed to create Discord session")

		err = session.Open()
		checkErr(err, "Failed to open Discord connection")
		defer func() {
			if err := session.Close(); err != nil {
				slog.Error("Failed to close Discord connection", "err", err)
			}
		}()

		// Construct the watcher but don't Start() it — we just want a one-shot
		// refresh, not the scheduled poll loop. Stop() is unsafe to call when
		// Start() wasn't, so we let the process exit clean up the ticker and
		// internal context.
		watcher := feeds.New(cfg, db, session)

		err = watcher.RefreshFeeds(context.Background())
		if err != nil {
			slog.Error("Refresh completed with errors", "err", err)
			return
		}

		slog.Info("Refresh complete")
	},
}

func init() {
	rootCmd.AddCommand(refreshCmd)
}
