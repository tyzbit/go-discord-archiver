package bot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	globals "github.com/tyzbit/go-discord-archiver/globals"
)

// retryOptions returns a []discordgo.SelectMenuOption for retry attempts
func retryOptions(sc ServerConfig) (options []discordgo.SelectMenuOption) {
	for i := globals.MinAllowedRetryAttempts; i <= globals.MaxAllowedRetryAttempts; i++ {

		description := ""
		if sc.RetryAttempts.Valid && int32(i) == sc.RetryAttempts.Int32 {
			description = "Current value"
		}

		options = append(options, discordgo.SelectMenuOption{
			Label:       fmt.Sprint(i),
			Description: description,
			Value:       fmt.Sprint(i),
		})
	}
	return options

}

// retryRemoveOptions returns a []discordgo.SelectMenuOption for retry removal delays
func retryRemoveOptions(sc ServerConfig) (options []discordgo.SelectMenuOption) {
	for _, value := range globals.AllowedRetryAttemptRemovalDelayValues {

		description := ""
		if sc.RemoveRetriesDelay.Valid && int32(value) == sc.RemoveRetriesDelay.Int32 {
			description = "Current value"
		}

		menuLabel := fmt.Sprint(value)
		if value == 0 {
			menuLabel = "Don't remove the retry button"
		}

		options = append(options, discordgo.SelectMenuOption{
			Label:       menuLabel,
			Value:       fmt.Sprint(value),
			Description: description,
		})
	}
	return options
}

// SettingsIntegrationResponse returns server settings in a *discordgo.InteractionResponseData
func (bot *ArchiverBot) SettingsIntegrationResponse(sc ServerConfig) *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{
		Flags: uint64(discordgo.MessageFlagsEphemeral),
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    getTagValue(sc, "ArchiveEnabled", "pretty"),
						Style:    globals.ButtonStyle[sc.ArchiveEnabled.Valid && sc.ArchiveEnabled.Bool],
						CustomID: globals.BotEnabled},
					discordgo.Button{
						Label:    getTagValue(sc, "ShowDetails", "pretty"),
						Style:    globals.ButtonStyle[sc.ShowDetails.Valid && sc.ShowDetails.Bool],
						CustomID: globals.Details},
					discordgo.Button{
						Label:    getTagValue(sc, "AlwaysArchiveFirst", "pretty"),
						Style:    globals.ButtonStyle[sc.AlwaysArchiveFirst.Valid && sc.AlwaysArchiveFirst.Bool],
						CustomID: globals.AlwaysArchiveFirst},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						Placeholder: getTagValue(sc, "RetryAttempts", "pretty"),
						CustomID:    globals.RetryAttempts,
						Options:     retryOptions(sc),
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						Placeholder: getTagValue(sc, "RemoveRetriesDelay", "pretty"),
						CustomID:    globals.RemoveRetryAfter,
						Options:     retryRemoveOptions(sc),
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						Placeholder: getTagValue(sc, "UTCOffset", "pretty"),
						CustomID:    globals.UTCOffset,
						Options:     timeZoneOffset(sc),
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						Placeholder: getTagValue(sc, "UTCSign", "pretty"),
						CustomID:    globals.UTCSign,
						Options:     timeZoneSign(sc),
					},
				},
			},
		},
	}
}

// settingsFailureIntegrationResponse returns a *discordgo.InteractionResponseData
// stating that a failure to update settings has occured
func (bot *ArchiverBot) settingsFailureIntegrationResponse() *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{
		Flags: uint64(discordgo.MessageFlagsEphemeral),
		Embeds: []*discordgo.MessageEmbed{
			{
				Title: "Unable to update setting",
				Color: globals.FrenchGray,
			},
		},
	}
}

// settingsFailureIntegrationResponse returns a *discordgo.InteractionResponseData
// stating that a failure to update settings has occured
func (bot *ArchiverBot) settingsDMFailureIntegrationResponse() *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{
		Flags: uint64(discordgo.MessageFlagsEphemeral),
		Embeds: []*discordgo.MessageEmbed{
			{
				Title: "The bot does not have any per-user settings",
				Color: globals.FrenchGray,
			},
		},
	}
}
