package globals

import (
	"github.com/bwmarrin/discordgo"
)

const (
	// Interactive command aliases
	Retry = "retry"

	// Commands
	Settings                  = "settings"
	Archive                   = "archive"
	ArchiveMessage            = "Get saved snapshots"
	ArchiveMessagePrivate     = "Get saved snapshots (private)"
	ArchiveMessageNewSnapshot = "Take new snapshot"
	Help                      = "help"

	// Command options
	UrlOption             = "url"
	TakeNewSnapshotOption = "new"

	// Bot settings unique handler names
	// Booleans
	BotEnabled         = "enabled"
	AlwaysArchiveFirst = "alwayssnapshotfirst"
	Details            = "showdetails"
	RemoveRetry        = "removeretry"
	// Integers
	RetryAttempts    = "retries"
	RemoveRetryAfter = "removeretryafter"
	UTCOffset        = "utcoffset"
	// Strings
	UTCSign = "utcsign"

	// Colors
	FrenchGray = 13424349
	BrightRed  = 16711680

	// Archive.org URL timestamp layout
	ArchiveOrgTimestampLayout = "20060102150405"

	// Shown to the user when `/help` is called
	BotHelpText = `**Usage**
- Right-click (or long press) a message and use "Get snapshot" to post a message with snapshots for the links in the message. 
  - Use the private option for a message only you can see.
- Select "Take snapshot" to take a fresh snapshot of the live page.

**This is a pretty good way to get around paywalls to read articles for free.**

Configure the bot:

` + "`/settings`" + `

Get a snapshot for one URL in a message visible only to you (It will ask if you want to try to find an existing snapshot or take a new one):

` + "`/archive`" + `

Get this help message:

` + "`/help`"
	BotHelpFooterText = "It can take up to a few minutes for archive.org to save a page, so if you don't get a link immediately, please be patient."
)

var (
	MinAllowedRetryAttempts      = 0
	MinAllowedRetryAttemptsFloat = float64(MinAllowedRetryAttempts)

	MaxAllowedRetryAttempts      = 5
	MaxAllowedRetryAttemptsFloat = float64(MaxAllowedRetryAttempts)

	AllowedRetryAttemptRemovalDelayValues = []int{0, 10, 30, 90, 120, 300}
	// Enabled takes a boolean and returns "enabled" or "disabled"
	Enabled = map[bool]string{
		true:  "enabled",
		false: "disabled",
	}
	// Button style takes a boolean and returns a colorized button if true
	ButtonStyle = map[bool]discordgo.ButtonStyle{
		true:  discordgo.PrimaryButton,
		false: discordgo.SecondaryButton,
	}
	SettingFailedResponseMessage = "Error changing setting"
	Commands                     = []*discordgo.ApplicationCommand{
		{
			Name:        Help,
			Description: "How to use this bot",
		},
		{
			Name:        Archive,
			Description: "Archive a URL directly, set new to true to grab a fresh snapshot",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        UrlOption,
					Description: "URL to get a Wayback Machine snapshot for",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:        TakeNewSnapshotOption,
					Description: "Whether to take a new snapshot (True) or try to look for an existing one first (False)",
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Required:    true,
				},
			},
		},
		{
			Name: ArchiveMessage,
			Type: discordgo.MessageApplicationCommand,
		},
		{
			Name: ArchiveMessagePrivate,
			Type: discordgo.MessageApplicationCommand,
		},
		{
			Name: ArchiveMessageNewSnapshot,
			Type: discordgo.MessageApplicationCommand,
		},
		{
			Name:        Settings,
			Description: "Change settings",
		},
	}
	RegisteredCommands = make([]*discordgo.ApplicationCommand, len(Commands))
)
