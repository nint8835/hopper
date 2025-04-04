package cmd

import (
	"log/slog"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/nint8835/hopper/pkg/bot"
	"github.com/nint8835/hopper/pkg/config"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the bot.",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		checkErr(err, "Failed to load config")

		botInst, err := bot.New(cfg)
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
