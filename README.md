# Hopper

_A RSS feed reader bot for Discord_

## Deployment

Hopper is distributed as a Docker container, available from `ghcr.io/nint8835/hopper`. It can be ran via your container orchestrator of choice. It expects the following environment variables:

- `HOPPER_DATABASE_PATH`: Path to the database file. Will be created if it doesn't exist.
  - This should be a path on a volume mount of some sort, so the data can persist across bot restarts.
- `HOPPER_DISCORD_APP_ID`: ID of the Discord application for the bot.
- `HOPPER_DISCORD_GUILD_ID`: ID of the guild the bot is for.
- `HOPPER_DISCORD_CHANNEL_ID`: ID of the channel to post new feed items in.
  - The intended workflow is this channel should be configured so that only the bot user can post & create threads in it, and users are restricted to just posting in existing threads.
- `HOPPER_DISCORD_TOKEN`: Token for the bot.

## Usage

Once deployed, Hopper will check for new feed items once an hour (this is configurable via `HOPPER_POLL_INTERVAL`), and will post any new items in the specified channel and create a thread to discuss the item.

Feeds are managed via 3 slash commands - `/add`, `/list`, and `/remove`
