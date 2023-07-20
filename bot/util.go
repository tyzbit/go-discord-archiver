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

// timeZoneOptions returns a []discordgo.SelectMenuOption for timezones
func timeZoneOffset(sc ServerConfig) (options []discordgo.SelectMenuOption) {
	for i := 0; i <= 14; i++ {
		description := ""
		if sc.UTCOffset.Valid && sc.UTCOffset.Int32 == int32(i) {
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

// timeZoneOptions returns a []discordgo.SelectMenuOption for timezones
func timeZoneSign(sc ServerConfig) (options []discordgo.SelectMenuOption) {
	signs := []string{"+", "-"}
	for _, s := range signs {
		description := ""
		if sc.UTCSign.Valid && sc.UTCSign.String == s {
			description = "Current value"
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:       s,
			Description: description,
			Value:       s,
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
