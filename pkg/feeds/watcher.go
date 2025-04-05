package feeds

import (
	"database/sql"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"

	"github.com/nint8835/hopper/pkg/config"
	"github.com/nint8835/hopper/pkg/database"
)

type FeedWatcher struct {
	Session *discordgo.Session
	Queries *database.Queries

	cfg    *config.Config
	parser *gofeed.Parser
}

func New(cfg *config.Config, db *sql.DB, session *discordgo.Session) *FeedWatcher {
	return &FeedWatcher{
		Session: session,
		Queries: database.New(db),
		cfg:     cfg,
		parser:  gofeed.NewParser(),
	}
}
