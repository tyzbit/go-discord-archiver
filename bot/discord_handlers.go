package bot

import (
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
	globals "github.com/tyzbit/go-discord-archiver/globals"
)

// BotReadyHandler is called when the bot is considered ready to use the Discord session
func (bot *ArchiverBot) BotReadyHandler(s *discordgo.Session, r *discordgo.Ready) {
	// r.Guilds has all of our connected servers, so we should
	// update server registrations and set any registered servers
	// not in r.Guilds as inactive
	bot.updateInactiveRegistrations(r.Guilds)

	// Use this to clean up commands if IDs have changed
	// TODO remove later if unnecessary
	// log.Debug("removing all commands")
	// bot.deleteAllCommands()
	// var err error
	// globals.RegisteredCommands, err = bot.DG.ApplicationCommandBulkOverwrite(bot.DG.State.User.ID, "", globals.Commands)
	log.Debug("registering slash commands")
	registeredCommands, err := bot.DG.ApplicationCommands(bot.DG.State.User.ID, "")
	if err != nil {
		log.Errorf("unable to look up registered application commands, err: %s", err)
	} else {
		for _, botCommand := range globals.Commands {
			for i, registeredCommand := range registeredCommands {
				// Check if this registered command matches a configured bot command
				if botCommand.Name == registeredCommand.Name {
					// Only update if it differs from what's already registered
					if botCommand != registeredCommand {
						editedCmd, err := bot.DG.ApplicationCommandEdit(bot.DG.State.User.ID, "", registeredCommand.ID, botCommand)
						if err != nil {
							log.Errorf("cannot update command %s: %v", botCommand.Name, err)
						}
						globals.RegisteredCommands = append(globals.RegisteredCommands, editedCmd)

						// Bot command was updated, so skip to the next bot command
						break
					}
				}

				// Check on the last item of registeredCommands
				if i == len(registeredCommands) {
					// This is a stale registeredCommand, so we should delete it
					err := bot.DG.ApplicationCommandDelete(bot.DG.State.User.ID, "", registeredCommand.ID)
					if err != nil {
						log.Errorf("cannot remove command %s: %v", registeredCommand.Name, err)
					}
				}
			}

			// If we're here, then we have a command that needs to be registered
			createdCmd, err := bot.DG.ApplicationCommandCreate(bot.DG.State.User.ID, "", botCommand)
			if err != nil {
				log.Errorf("cannot update command %s: %v", botCommand.Name, err)
			}
			globals.RegisteredCommands = append(globals.RegisteredCommands, createdCmd)
			if err != nil {
				log.Errorf("cannot update commands: %v", err)
			}
		}
	}

	err = bot.updateServersWatched()
	if err != nil {
		log.Error("unable to update servers watched")
	}
}

// GuildCreateHandler is called whenever the bot joins a new guild.
func (bot *ArchiverBot) GuildCreateHandler(s *discordgo.Session, gc *discordgo.GuildCreate) {
	if gc.Guild.Unavailable {
		return
	}

	err := bot.registerOrUpdateServer(gc.Guild, false)
	if err != nil {
		log.Errorf("unable to register or update server: %v", err)
	}
}

// GuildDeleteHandler is called whenever the bot leaves a guild.
func (bot *ArchiverBot) GuildDeleteHandler(s *discordgo.Session, gd *discordgo.GuildDelete) {
	if gd.Guild.Unavailable {
		return
	}

	log.Infof("guild %s(%s) deleted (bot was probably kicked)", gd.Guild.Name, gd.Guild.ID)
	err := bot.registerOrUpdateServer(gd.BeforeDelete, true)
	if err != nil {
		log.Errorf("unable to register or update server: %v", err)
	}
}

