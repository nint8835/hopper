package cmd

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/mmcdole/gofeed"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Command for misc. testing stuff.",
	Run: func(cmd *cobra.Command, args []string) {
		feedUrl := args[0]

		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(feedUrl)
		checkErr(err, "Failed to parse feed")

		spew.Dump(feed)
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
