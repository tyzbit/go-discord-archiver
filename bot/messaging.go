package bot

import (
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

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

		message, err = bot.DG.ChannelMessageSendComplex(message.ChannelID, reply)
		message.GuildID = guild.ID
		go bot.removeRetryButtonAfterSleep(message)
	} else {
		log.Debug("declining archive message response in ",
			"calling user: ", username, "(", message.Member.User.ID, ")")
	}

	if err != nil {
		log.Errorf("problem sending message: %v", err)
	}
	return nil
}

func (bot *ArchiverBot) removeRetryButtonAfterSleep(message *discordgo.Message) {
	guild, gErr := bot.DG.Guild(message.GuildID)
	if gErr != nil {
		log.Errorf("unable to look up server by id: %v", message.GuildID)

	}

	sc := bot.getServerConfig(guild.ID)
	var sleep int32
	if sc.RemoveRetriesDelay.Valid {
		sleep = sc.RemoveRetriesDelay.Int32
	} else {
		field := "RemoveRetriesDelay"
		log.Debugf("%s was not set, getting gorm default", field)
		gormDefault := getTagValue(ServerConfig{}, field, "gorm")
		if value, err := strconv.ParseInt(strings.Split(gormDefault, ":")[1], 10, 32); err != nil {
			log.Errorf("unable to get default gorm value for %s", field)
		} else {
			sleep = int32(value)
		}
	}
	time.Sleep(time.Duration(sleep) * time.Second)
	me := discordgo.MessageEdit{
		// Remove the components (button)
		Components: []discordgo.MessageComponent{},
		Embeds:     message.Embeds,
		ID:         message.ID,
		Channel:    message.ChannelID,
	}

	log.Debugf("removing reply button (waited %vs) for message ID %s in channel %s, guild: %s(%s)",
		sleep, message.ID, message.ChannelID, guild.Name, guild.ID)
	_, err := bot.DG.ChannelMessageEditComplex(&me)
	if err != nil {
		log.Errorf("unable to remove retry button on message id %v, server: %s(%s): %v, ",
			message.ID, message.GuildID, guild.Name, err)
	}
}
