package cmd

import (
	"log/slog"
	"os"
)

func checkErr(err error, msg string) {
	if err != nil {
		slog.Error(msg, "err", err)
		os.Exit(1)
	}
}
