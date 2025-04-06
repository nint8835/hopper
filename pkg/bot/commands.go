package bot

import (
	"cmp"
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/davecgh/go-spew/spew"
	"golang.org/x/exp/slog"
	"pkg.nit.so/switchboard"

	"github.com/nint8835/hopper/pkg/database"
	"github.com/nint8835/hopper/pkg/utils"
)

type addCommandsArgs struct {
	URL string `description:"URL to add. Should be either a link to a feed or a site under which to search for feeds."`
}

func (b *Bot) handleAddCommand(session *discordgo.Session, i *discordgo.InteractionCreate, args addCommandsArgs) {
	session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{},
	})

	feed, feedUrl, err := b.watcher.DiscoverFeed(args.URL)
	if err != nil {
		// TODO: Better response for when there is no feed
		_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: utils.PtrTo("Failed to discover feed: " + err.Error()),
		})
		return
	}
	feed.Items = nil

	siteLink := feed.Link
	if siteLink == "" && len(feed.Links) > 0 {
		siteLink = feed.Links[0]
	}

	newFeed, err := b.Queries.CreateFeed(
		context.Background(),
		database.CreateFeedParams{
			Title:       feed.Title,
			Description: feed.Description,
			Url:         siteLink,
			FeedUrl:     cmp.Or(feed.FeedLink, feedUrl),
		},
	)

	// TODO: Respond with an embed
	_, err = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: utils.PtrTo(fmt.Sprintf("```\n%s\n```", spew.Sdump(newFeed))),
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
