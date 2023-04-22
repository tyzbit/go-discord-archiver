package globals

import "github.com/bwmarrin/discordgo"

const (
	// Interactive command aliases
	TakeCurrentSnapshot = "retry"
	// Commands
	Stats    = "stats"
	Settings = "settings"
	Help     = "help"
	// Bot settings
	BotEnabled         = "enabled"
	RetryAttempts      = "retries"
	AlwaysArchiveFirst = "alwayssnapshotfirst"
	Details            = "showdetails"
	// Colors
	FrenchGray = 13424349
	// Archive.org URL timestamp layout
	ArchiveOrgTimestampLayout = "20060102150405"

	// Help text
	BotHelpText = `**Usage**
	React to a message that has links with 🏛 (The "classical building" emoji) and the bot will respond in the channel with an archive.org link for the link(s). It saves the page to archive.org if needed.

**This is a pretty good way to get around paywalls to read articles for free.**

Configure the bot with slash commands:
` + "`/settings`" + `

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
	// Verb takes a boolean and returns "enabled" or "disabled"
	Verb = map[bool]string{
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
