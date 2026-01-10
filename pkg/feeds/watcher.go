package feeds

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"

	"github.com/nint8835/hopper/pkg/config"
	"github.com/nint8835/hopper/pkg/database"
	"github.com/nint8835/hopper/pkg/utils"
)

type FeedWatcher struct {
	Session *discordgo.Session
	Queries *database.Queries

	cfg    *config.Config
	parser *gofeed.Parser
	logger *slog.Logger

	channelTypeCache map[string]discordgo.ChannelType

	watcherTicker *time.Ticker
	watcherCtx    context.Context
	stopWatcher   context.CancelFunc
	stoppedChan   chan struct{}
}

func (f *FeedWatcher) getChannelType(channelID string) (discordgo.ChannelType, error) {
	if channelType, exists := f.channelTypeCache[channelID]; exists {
		return channelType, nil
	}

	channel, err := f.Session.State.Channel(channelID)
	if err != nil {
		channel, err = f.Session.Channel(channelID)
		if err != nil {
			return 0, fmt.Errorf("failed to get channel info: %w", err)
		}
	}

	f.channelTypeCache[channelID] = channel.Type
	return channel.Type, nil
}

func (f *FeedWatcher) postItem(feed database.Feed, item *gofeed.Item) (string, error) {
	f.logger.Debug("Posting item", "feed_id", feed.ID, "item_guid", item.GUID)

	channelID := f.cfg.DiscordChannelId
	if feed.ChannelID.Valid {
		channelID = feed.ChannelID.String
	}

	markdownDescription, err := htmltomarkdown.ConvertString(item.Description)
	if err != nil {
		return "", fmt.Errorf("failed to convert description to markdown: %w", err)
	}

	embed := &discordgo.MessageEmbed{
		Title:       utils.TruncateString(item.Title, 256),
		URL:         item.Link,
		Description: utils.TruncateString(markdownDescription, 256),
		Author: &discordgo.MessageEmbedAuthor{
			Name: feed.Title,
			URL:  feed.Url,
		},
	}

	if item.Image != nil {
		imageUrl := item.Image.URL
		if strings.HasPrefix(imageUrl, "/") {
			imageUrl = feed.Url + imageUrl
		}

		_, err = url.ParseRequestURI(imageUrl)

		if err != nil {
			f.logger.Warn("Invalid image URL", "feed_id", feed.ID, "item_guid", item.GUID, "image_url", imageUrl, "error", err)
		} else {
			f.logger.Debug("Using image URL", "feed_id", feed.ID, "item_guid", item.GUID, "image_url", imageUrl)
			embed.Image = &discordgo.MessageEmbedImage{
				URL: imageUrl,
			}
		}
	}

	authorNames := make([]string, 0, len(item.Authors))
	if len(item.Authors) > 0 {
		for _, author := range item.Authors {
			authorNames = append(authorNames, author.Name)
		}

		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("By %s", strings.Join(authorNames, ", ")),
		}
	}

	channelType, err := f.getChannelType(channelID)
	if err != nil {
		return "", fmt.Errorf("failed to get channel type: %w", err)
	}

	switch channelType {
	case discordgo.ChannelTypeGuildText:
		postMsg, err := f.Session.ChannelMessageSendEmbed(
			channelID,
			embed,
			discordgo.WithContext(f.watcherCtx),
		)
		if err != nil {
			return "", fmt.Errorf("failed to send message: %w", err)
		}

		_, err = f.Session.MessageThreadStart(
			channelID,
			postMsg.ID,
			utils.TruncateString(item.Title, 100),
			4320,
			discordgo.WithContext(f.watcherCtx),
		)
		if err != nil {
			return "", fmt.Errorf("failed to start thread: %w", err)
		}
		return postMsg.ID, nil
	case discordgo.ChannelTypeGuildForum:
		forumThread, err := f.Session.ForumThreadStartEmbed(
			channelID,
			utils.TruncateString(item.Title, 100),
			4320,
			embed,
			discordgo.WithContext(f.watcherCtx),
		)
		if err != nil {
			return "", fmt.Errorf("failed to start forum thread: %w", err)
		}
		return forumThread.ID, nil
	default:
		return "", fmt.Errorf("unsupported channel type: %d", channelType)
	}
}

