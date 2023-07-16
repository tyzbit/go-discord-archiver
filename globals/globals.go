package globals

import "github.com/bwmarrin/discordgo"

const (
	// Interactive command aliases
	Retry = "retry"

	// Commands
	Stats          = "stats"
	Settings       = "settings"
	Archive        = "archive"
	ArchiveMessage = "Get snapshots"
	Help           = "help"

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

	// Archive.org URL timestamp layout
	ArchiveOrgTimestampLayout = "20060102150405"

	// Shown to the user when `/help` is called
	BotHelpText = `**Usage**
	React to a message that has links with 🏛 (The "classical building" emoji) and the bot will respond in the channel with an archive.org link for the link(s). It saves the page to archive.org if needed. 
You can also right-click (or long press) a message and use "Get snapshots" to get a message with snapshots for any link that only you can see.

**This is a pretty good way to get around paywalls to read articles for free.**

Configure the bot with slash commands:
` + "`/settings`" + `

Get a snapshot for one URL in a message visible only to you:
` + "`/archive`" + `

Get stats for the bot:
` + "`/stats`" + `

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
			Description: "Archive a URL directly and privately, react with 🏛️ on a message instead for others to see it",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "url",
					Description: "URL to get a Wayback Machine snapshot for",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name: ArchiveMessage,
			Type: discordgo.MessageApplicationCommand,
		},
		{
			Name:        Stats,
			Description: "Show bot stats",
		},
		{
			Name:        Settings,
			Description: "Change settings",
		},
	}
	RegisteredCommands = make([]*discordgo.ApplicationCommand, len(Commands))
)
