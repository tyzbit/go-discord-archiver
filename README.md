# go-discord-archiver
Discord bot for archive.org, written in Go



## Configuration

Set some environment variables before launching, or add a `.env` file.

If database environment variables are provided, the bot will save stats to an external database.
Otherwise, it will save stats to a local sqlite database at `/var/go-discord-archiver/local.db`

| Variable | Value(s) |
|:-|:-|
| ADMINISTRATOR_IDS | IDs of users allowed to use administrator commands |
| DB_DATABASE | Database name for database
| DB_HOST | Hostname for database |
| DB_PASSWORD | Password for database user |
| DB_USER | Username for database user |
| LOG_LEVEL | `trace`, `debug`, `info`, `warn`, `error` |
| TOKEN | The Discord token the bot should use |

## Usage

Configure the bot with `!archive config [setting] [value]`. The settings are below:

| Setting | Default | Description |
|:-|:-|:-|
| switch | `on` | Enable the bot: `on`, disable the bot: `off` |
| replyto | `off` | Reply to the original message for context, `on` or `off` (embed must be off) |
| embed | `on` | Whether to use an embed message or just reply with links (Discord will then auto preview them), `on` or `off` |
| archive | `on` | Whether or not to try archiving pages that don't already have a saved version |

You can also use `!archive stats` to get archive stats for your server.

## Development

Create a `.env` file with your configuration, at the bare minimum you need
a Discord token for `TOKEN`. You can either `docker compose up --build` to run 
with a mysql database, or just `go run main.go` to run with a sqlite database.