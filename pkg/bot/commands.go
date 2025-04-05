package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/davecgh/go-spew/spew"
	"golang.org/x/exp/slog"
	"pkg.nit.so/switchboard"
)

type addCommandsArgs struct {
	URL string `description:"URL to add. Should be either a link to a feed or a site under which to search for feeds."`
}

func (b *Bot) handleAddCommand(session *discordgo.Session, i *discordgo.InteractionCreate, args addCommandsArgs) {
	session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{},
	})

	feed, err := b.watcher.DiscoverFeed(args.URL)
	if err != nil {
		respText := "Failed to discover feed: " + err.Error()
		_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &respText,
		})
		return
	}
	feed.Items = nil

	respText := "```\n" + spew.Sdump(feed) + "\n```"

	_, err = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &respText,
	})
	if err != nil {
		slog.Error("Failed to respond to interaction", "error", err)
	}
}

func (b *Bot) registerCommands() {
	_ = b.parser.AddCommand(&switchboard.Command{
		Name:        "add",
		Description: "Add a new feed",
		Handler:     b.handleAddCommand,
		GuildID:     b.config.DiscordGuildId,
	})
}
