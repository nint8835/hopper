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
	"sync"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
	"tailscale.com/util/truncate"

	"github.com/nint8835/hopper/pkg/config"
	"github.com/nint8835/hopper/pkg/database"
)

type FeedWatcher struct {
	Session *discordgo.Session
	Queries *database.Queries

	cfg    *config.Config
	parser *gofeed.Parser
	logger *slog.Logger

	watcherTicker *time.Ticker
	watcherCtx    context.Context
	stopWatcher   context.CancelFunc
	stoppedChan   chan struct{}

	// refreshMu serializes refresh passes so the scheduled ticker and manual
	// refresh invocations don't race on the (feed_id, post_guid) primary key.
	refreshMu sync.Mutex
}

func (f *FeedWatcher) postItem(ctx context.Context, feed database.Feed, item *gofeed.Item) (string, error) {
	f.logger.Debug("Posting item", "feed_id", feed.ID, "item_guid", item.GUID)

	channelID := f.cfg.DiscordChannelId
	if feed.ChannelID.Valid {
		channelID = feed.ChannelID.String
	}

	truncatedTitleForEmbed := truncate.String(item.Title, 253)
	if truncatedTitleForEmbed != item.Title {
		truncatedTitleForEmbed += "..."
	}

	truncatedTitleForThread := truncate.String(item.Title, 97)
	if truncatedTitleForThread != item.Title {
		truncatedTitleForThread += "..."
	}

	markdownDescription, err := htmltomarkdown.ConvertString(item.Description)
	if err != nil {
		return "", fmt.Errorf("failed to convert description to markdown: %w", err)
	}

	truncatedDescription := truncate.String(markdownDescription, 253)
	if truncatedDescription != markdownDescription {
		truncatedDescription += "..."
	}

	embed := &discordgo.MessageEmbed{
		Title:       truncatedTitleForEmbed,
		URL:         item.Link,
		Description: truncatedDescription,
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

	postMsg, err := f.Session.ChannelMessageSendEmbed(
		channelID,
		embed,
		discordgo.WithContext(ctx),
	)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	_, err = f.Session.MessageThreadStart(
		channelID,
		postMsg.ID,
		truncatedTitleForThread,
		4320,
		discordgo.WithContext(ctx),
	)
	if err != nil {
		return "", fmt.Errorf("failed to start thread: %w", err)
	}
	return postMsg.ID, nil
}

func (f *FeedWatcher) RefreshFeed(ctx context.Context, feed database.Feed, isBackfill bool) error {
	f.refreshMu.Lock()
	defer f.refreshMu.Unlock()

	f.logger.Debug("Refreshing feed", "feed_id", feed.ID, "feed_url", feed.FeedUrl)

	seenPosts, err := f.Queries.GetPosts(ctx, feed.ID)
	if err != nil {
		return fmt.Errorf("failed to get seen posts for feed %d: %w", feed.ID, err)
	}
	seenPostsMap := make(map[string]struct{}, len(seenPosts))
	for _, post := range seenPosts {
		seenPostsMap[post] = struct{}{}
	}

	feedData, err := f.parser.ParseURLWithContext(feed.FeedUrl, ctx)
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

	isPaused := feed.PausedUntil.Valid && feed.PausedUntil.Time.After(time.Now())

	for _, item := range feedData.Items {
		if _, seen := seenPostsMap[item.GUID]; seen {
			continue
		}

		f.logger.Debug("New item found", "feed_id", feed.ID, "item_guid", item.GUID)

		var postMsgId string
		shouldPost := (f.cfg.ShowBackfill || !isBackfill) && !isPaused
		if shouldPost {
			postMsgId, err = f.postItem(ctx, feed, item)
			if err != nil {
				return fmt.Errorf("failed to post item: %w", err)
			}
		}

		err = f.Queries.CreatePost(ctx, database.CreatePostParams{
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

func (f *FeedWatcher) RefreshFeeds(ctx context.Context) error {
	feeds, err := f.Queries.GetFeeds(ctx)
	if err != nil {
		return fmt.Errorf("failed to get feeds: %w", err)
	}

	var feedErrors []error

	for _, feed := range feeds {
		err := f.RefreshFeed(ctx, feed, false)
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

	err := f.RefreshFeeds(f.watcherCtx)
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

		watcherTicker: time.NewTicker(cfg.PollInterval),
		watcherCtx:    watcherCtx,
		stopWatcher:   cancel,
		stoppedChan:   make(chan struct{}),
	}
}
