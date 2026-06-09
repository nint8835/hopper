package bot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"pkg.nit.so/switchboard"

	"github.com/nint8835/hopper/pkg/database"
	"github.com/nint8835/hopper/pkg/feeds"
	"github.com/nint8835/hopper/pkg/utils"
)

type addCommandsArgs struct {
	URL     string             `description:"URL to add. Should be either a link to a feed or a site under which to search for feeds."`
	Channel *discordgo.Channel `description:"Channel to post feed updates to. If not specified, uses the default channel."`
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

	var channelID sql.NullString
	displayChannelID := b.config.DiscordChannelId

	if args.Channel != nil {
		channelID = sql.NullString{String: args.Channel.ID, Valid: true}
		displayChannelID = args.Channel.ID
	}

	newFeed, err := b.Queries.CreateFeed(
		context.Background(),
		database.CreateFeedParams{
			Title:       feed.Title,
			Description: feed.Description,
			Url:         siteLink,
			FeedUrl:     feedUrl,
			ChannelID:   channelID,
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
					{
						Name:  "Posting in",
						Value: fmt.Sprintf("<#%s>", displayChannelID),
					},
				},
			},
		}),
	})
	if err != nil {
		b.logger.Error("Failed to respond to interaction", "error", err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := b.watcher.RefreshFeed(ctx, newFeed, true); err != nil {
			b.logger.Error("Failed to backfill new feed", "feed_id", newFeed.ID, "error", err)
		}
	}()
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
		pausedSuffix := ""
		if feed.PausedUntil.Valid && feed.PausedUntil.Time.After(time.Now()) {
			if feed.PausedUntil.Time.Year() >= 9999 {
				pausedSuffix = " (paused)"
			} else {
				pausedSuffix = fmt.Sprintf(" (paused until <t:%d:f>)", feed.PausedUntil.Time.Unix())
			}
		}
		feedStrings = append(feedStrings, fmt.Sprintf("- `%d`. **%s** (`%s`)%s", feed.ID, feed.Title, feed.FeedUrl, pausedSuffix))
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

type removeCommandArgs struct {
	ID int `description:"ID of the feed to remove."`
}

func (b *Bot) handleRemoveCommand(session *discordgo.Session, i *discordgo.InteractionCreate, args removeCommandArgs) {
	err := b.Queries.DeleteFeed(context.Background(), int64(args.ID))
	if err != nil {
		b.logger.Error("Failed to delete feed", "error", err)
		_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
				{
					Title:       "Failed to delete feed",
					Description: fmt.Sprintf("```\n%s\n```", err.Error()),
					Color:       0xff0000,
				},
			}),
		})
		return
	}

	err = session.InteractionRespond(
		i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Feed with ID `%d` removed.", args.ID),
			},
		},
	)
	if err != nil {
		b.logger.Error("Failed to respond to interaction", "error", err)
	}
}

type pauseCommandArgs struct {
	ID       int     `description:"ID of the feed to pause."`
	Duration *string `description:"Optional duration to pause for (e.g. 1h, 30m, 24h). If not specified, pauses indefinitely."`
}

// indefinitePauseTime is used to represent an indefinite pause. Far enough in the future
// to never be reached, but still a valid sql.NullTime.
var indefinitePauseTime = time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)

func (b *Bot) handlePauseCommand(session *discordgo.Session, i *discordgo.InteractionCreate, args pauseCommandArgs) {
	feed, err := b.Queries.GetFeedByID(context.Background(), int64(args.ID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_ = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("No feed found with ID `%d`.", args.ID),
				},
			})
			return
		}

		b.logger.Error("Failed to get feed", "error", err)
		_ = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Failed to get feed",
						Description: fmt.Sprintf("```\n%s\n```", err.Error()),
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	pausedUntil := indefinitePauseTime
	durationDisplay := "indefinitely"

	if args.Duration != nil && *args.Duration != "" {
		duration, err := time.ParseDuration(*args.Duration)
		if err != nil {
			_ = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Invalid duration `%s`. Use a Go duration string like `1h`, `30m`, or `24h`.", *args.Duration),
				},
			})
			return
		}
		if duration <= 0 {
			_ = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Duration must be positive.",
				},
			})
			return
		}
		pausedUntil = time.Now().Add(duration)
		durationDisplay = fmt.Sprintf("until <t:%d:f>", pausedUntil.Unix())
	}

	err = b.Queries.PauseFeed(context.Background(), database.PauseFeedParams{
		PausedUntil: sql.NullTime{Time: pausedUntil, Valid: true},
		ID:          feed.ID,
	})
	if err != nil {
		b.logger.Error("Failed to pause feed", "error", err)
		_ = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Failed to pause feed",
						Description: fmt.Sprintf("```\n%s\n```", err.Error()),
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	err = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Paused **%s** %s. New items will be silently consumed until resumed.", feed.Title, durationDisplay),
		},
	})
	if err != nil {
		b.logger.Error("Failed to respond to interaction", "error", err)
	}
}

