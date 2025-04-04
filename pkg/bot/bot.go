package bot

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"pkg.nit.so/switchboard"

	"github.com/nint8835/hopper/pkg/config"
)

type Bot struct {
	Session *discordgo.Session

	config      *config.Config
	parser      *switchboard.Switchboard
	quitChan    chan struct{}
	stoppedChan chan struct{}
}

func (b *Bot) Run() error {
	defer close(b.stoppedChan)
	b.Session.AddHandler(b.parser.HandleInteractionCreate)

	err := b.parser.SyncCommands(b.Session, b.config.DiscordAppId)
	if err != nil {
		return fmt.Errorf("error syncing commands: %w", err)
	}

	if err = b.Session.Open(); err != nil {
		return fmt.Errorf("error opening Discord connection: %w", err)
	}

	slog.Info("Discord bot running")

	<-b.quitChan
	slog.Info("Stopping bot...")

	if err = b.Session.Close(); err != nil {
		return fmt.Errorf("error closing Discord connection: %w", err)
	}

	return nil
}

func (b *Bot) Stop() {
	b.quitChan <- struct{}{}
	<-b.stoppedChan
}

func New(cfg *config.Config) (*Bot, error) {
	session, err := discordgo.New(fmt.Sprintf("Bot %s", cfg.DiscordToken))
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %w", err)
	}

	bot := &Bot{
		Session:     session,
		config:      cfg,
		parser:      &switchboard.Switchboard{},
		quitChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
	}

	return bot, nil
}
