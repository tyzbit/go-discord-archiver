package bot

import (
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
	globals "github.com/tyzbit/go-discord-archiver/globals"
)

// getFieldNamesByType takes an interface as an argument
// and returns an array of the field names. Ripped from
// https://stackoverflow.com/a/18927729
func convertFlatStructToSliceStringMap(i interface{}) []map[string]string {
	// Get reflections
	t := reflect.TypeOf(i)
	tv := reflect.ValueOf(i)

	// Keys is a list of keys of the values map. It's used for sorting later
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

	// Now we will sort the values returned above into a sortedValues
	sortedValues := make([]map[string]string, 0, t.NumField())
	for _, k := range keys {
		sortedValues = append(sortedValues, map[string]string{k: values[k]})
	}

	return sortedValues
}

// getTagValue looks up the tag for a given field of the specified type.
// Be advised, if the tag can't be found, it returns an empty string.
func getTagValue(i interface{}, field string, tag string) string {
	r, ok := reflect.TypeOf(i).FieldByName(field)
	if !ok {
		return ""
	}
	return r.Tag.Get(tag)
}

// Returns a multiline string that pretty prints botStats.
func structToPrettyDiscordFields(i any) []*discordgo.MessageEmbedField {
	var fields ([]*discordgo.MessageEmbedField)

	stringMapSlice := convertFlatStructToSliceStringMap(i)

	for _, stringMap := range stringMapSlice {
		for key, value := range stringMap {
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
func retryOptions() (options []discordgo.SelectMenuOption) {
	for i := globals.MinAllowedRetryAttempts; i <= globals.MaxAllowedRetryAttempts; i++ {
		options = append(options, discordgo.SelectMenuOption{
			Label: fmt.Sprint(i),
			Value: fmt.Sprint(i),
		})
	}
	return options
}

// settingsFailureIntegrationResponse returns a *discordgo.InteractionResponseData
// stating that a failure to update settings has occured.
func (bot *ArchiverBot) settingsFailureIntegrationResponse(sc ServerConfig) *discordgo.InteractionResponseData {
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
// stating that a failure to update settings has occured.
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

// deleteAllCommands is used but commented out in bot.go
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

// sendArchiveResponse sends the message with a result from archive.org
func (bot *ArchiverBot) sendArchiveResponse(message *discordgo.Message, reply *discordgo.MessageSend) error {
	username := ""
	user, err := bot.DG.User(message.Member.User.ID)
	if err != nil {
		log.Errorf("unable to look up user with ID %v, err: %v", message.Member.User.ID, err)
		username = "unknown"
	} else {
		username = user.Username
	}

	if message.GuildID != "" {
		// Do a lookup for the full guild object
		guild, gErr := bot.DG.Guild(message.GuildID)
		if gErr != nil {
			return gErr
		}
		bot.createMessageEvent(MessageEvent{
			AuthorId:       message.Member.User.ID,
			AuthorUsername: message.Member.User.Username,
			MessageId:      message.ID,
			ChannelId:      message.ChannelID,
			ServerID:       message.GuildID,
		})
		log.Debug("sending archive message response in ",
			guild.Name, "(", guild.ID, "), calling user: ",
			username, "(", message.Member.User.ID, ")")
	} else {
		log.Debug("declining archive message response in ",
			"calling user: ", username, "(", message.Member.User.ID, ")")
	}

	_, err = bot.DG.ChannelMessageSendComplex(message.ChannelID, reply)
	if err != nil {
		log.Errorf("problem sending message: %v", err)
	}
	return nil
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
