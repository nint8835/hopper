package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "hopper",
	Short: "RSS feed reader bot for Discord.",
}

func Execute() {
	err := rootCmd.Execute()
	checkErr(err, "Failed to execute")
}
