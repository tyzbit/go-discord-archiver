# go-discord-archiver

Discord bot for archive.org, written in Go

# NOTICE

The emoji functionality was deprecated September 1, 2024. Please see the
[issue on the issue tracker](https://github.com/tyzbit/go-discord-archiver/issues/38)
for more information.

## Help and data deletion requests

Join the Discord for help and for deletion requests

https://discord.gg/kvE2bbfYu3

## Configuration

Set some environment variables before launching, or add a `.env` file.

If database environment variables are provided, the bot will save configuration to an external database.
Otherwise, it will save configuration to a local sqlite database at `/var/go-discord-archiver/local.db`

| Variable          | Value(s)                                                                               |
| :---------------- | :------------------------------------------------------------------------------------- |
| ADMINISTRATOR_IDS | Comma separated IDs of users allowed to use administrator commands                     |
| DB_NAME           | Database name for database                                                             |
| DB_HOST           | Hostname for database                                                                  |
| DB_PASSWORD       | Password for database user                                                             |
| DB_USER           | Username for database user                                                             |
| LOG_LEVEL         | `trace`, `debug`, `info`, `warn`, `error`                                              |
| COOKIE            | Archive.org login cookie, get this from a web browser's Dev Tools visiting Archive.org |
| TOKEN             | The Discord token the bot should use                                                   |

## Usage

Right-click (or long press) a message and use "Get snapshot" to get a message with snapshots (or use the private option for a message only you can see) or select "Take snapshot" to take a fresh snapshot of the live page.

**This is a pretty good way to get around paywalls to read articles for free.**

### Commands

Configure the bot:

`/settings`

Get a snapshot for one URL in a message visible only to you (It will ask if you want to try to find an existing snapshot or take a new one):

`/archive`

Get this help message:

`/help`

### NOTES

- It can take up to a few minutes for archive.org to save a page, so if you don't get a link immediately, please be patient.

## Development

Create a `.env` file with your configuration, at the bare minimum you need
a Discord token for `TOKEN` and an Archive.org Cookie for `COOKIE` (Need at least `PHPSESSID`, `logged-in-sig` and `logged-in-user`, looks like: `PHPSESSID=12345; logged-in-sig=54321; logged-in-user=example%40example.com`).

Logins are currently good for a year.
You can either `docker compose up --build` to run with a mysql database, or just `go run main.go` to run with a sqlite database.
