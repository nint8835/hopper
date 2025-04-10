package bot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"pkg.nit.so/switchboard"

	"github.com/nint8835/hopper/pkg/database"
	"github.com/nint8835/hopper/pkg/feeds"
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
		if errors.Is(err, feeds.ErrNoFeedFound) {
			_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
					{
						Title:       "No feed found",
						Description: "No feed found at that URL. If the URL is correct, try providing the URL directly to the site's feed.",
						Color:       0xffbc00,
					},
				}),
			})
			return
		} else {
			b.logger.Error("Failed to discover feed", "error", err)
			_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
					{
						Title:       "Failed to discover feed",
						Description: fmt.Sprintf("```\n%s\n```", err.Error()),
						Color:       0xff0000,
					},
				}),
			})
			return
		}
	}

	_, err = b.Queries.GetFeedByUrl(context.Background(), feedUrl)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		b.logger.Error("Failed to check if feed exists", "error", err)
		_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
				{
					Title:       "Failed to check if feed exists",
					Description: fmt.Sprintf("```\n%s\n```", err.Error()),
					Color:       0xff0000,
				},
			}),
		})
		return
	} else if err == nil {
		_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
				{
					Title:       "Feed already exists",
					Description: "A feed with that URL already exists.",
					Color:       0xffbc00,
				},
			}),
		})
		return
	}

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
			FeedUrl:     feedUrl,
		},
	)
	if err != nil {
		b.logger.Error("Failed to create feed", "error", err)
		_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
				{
					Title:       "Failed to create feed",
					Description: fmt.Sprintf("```\n%s\n```", err.Error()),
					Color:       0xff0000,
				},
			}),
		})
		return
	}

	_, err = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
			{
				Title: "Feed added!",
				Color: 0x44b649,
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:  "Title",
						Value: feed.Title,
					},
					{
						Name:  "Description",
						Value: feed.Description,
					},
					{
						Name:  "Site URL",
						Value: siteLink,
					},
					{
						Name:  "Feed URL",
						Value: fmt.Sprintf("`%s`", newFeed.FeedUrl),
					},
				},
			},
		}),
	})
	if err != nil {
		b.logger.Error("Failed to respond to interaction", "error", err)
	}

	go b.watcher.RefreshFeed(newFeed, true)
}

func (b *Bot) handleListCommand(session *discordgo.Session, i *discordgo.InteractionCreate, args struct{}) {
	allFeeds, err := b.Queries.GetFeeds(context.Background())
	if err != nil {
		b.logger.Error("Failed to get feeds", "error", err)
		_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
				{
					Title:       "Failed to get feeds",
					Description: fmt.Sprintf("```\n%s\n```", err.Error()),
					Color:       0xff0000,
				},
			}),
		})
		return
	}

	feedStrings := make([]string, 0, len(allFeeds))
	for _, feed := range allFeeds {
		feedStrings = append(feedStrings, fmt.Sprintf("- `%d`. **%s** (`%s`)", feed.ID, feed.Title, feed.FeedUrl))
	}

	err = session.InteractionRespond(
		i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: strings.Join(feedStrings, "\n"),
			},
		},
	)
	if err != nil {
		b.logger.Error("Failed to respond to interaction", "error", err)
	}
}

func (b *Bot) registerCommands() {
	_ = b.parser.AddCommand(&switchboard.Command{
		Name:        "add",
		Description: "Add a new feed",
		Handler:     b.handleAddCommand,
		GuildID:     b.config.DiscordGuildId,
	})
	_ = b.parser.AddCommand(&switchboard.Command{
		Name:        "list",
		Description: "List all feeds",
		Handler:     b.handleListCommand,
		GuildID:     b.config.DiscordGuildId,
	})
}
