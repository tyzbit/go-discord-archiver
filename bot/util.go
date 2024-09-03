package bot

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

// getTagValue looks up the tag for a given field of the specified type
// Be advised, if the tag can't be found, it returns an empty string
func getTagValue(i interface{}, field string, tag string) string {
	r, ok := reflect.TypeOf(i).FieldByName(field)
	if !ok {
		return ""
	}
	return r.Tag.Get(tag)
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
