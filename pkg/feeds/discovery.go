package feeds

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
)

var ErrNoFeedFound = fmt.Errorf("no feed found")

func (f *FeedWatcher) DiscoverFeed(targetUrl string) (*gofeed.Feed, string, error) {
	parsedUrl, err := url.Parse(targetUrl)
	if err != nil {
		return nil, "", fmt.Errorf("invalid URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, targetUrl, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch provided URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK && strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		f.logger.Debug("Provided URL is HTML, checking for feed links")
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("failed to parse HTML: %w", err)
		}

		for _, query := range []string{"link[type='application/rss+xml']", "link[type='application/atom+xml']"} {
			results := doc.Find(query).First()
			if results.Length() == 0 {
				continue
			}

			href, exists := results.Attr("href")
			if !exists {
				continue
			}

			parsedHref, err := url.Parse(href)
			if err != nil {
				f.logger.Debug("Failed to parse feed URL from tag", "url", href, "error", err)
				continue
			}

			href = parsedUrl.ResolveReference(parsedHref).String()

			feed, err := f.parser.ParseURL(href)
			if err != nil {
				f.logger.Debug("Failed to parse feed from tag URL", "url", href, "error", err)
				continue
			}

			return feed, href, nil
		}
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
		f.logger.Debug("Checking for feed at URL", "url", targetUrl.String())

		feed, err := f.parser.ParseURL(targetUrl.String())
		if err == nil {
			return feed, targetUrl.String(), nil
		}
	}

	// If there still hasn't been a feed found, check all above routes with a trailing slash
	for _, route := range targetRoutes {
		targetUrl := parsedUrl.JoinPath(route + "/")
		f.logger.Debug("Checking for feed at URL", "url", targetUrl.String())

		feed, err := f.parser.ParseURL(targetUrl.String())
		if err == nil {
			return feed, targetUrl.String(), nil
		}
	}

	return nil, "", ErrNoFeedFound
}
