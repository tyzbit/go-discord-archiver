package globals

import "github.com/bwmarrin/discordgo"

const (
	// Interactive command aliases
	TakeCurrentSnapshot = "retry"
	// Commands
	Stats    = "stats"
	Settings = "settings"
	// Bot settings
	BotEnabled         = "enabled"
	RetryAttempts      = "retries"
	AlwaysArchiveFirst = "alwayssnapshotfirst"
	Details            = "showdetails"
	// Colors
	FrenchGray = 13424349
	// Archive.org URL timestamp layout
	ArchiveOrgTimestampLayout = "20060102150405"
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
			Name:        Stats,
			Description: "Show bot stats",
		},
		{
			Name:        Settings,
			Description: "Change bot settings",
		},
	}
	RegisteredCommands = make([]*discordgo.ApplicationCommand, len(Commands))
)
