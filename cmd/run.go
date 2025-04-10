package cmd

import (
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"os/signal"

	"github.com/golang-migrate/migrate/v4"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"

	"github.com/nint8835/hopper/pkg/bot"
	"github.com/nint8835/hopper/pkg/config"
	"github.com/nint8835/hopper/pkg/database/migrations"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the bot.",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		checkErr(err, "Failed to load config")

		db, err := sql.Open("sqlite", cfg.DatabasePath+"?_pragma=foreign_keys(1)")
		checkErr(err, "Failed to connect to database")

		migrationRunner, err := migrations.New(db)
		checkErr(err, "Failed to create migration runner")
		err = migrationRunner.Up()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			checkErr(err, "Failed to run migrations")
		}

		botInst, err := bot.New(cfg, db)
		checkErr(err, "Failed to create bot instance")

		go func() {
			err = botInst.Run()
			if err != nil {
				slog.Error("Error running bot", "error", err)
				os.Exit(1)
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)
		<-quit
		botInst.Stop()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
