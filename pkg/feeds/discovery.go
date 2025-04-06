package feeds

import (
	"fmt"
	"log/slog"
	"net/url"

	"github.com/mmcdole/gofeed"
)

var ErrNoFeedFound = fmt.Errorf("no feed found")

func (f *FeedWatcher) DiscoverFeed(targetUrl string) (*gofeed.Feed, string, error) {
	parsedUrl, err := url.Parse(targetUrl)
	if err != nil {
		return nil, "", fmt.Errorf("invalid URL: %w", err)
	}

	// List of routes to check based off Miniflux's source code https://github.com/miniflux/v2/blob/7514e8a0c119e2c11e49a5eb9c8e566158757ecf/internal/reader/subscription/finder.go#L191-L201
	targetRoutes := []string{
		".",
		"/atom.xml",
		"/atom",
		"/feed_rss_created.xml", // MkDocs RSS feed plugin
		"/feed.atom",
		"/feed.xml",
		"/feed",
		"/index.rss",
		"/index.xml",
		"/rss.xml",
		"/rss",
		"/rss/feed.xml",
	}

	for _, route := range targetRoutes {
		targetUrl := parsedUrl.JoinPath(route)
		slog.Debug("Checking URL", "url", targetUrl.String())

		feed, err := f.parser.ParseURL(targetUrl.String())
		if err == nil {
			return feed, targetUrl.String(), nil
		}
	}

	// If there still hasn't been a feed found, check all above routes with a trailing slash
	for _, route := range targetRoutes {
		targetUrl := parsedUrl.JoinPath(route + "/")
		slog.Debug("Checking URL", "url", targetUrl.String())

		feed, err := f.parser.ParseURL(targetUrl.String())
		if err == nil {
			return feed, targetUrl.String(), nil
		}
	}

	return nil, "", ErrNoFeedFound
}
