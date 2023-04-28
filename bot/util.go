package bot

import (
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
	globals "github.com/tyzbit/go-discord-archiver/globals"
)

// getFieldNamesByType takes an interface as an argument
// and returns an array of the field names. Ripped from
// https://stackoverflow.com/a/18927729
func convertFlatStructToSliceStringMap(i interface{}) []map[string]string {
	t := reflect.TypeOf(i)
	tv := reflect.ValueOf(i)

	// Keys is a list of keys of the values map
	// It's used for alphanumeric sorting later
	keys := make([]string, 0, t.NumField())

	// Values is an object that will hold an unsorted representation
	// of the interface
	values := map[string]string{}

	// Convert the struct to map[string]string
	for i := 0; i < t.NumField(); i++ {
		k := t.Field(i).Name
		v := tv.Field(i)
		values[k] = fmt.Sprintf("%v", v)
		keys = append(keys, k)
	}

	sort.Strings(keys)
	sortedValues := make([]map[string]string, 0, t.NumField())
	for _, k := range keys {
		sortedValues = append(sortedValues, map[string]string{k: values[k]})
	}

	return sortedValues
}

// getTagValue looks up the tag for a given field of the specified type
// Be advised, if the tag can't be found, it returns an empty string
func getTagValue(i interface{}, field string, tag string) string {
	r, ok := reflect.TypeOf(i).FieldByName(field)
	if !ok {
		return ""
	}
	return r.Tag.Get(tag)
}

// Returns a multiline string that pretty prints botStats
func structToPrettyDiscordFields(i any, globalMessage bool) []*discordgo.MessageEmbedField {
	var fields ([]*discordgo.MessageEmbedField)

	stringMapSlice := convertFlatStructToSliceStringMap(i)

	for _, stringMap := range stringMapSlice {
		for key, value := range stringMap {
			globalKey := getTagValue(i, key, "global") == "true"
			// If this key is a global key but
			// the message is not a global message, skip adding the field
			if globalKey && !globalMessage {
				continue
			}
			formattedKey := getTagValue(i, key, "pretty")
			newField := discordgo.MessageEmbedField{
				Name:   formattedKey,
				Value:  fmt.Sprintf("%v", value),
				Inline: getTagValue(i, key, "inline") == "",
			}
			fields = append(fields, &newField)
		}
	}

	return fields
}

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

		options = append(options, discordgo.SelectMenuOption{
			Label:       fmt.Sprint(value),
			Value:       fmt.Sprint(value),
			Description: description,
		})
	}
	return options
}

// typeInChannel sets the typing indicator for a channel. The indicator is cleared
// when a message is sent
func (bot *ArchiverBot) typeInChannel(channel chan bool, channelID string) {
	for {
		select {
		case <-channel:
			return
		default:
			if err := bot.DG.ChannelTyping(channelID); err != nil {
				log.Error("unable to set typing indicator: ", err)
			}
			time.Sleep(time.Second * 5)
		}
	}
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
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    getTagValue(sc, "AlwaysArchiveFirst", "pretty"),
						Style:    globals.ButtonStyle[sc.AlwaysArchiveFirst.Valid && sc.AlwaysArchiveFirst.Bool],
						CustomID: globals.AlwaysArchiveFirst},
					discordgo.Button{
						Label:    getTagValue(sc, "RemoveRetry", "pretty"),
						Style:    globals.ButtonStyle[sc.RemoveRetry.Valid && sc.RemoveRetry.Bool],
						CustomID: globals.RemoveRetry},
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

// deleteAllCommands is referenced in bot.go (but is probably commented out)
func (bot *ArchiverBot) deleteAllCommands() {
	globalCommands, err := bot.DG.ApplicationCommands(bot.DG.State.User.ID, "")
	if err != nil {
		log.Fatalf("could not fetch registered global commands: %v", err)
	}
	for _, command := range globalCommands {
		err = bot.DG.ApplicationCommandDelete(bot.DG.State.User.ID, "", command.ID)
		if err != nil {
			log.Panicf("cannot delete '%v' command: %v", command.Name, err)
		}
	}
}

// getDomainName receives a URL and returns the FQDN
func getDomainName(s string) (string, error) {
	url, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("unable to determine domain name for url: %v", s)
	}

	hostname := strings.TrimPrefix(url.Hostname(), "www.")
	return hostname, nil
}
