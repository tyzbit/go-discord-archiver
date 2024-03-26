package bot

import (
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

// sendArchiveResponse sends the message with a result from archive.org
func (bot *ArchiverBot) sendArchiveResponse(userMessage *discordgo.Message, messagesToSend *discordgo.MessageSend) error {
	username := ""
	user, err := bot.DG.User(userMessage.Member.User.ID)
	if err != nil {
		log.Errorf("unable to look up user with ID %v, err: %v", userMessage.Member.User.ID, err)
		username = "unknown"
	} else {
		username = user.Username
	}

	var guild *discordgo.Guild
	if userMessage.GuildID != "" {
		var gErr error
		// Do a lookup for the full guild object
		guild, gErr = bot.DG.Guild(userMessage.GuildID)
		if gErr != nil {
			return gErr
		}
		log.Debugf("sending archive message response in %s(%s), calling user: %s(%s)",
			guild.Name, guild.ID, username, userMessage.Member.User.ID)
	}

	botMessage, err := bot.DG.ChannelMessageSendComplex(userMessage.ChannelID, messagesToSend)
	// For some reason, this message is absent a Guild ID, so we copy from the previous message
	if guild.ID != "" {
		botMessage.GuildID = guild.ID
	}

	if err != nil {
		log.Errorf("problem sending message: %v", err)
		return err
	}

	go bot.removeRetryButtonAfterSleep(botMessage)
	return nil
}

// sendArchiveResponse sends the message with a result from archive.org
func (bot *ArchiverBot) sendArchiveCommandResponse(i *discordgo.Interaction, message *discordgo.MessageSend) error {
	username := ""
	var user *discordgo.User
	var err error
	if i.User != nil {
		user, err = bot.DG.User(i.User.ID)
	} else {
		user, err = bot.DG.User(i.Member.User.ID)
	}
	if err != nil {
		log.Errorf("unable to look up user with ID %v, err: %v", i.User.ID, err)
		username = "unknown"
	} else {
		username = user.Username
	}

	if i.GuildID != "" {
		// Do a lookup for the full guild object
		guild, gErr := bot.DG.Guild(i.GuildID)
		if gErr != nil {
			return gErr
		}
		log.Debugf("sending archive message response in %s(%s), calling user: %s(%s)",
			guild.Name, guild.ID, username, user.ID)
	}

	interactionMessage, err := bot.DG.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Embeds:     &message.Embeds,
		Components: &message.Components,
	})

	if err != nil {
		return err
	}

	// For some reason, this message is absent a Guild ID, so we copy from the previous message
	if i.GuildID != "" {
		interactionMessage.GuildID = i.GuildID
	}

	// We don't remove the reply button because one, the message is visible only
	// to the calling user, so the space it takes up shouldn't matter (they
	// can dismiss the message entirely as well). Second, it doesn't seem it's
	// possible to edit that kind of message ¯\_(ツ)_/¯
	return nil
}

func (bot *ArchiverBot) removeRetryButtonAfterSleep(message *discordgo.Message) {
	var guild *discordgo.Guild
	var gErr error
	guild, gErr = bot.DG.Guild(message.GuildID)
	if gErr != nil || guild.ID == "" {
		log.Errorf("unable to look up server by id: %v", message.GuildID)
		message.GuildID = ""
		guild = &discordgo.Guild{
			Name: "GuildLookupError",
		}
	}

	sc := bot.getServerConfig(guild.ID)
	var sleep int32
	if sc.RemoveRetriesDelay.Valid {
		if sc.RemoveRetriesDelay.Int32 == 0 {
			// 0 is disabled
			return
		}
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

	log.Debugf("removing retry button (waited %vs) for message ID %s in channel %s, guild: %s(%s)",
		sleep, message.ID, message.ChannelID, sc.Name, sc.DiscordId)
	_, err := bot.DG.ChannelMessageEditComplex(&me)
	if err != nil {
		log.Errorf("unable to remove retry button on message id %v, server: %s(%s): %v, ",
			message.ID, message.GuildID, guild.Name, err)
	}
}