// InteractionInit configures all interactive commands
func (bot *ArchiverBot) InteractionHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	commandsHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		globals.Help: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			err := bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "🏛️ Archive.org Bot Help",
							Description: globals.BotHelpText,
							Footer: &discordgo.MessageEmbedFooter{
								Text: globals.BotHelpFooterText,
							},
							Color: globals.FrenchGray,
						},
					},
				},
			})

			if err != nil {
				log.Errorf("error responding to help command "+globals.Help+", err: %v", err)
			}
		},
		// bot.archiveInteraction can handle both the archive slash command and the app menu function
		globals.Archive:                   func(s *discordgo.Session, i *discordgo.InteractionCreate) { bot.archiveInteraction(i, false, false) },
		globals.ArchiveMessage:            func(s *discordgo.Session, i *discordgo.InteractionCreate) { bot.archiveInteraction(i, false, false) },
		globals.ArchiveMessagePrivate:     func(s *discordgo.Session, i *discordgo.InteractionCreate) { bot.archiveInteraction(i, false, true) },
		globals.ArchiveMessageNewSnapshot: func(s *discordgo.Session, i *discordgo.InteractionCreate) { bot.archiveInteraction(i, true, true) },
		globals.Settings: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			log.Debug("handling settings request")
			if i.GuildID == "" {
				// This is a DM, so settings cannot be changed
				err := bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: bot.settingsDMFailureIntegrationResponse(),
				})
				if err != nil {
					log.Errorf("error responding to settings DM"+globals.Settings+", err: %v", err)
				}
				return
			} else {
				guild, err := bot.DG.Guild(i.Interaction.GuildID)
				if err != nil {
					guild.Name = "GuildLookupError"
				}

				sc := bot.getServerConfig(i.GuildID)
				resp := bot.SettingsIntegrationResponse(sc)
				err = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: resp,
				})

				if err != nil {
					log.Errorf("error responding to slash command "+globals.Settings+", err: %v", err)
				}
			}
		},
	}

	buttonHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		globals.Retry: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			typingStop := make(chan bool, 1)
			go bot.typeInChannel(typingStop, i.ChannelID)

			// Remove retry button
			i.Message.Components = []discordgo.MessageComponent{}

			guild, err := bot.DG.Guild(i.Interaction.GuildID)
			if err != nil {
				guild.Name = "GuildLookupError"
			}

			interactionErr := bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Embeds:     i.Message.Embeds,
					Components: i.Message.Components,
					Flags:      i.Message.Flags,
				},
			})
			if interactionErr != nil {
				log.Errorf("error responding to archive message messagesToSend interaction, err: %v", interactionErr)
			}

			var messagesToBeSent []*discordgo.MessageSend
			var messageResponses []*discordgo.MessageSend
			var errs []error
			if i.Interaction != nil {
				i.Interaction.Message.GuildID = guild.ID
				messageResponses, errs = bot.buildMessageResponse(i.Interaction.Message, true)
				messagesToBeSent = append(messagesToBeSent, messageResponses...)
			} else {
				i.Message.GuildID = guild.ID
				messageResponses, errs = bot.buildMessageResponse(i.Message, true)
				messagesToBeSent = append(messagesToBeSent, messageResponses...)
			}

			for _, err := range errs {
				if err != nil {
					log.Errorf("problem handling archive request: %v", err)
				}
			}

			// This is necessary because the type is unknown
			if len(messagesToBeSent) == 0 {
				log.Warn("retry used but no messagesToSend was generated")
				typingStop <- true
				return
			}

			for index, messagesToSend := range messagesToBeSent {
				m := discordgo.Message{
					Member: &discordgo.Member{
						User: &discordgo.User{
							ID: i.Member.User.ID,
						},
					},
					GuildID:   i.GuildID,
					ChannelID: i.ChannelID,
				}

				if len(errs) >= index+1 {
					if errs[index] != nil {
						guild.Name = "None"
						guild.ID = "0"
					}
				}

				err = bot.sendArchiveResponse(&m, messagesToSend)

				if err != nil {
					log.Errorf("problem sending message: %v", err)
				}
			}

			// This only has an effect if the message is not ephemeral
			typingStop <- true
		},
		// Settings buttons/choices
		globals.BotEnabled: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			sc := bot.getServerConfig(i.GuildID)
			inverse := sc.ArchiveEnabled.Valid && !sc.ArchiveEnabled.Bool
			bot.respondToSettingsChoice(i, "archive_enabled", inverse)
		},
		globals.Details: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			sc := bot.getServerConfig(i.GuildID)
			inverse := sc.ShowDetails.Valid && !sc.ShowDetails.Bool
			bot.respondToSettingsChoice(i, "show_details", inverse)
		},
		globals.AlwaysArchiveFirst: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			sc := bot.getServerConfig(i.GuildID)
			inverse := sc.AlwaysArchiveFirst.Valid && !sc.AlwaysArchiveFirst.Bool
			bot.respondToSettingsChoice(i, "always_archive_first", inverse)
		},
		globals.UTCOffset: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			mcd := i.MessageComponentData()
			bot.respondToSettingsChoice(i, "utc_offset", mcd.Values[0])
		},
		globals.UTCSign: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			mcd := i.MessageComponentData()
			bot.respondToSettingsChoice(i, "utc_sign", mcd.Values[0])
		},
		globals.RetryAttempts: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			mcd := i.MessageComponentData()
			bot.respondToSettingsChoice(i, "retry_attempts", mcd.Values[0])
		},
		globals.RemoveRetryAfter: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			mcd := i.MessageComponentData()
			bot.respondToSettingsChoice(i, "remove_retries_delay", mcd.Values[0])
		},
	}

	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if h, ok := commandsHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	case discordgo.InteractionMessageComponent:
		if h, ok := buttonHandlers[i.MessageComponentData().CustomID]; ok {
			h(s, i)
		}
	}
}

// archiveInteraction is called by using /archive and using the "Get archived snapshots" app function.
func (bot *ArchiverBot) archiveInteraction(i *discordgo.InteractionCreate, newSnapshot bool, ephemeral bool) {
	log.Debug("handling archive command request")
	var flags discordgo.MessageFlags
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	// Send a response immediately that says the bot is thinking
	_ = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: flags,
		},
	})
	messagesToSend, errs := bot.buildInteractionResponse(i, newSnapshot)
	for _, err := range errs {
		if err != nil {
			log.Errorf("problem handling archive command request: %v", err)
		}
	}

	if len(messagesToSend) == 0 {
		log.Warn("no embeds were generated")
		return
	}

	for _, message := range messagesToSend {
		if message == nil {
			log.Errorf("empty message, not trying to send")
			message = &discordgo.MessageSend{
				Content: "Error handling interaction",
			}
		}
		err := bot.sendArchiveCommandResponse(i.Interaction, message)
		if err != nil {
			log.Errorf("problem sending message: %v", err)
		}
	}
}