type unpauseCommandArgs struct {
	ID int `description:"ID of the feed to unpause."`
}

func (b *Bot) handleUnpauseCommand(session *discordgo.Session, i *discordgo.InteractionCreate, args unpauseCommandArgs) {
	feed, err := b.Queries.GetFeedByID(context.Background(), int64(args.ID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_ = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("No feed found with ID `%d`.", args.ID),
				},
			})
			return
		}

		b.logger.Error("Failed to get feed", "error", err)
		_ = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Failed to get feed",
						Description: fmt.Sprintf("```\n%s\n```", err.Error()),
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	err = b.Queries.UnpauseFeed(context.Background(), feed.ID)
	if err != nil {
		b.logger.Error("Failed to unpause feed", "error", err)
		_ = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Failed to unpause feed",
						Description: fmt.Sprintf("```\n%s\n```", err.Error()),
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	err = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Resumed **%s**.", feed.Title),
		},
	})
	if err != nil {
		b.logger.Error("Failed to respond to interaction", "error", err)
	}
}

type refreshCommandArgs struct {
	ID *int `description:"Optional ID of the feed to refresh. If omitted, refreshes all feeds."`
}

func (b *Bot) handleRefreshCommand(session *discordgo.Session, i *discordgo.InteractionCreate, args refreshCommandArgs) {
	if err := session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{},
	}); err != nil {
		b.logger.Error("Failed to defer interaction response", "error", err)
		return
	}

	// Discord interaction tokens expire after 15 minutes; cap the refresh so we
	// always have time to edit the response with the outcome.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if args.ID != nil {
		feed, err := b.Queries.GetFeedByID(ctx, int64(*args.ID))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: utils.PtrTo(fmt.Sprintf("No feed found with ID `%d`.", *args.ID)),
				})
				return
			}

			b.logger.Error("Failed to get feed", "error", err)
			_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
					{
						Title:       "Failed to get feed",
						Description: fmt.Sprintf("```\n%s\n```", err.Error()),
						Color:       0xff0000,
					},
				}),
			})
			return
		}

		err = b.watcher.RefreshFeed(ctx, feed, false)
		if err != nil {
			b.logger.Error("Failed to refresh feed", "feed_id", feed.ID, "error", err)
			_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
					{
						Title:       "Failed to refresh feed",
						Description: fmt.Sprintf("```\n%s\n```", err.Error()),
						Color:       0xff0000,
					},
				}),
			})
			return
		}

		_, err = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: utils.PtrTo(fmt.Sprintf("Refreshed **%s**.", feed.Title)),
		})
		if err != nil {
			b.logger.Error("Failed to respond to interaction", "error", err)
		}
		return
	}

	err := b.watcher.RefreshFeeds(ctx)
	if err != nil {
		b.logger.Error("Failed to refresh feeds", "error", err)
		_, _ = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: utils.PtrTo([]*discordgo.MessageEmbed{
				{
					Title:       "Failed to refresh one or more feeds",
					Description: fmt.Sprintf("```\n%s\n```", err.Error()),
					Color:       0xff0000,
				},
			}),
		})
		return
	}

	_, err = session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: utils.PtrTo("Refreshed all feeds."),
	})
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
	_ = b.parser.AddCommand(&switchboard.Command{
		Name:        "remove",
		Description: "Remove a feed",
		Handler:     b.handleRemoveCommand,
		GuildID:     b.config.DiscordGuildId,
	})
	_ = b.parser.AddCommand(&switchboard.Command{
		Name:        "pause",
		Description: "Pause a feed from posting new items",
		Handler:     b.handlePauseCommand,
		GuildID:     b.config.DiscordGuildId,
	})
	_ = b.parser.AddCommand(&switchboard.Command{
		Name:        "unpause",
		Description: "Resume a paused feed",
		Handler:     b.handleUnpauseCommand,
		GuildID:     b.config.DiscordGuildId,
	})
	_ = b.parser.AddCommand(&switchboard.Command{
		Name:        "refresh",
		Description: "Manually trigger a feed refresh (useful for testing)",
		Handler:     b.handleRefreshCommand,
		GuildID:     b.config.DiscordGuildId,
	})
}