func (f *FeedWatcher) RefreshFeed(feed database.Feed, isBackfill bool) error {
	f.logger.Debug("Refreshing feed", "feed_id", feed.ID, "feed_url", feed.FeedUrl)

	seenPosts, err := f.Queries.GetPosts(f.watcherCtx, feed.ID)
	if err != nil {
		return fmt.Errorf("failed to get seen posts for feed %d: %w", feed.ID, err)
	}
	seenPostsMap := make(map[string]struct{}, len(seenPosts))
	for _, post := range seenPosts {
		seenPostsMap[post] = struct{}{}
	}

	feedData, err := f.parser.ParseURLWithContext(feed.FeedUrl, f.watcherCtx)
	if err != nil {
		return fmt.Errorf("failed to parse feed: %w", err)
	}

	// Sort items by published date ascending, to ensure they are posted in the order they were published
	sort.Slice(feedData.Items, func(i, j int) bool {
		iTime := feedData.Items[i].PublishedParsed
		jTime := feedData.Items[j].PublishedParsed

		if iTime == nil || jTime == nil {
			return false
		}

		return iTime.Before(*jTime)
	})

	for _, item := range feedData.Items {
		if _, seen := seenPostsMap[item.GUID]; seen {
			continue
		}

		f.logger.Debug("New item found", "feed_id", feed.ID, "item_guid", item.GUID)

		var postMsgId string
		if f.cfg.ShowBackfill || !isBackfill {
			postMsgId, err = f.postItem(feed, item)
			if err != nil {
				return fmt.Errorf("failed to post item: %w", err)
			}
		}

		err = f.Queries.CreatePost(f.watcherCtx, database.CreatePostParams{
			PostGuid:    item.GUID,
			FeedID:      feed.ID,
			Title:       item.Title,
			Description: item.Description,
			Url:         item.Link,
			MessageID:   postMsgId,
		})
		if err != nil {
			return fmt.Errorf("failed to create post: %w", err)
		}
	}

	return nil
}

func (f *FeedWatcher) refreshFeeds() error {
	feeds, err := f.Queries.GetFeeds(f.watcherCtx)
	if err != nil {
		return fmt.Errorf("failed to get feeds: %w", err)
	}

	var feedErrors []error

	for _, feed := range feeds {
		err := f.RefreshFeed(feed, false)
		if err != nil {
			feedErrors = append(feedErrors, fmt.Errorf("failed to refresh feed %d: %w", feed.ID, err))
		}
	}

	if len(feedErrors) > 0 {
		return errors.Join(feedErrors...)
	}

	return nil
}

func (f *FeedWatcher) scheduledTask() {
	f.logger.Debug("Refreshing feeds")

	err := f.refreshFeeds()
	if err != nil {
		f.logger.Error("Failed to refresh feeds", "error", err)
		return
	}
}

func (f *FeedWatcher) run() {
	defer close(f.stoppedChan)

	f.scheduledTask()

	for {
		select {
		case <-f.watcherTicker.C:
			f.scheduledTask()
		case <-f.watcherCtx.Done():
			return
		}
	}
}

func (f *FeedWatcher) Start() {
	f.logger.Debug("Starting feed watcher")
	go f.run()
}

func (f *FeedWatcher) Stop() {
	f.logger.Debug("Stopping feed watcher")

	f.stopWatcher()
	f.watcherTicker.Stop()

	<-f.stoppedChan
	f.logger.Debug("Feed watcher stopped")
}

func New(cfg *config.Config, db *sql.DB, session *discordgo.Session) *FeedWatcher {
	ctx := context.Background()
	watcherCtx, cancel := context.WithCancel(ctx)

	return &FeedWatcher{
		Session: session,
		Queries: database.New(db),
		cfg:     cfg,
		parser:  gofeed.NewParser(),
		logger:  slog.Default().With("component", "feed-watcher"),

		channelTypeCache: make(map[string]discordgo.ChannelType),

		watcherTicker: time.NewTicker(cfg.PollInterval),
		watcherCtx:    watcherCtx,
		stopWatcher:   cancel,
		stoppedChan:   make(chan struct{}),
	}
}
