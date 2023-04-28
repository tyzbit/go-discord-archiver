package bot

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
	globals "github.com/tyzbit/go-discord-archiver/globals"
	"gorm.io/gorm"
)

// ArchiverBot is the main type passed around throughout the code
// It has many functions for overall bot management
type ArchiverBot struct {
	DB         *gorm.DB
	DG         *discordgo.Session
	Config     ArchiverBotConfig
	StartingUp bool
}

// ArchiverBotConfig is attached to ArchiverBot so config settings can be
// accessed easily
type ArchiverBotConfig struct {
	AdminIds   []string `env:"ADMINISTRATOR_IDS"`
	DBHost     string   `env:"DB_HOST"`
	DBName     string   `env:"DB_NAME"`
	DBPassword string   `env:"DB_PASSWORD"`
	DBUser     string   `env:"DB_USER"`
	LogLevel   string   `env:"LOG_LEVEL"`
	Token      string   `env:"TOKEN"`
	Cookie     string   `env:"COOKIE"`
}

// BotReadyHandler is called when the bot is considered ready to use the Discord session
func (bot *ArchiverBot) BotReadyHandler(s *discordgo.Session, r *discordgo.Ready) {
	// Register all servers the bot is active in
	for _, g := range r.Guilds {
		err := bot.registerOrUpdateServer(g)
		if err != nil {
			log.Errorf("unable to register or update server: %v", err)
		}
	}

	bot.updateServerRegistrations(r.Guilds)

	// Use this to clean up commands if IDs have changed
	// TODO remove later if unnecessary
	// log.Debug("removing all commands")
	// bot.deleteAllCommands()
	// globals.RegisteredCommands, err = bot.DG.ApplicationCommandBulkOverwrite(bot.DG.State.User.ID, "", globals.Commands)
	log.Debug("registering slash commands")
	var err error
	existingCommands, err := bot.DG.ApplicationCommands(bot.DG.State.User.ID, "")
	for _, cmd := range globals.Commands {
		for _, existingCmd := range existingCommands {
			if existingCmd.Name == cmd.Name {
				editedCmd, err := bot.DG.ApplicationCommandEdit(bot.DG.State.User.ID, "", existingCmd.ID, cmd)
				if err != nil {
					log.Errorf("cannot update command %s: %v", cmd.Name, err)
				}
				globals.RegisteredCommands = append(globals.RegisteredCommands, editedCmd)
			} else {
				createdCmd, err := bot.DG.ApplicationCommandCreate(bot.DG.State.User.ID, "", cmd)
				if err != nil {
					log.Errorf("cannot update command %s: %v", cmd.Name, err)
				}
				globals.RegisteredCommands = append(globals.RegisteredCommands, createdCmd)

			}
		}
	}

	if err != nil {
		log.Errorf("cannot update commands: %v", err)
	}

	if bot.StartingUp {
		time.Sleep(time.Second * 10)
		bot.StartingUp = false
		err := bot.updateServersWatched()
		if err != nil {
			log.Error("unable to update servers watched")
		}
	}
}

// GuildCreateHandler is called whenever the bot joins a new guild. It is also lazily called upon initial
// connection to Discord
func (bot *ArchiverBot) GuildCreateHandler(s *discordgo.Session, gc *discordgo.GuildCreate) {
	if gc.Guild.Unavailable {
		return
	}

	err := bot.registerOrUpdateServer(gc.Guild)
	if err != nil {
		log.Errorf("unable to register or update server: %v", err)
	}
}

// This function will be called every time a new react is created on any message
// that the authenticated bot has access to
func (bot *ArchiverBot) MessageReactionAddHandler(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.MessageReaction.Emoji.Name == "🏛️" {
		var m *discordgo.Message
		// Guild ID is blank if the user is DMing us
		if r.MessageReaction.GuildID == "" {
			user, err := s.User(r.MessageReaction.UserID)
			if err != nil {
				log.Errorf("unable to look up user by id: %v", r.MessageReaction.UserID+", "+fmt.Sprintf("%v", err))
				return
			}
			// Create a fake message so that we can handle reacts
			// and interactions
			m = &discordgo.Message{
				ID: r.MessageReaction.MessageID,
				Member: &discordgo.Member{
					User: &discordgo.User{
						ID:       user.ID,
						Username: user.Username,
					},
				},
				ChannelID: r.ChannelID,
			}
		} else {
			// Create a fake message so that we can handle reacts
			// and interactions
			m = &discordgo.Message{
				ID: r.MessageID,
				Member: &discordgo.Member{
					User: &discordgo.User{
						ID:       r.Member.User.ID,
						Username: r.Member.User.Username,
					},
				},
				GuildID:   r.MessageReaction.GuildID,
				ChannelID: r.ChannelID,
			}
		}
		replies, errs := bot.handleArchiveRequest(r, false)
		for _, err := range errs {
			if err != nil {
				log.Errorf("problem handling archive request: %v", err)
			}
		}

		if replies == nil {
			log.Warn("no archive replies were returned")
			return
		}

		for _, reply := range replies {
			if r.MessageReaction.GuildID != "" {
				g, err := bot.DG.Guild(r.MessageReaction.GuildID)
				if err != nil {
					g.Name = "None"
				}
				bot.createMessageEvent(MessageEvent{
					AuthorId:       r.Member.User.ID,
					AuthorUsername: r.Member.User.Username,
					MessageId:      r.MessageReaction.MessageID,
					ChannelId:      r.MessageReaction.ChannelID,
					ServerID:       r.MessageReaction.GuildID,
					ServerName:     g.Name,
				})
			}
			err := bot.sendArchiveResponse(m, reply)
			if err != nil {
				log.Errorf("problem sending message: %v", err)
			}
		}
	}
}

// InteractionInit configures all interactive commands
func (bot *ArchiverBot) InteractionHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	commandsHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		globals.Help: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			err := bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: uint64(discordgo.MessageFlagsEphemeral),
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
		// Stats does not create an InteractionEvent
		globals.Stats: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			log.Debug(i.GuildID)
			directMessage := (i.GuildID == "")
			var stats botStats
			logMessage := ""
			if !directMessage {
				log.Debug("handling stats request")
				stats = bot.getServerStats(i.GuildID)
				guild, err := bot.DG.Guild(i.GuildID)
				if err != nil {
					log.Errorf("unable to look up server by id: %v", i.GuildID+", "+fmt.Sprintf("%v", err))
					return
				}
				logMessage = "sending stats response to " + i.Member.User.Username + "(" + i.Member.User.ID + ") in " +
					guild.Name + "(" + guild.ID + ")"
			} else {
				log.Debug("handling stats DM request")
				// We can be sure now the request was a direct message
				// Deny by default
				administrator := false

			out:
				for _, id := range bot.Config.AdminIds {
					if i.User.ID == id {
						administrator = true

						// This prevents us from checking all IDs now that
						// we found a match but is a fairly ineffectual
						// optimization since config.AdminIds will probably
						// only have dozens of IDs at most
						break out
					}
				}

				if !administrator {
					log.Errorf("did not respond to global stats command from %v(%v), because user is not an administrator",
						i.User.Username, i.User.ID)

					err := bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Embeds: []*discordgo.MessageEmbed{
								{
									Title: "Stats are not available in DMs",
									Color: globals.FrenchGray,
								},
							},
						},
					})

					if err != nil {
						log.Errorf("error responding to slash command "+globals.Stats+", err: %v", err)
					}
					return
				}
				stats = bot.getGlobalStats()
				logMessage = "sending global " + globals.Stats + " response to " + i.User.Username + "(" + i.User.ID + ")"
			}

			log.Info(logMessage)

			err := bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: uint64(discordgo.MessageFlagsEphemeral),
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:  "🏛️ Archive.org Bot Stats",
							Fields: structToPrettyDiscordFields(stats, directMessage),
							Color:  globals.FrenchGray,
						},
					},
				},
			})

			if err != nil {
				log.Errorf("error responding to slash command "+globals.Stats+", err: %v", err)
			}
		},
		globals.Settings: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			log.Debug("handling settings request")
			// This is a DM, so settings cannot be changed
			if i.GuildID == "" {
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

				bot.createInteractionEvent(InteractionEvent{
					UserID:        i.Interaction.Member.User.ID,
					Username:      i.Interaction.Member.User.Username,
					InteractionId: i.ID,
					ChannelId:     i.Interaction.ChannelID,
					ServerID:      i.Interaction.GuildID,
					ServerName:    guild.Name,
				})

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
			// Remove retry button
			i.Message.Components = []discordgo.MessageComponent{}

			guild, err := bot.DG.Guild(i.Interaction.GuildID)
			if err != nil {
				guild.Name = "None"
			}
			bot.createInteractionEvent(InteractionEvent{
				UserID:        i.Member.User.ID,
				Username:      i.Member.User.Username,
				InteractionId: i.Message.ID,
				ChannelId:     i.Message.ChannelID,
				ServerID:      guild.ID,
				ServerName:    guild.Name,
			})

			interactionErr := bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Embeds:     i.Message.Embeds,
					Components: i.Message.Components,
					Flags:      uint64(i.Message.Flags),
				},
			})
			if interactionErr != nil {
				log.Errorf("error responding to archive message reply interaction, err: %v", interactionErr)
			}

			// We trick handleArchiveRequest by giving it a fake message reaction
			replies, errs := bot.handleArchiveRequest(&discordgo.MessageReactionAdd{
				MessageReaction: &discordgo.MessageReaction{
					MessageID: i.Message.ID,
					ChannelID: i.ChannelID,
					GuildID:   i.GuildID,
				},
			}, true)

			for _, err := range errs {
				if err != nil {
					log.Errorf("problem handling archive request: %v", err)
				}
			}

			// This is necessary because the type is unknown
			if replies == nil {
				log.Warn("retry used but no reply was generated")
				return
			}

			for _, reply := range replies {
				if len(reply.Embeds) == 0 {
					log.Errorf("not sending an empty reply")
					break
				}
				m := discordgo.Message{
					Member: &discordgo.Member{
						User: &discordgo.User{
							ID: i.Member.User.ID,
						},
					},
					GuildID:   i.GuildID,
					ChannelID: i.ChannelID,
				}

				if err != nil {
					guild.Name = "None"
					guild.ID = "0"
				}
				bot.createMessageEvent(MessageEvent{
					AuthorId:       s.State.User.ID,
					AuthorUsername: i.Member.User.Username,
					MessageId:      i.Message.ID,
					ChannelId:      i.Message.ChannelID,
					ServerID:       guild.ID,
					ServerName:     guild.Name,
				})

				err := bot.sendArchiveResponse(&m, reply)
				if err != nil {
					log.Errorf("problem sending message: %v", err)
				}
			}
		},
		globals.BotEnabled: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			sc := bot.getServerConfig(i.GuildID)
			var interactionErr error

			inverse := sc.ArchiveEnabled.Valid && !sc.ArchiveEnabled.Bool
			sc, ok := bot.updateServerSetting(i.GuildID, "archive_enabled", inverse)

			guild, err := bot.DG.Guild(i.Interaction.GuildID)
			if err != nil {
				guild.Name = "None"
			}
			bot.createInteractionEvent(InteractionEvent{
				UserID:        i.Member.User.ID,
				Username:      i.Member.User.Username,
				InteractionId: i.Message.ID,
				ChannelId:     i.Message.ChannelID,
				ServerID:      i.Interaction.GuildID,
				ServerName:    guild.Name,
			})

			if i.GuildID == "" {
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.settingsDMFailureIntegrationResponse(),
				})
			} else if !ok {
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.settingsFailureIntegrationResponse(),
				})
			} else {
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.SettingsIntegrationResponse(sc),
				})
			}

			if interactionErr != nil {
				log.Errorf("error responding to settings interaction, err: %v", interactionErr)
			}
		},
		globals.Details: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			sc := bot.getServerConfig(i.GuildID)
			inverse := sc.ShowDetails.Valid && !sc.ShowDetails.Bool
			var interactionErr error
			sc, ok := bot.updateServerSetting(i.GuildID, "show_details", inverse)

			guild, err := bot.DG.Guild(i.Interaction.GuildID)
			if err != nil {
				guild.Name = "None"
			}
			bot.createInteractionEvent(InteractionEvent{
				UserID:        i.Member.User.ID,
				Username:      i.Member.User.Username,
				InteractionId: i.Message.ID,
				ChannelId:     i.Message.ChannelID,
				ServerID:      i.Interaction.GuildID,
				ServerName:    guild.Name,
			})

			if !ok {
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.settingsFailureIntegrationResponse(),
				})
			} else {
				nc := bot.getServerConfig(i.GuildID)
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.SettingsIntegrationResponse(nc),
				})
			}

			if interactionErr != nil {
				log.Errorf("error responding to settings interaction, err: %v", interactionErr)
			}
		},
		globals.AlwaysArchiveFirst: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			sc := bot.getServerConfig(i.GuildID)
			inverse := sc.AlwaysArchiveFirst.Valid && !sc.AlwaysArchiveFirst.Bool
			var interactionErr error
			sc, ok := bot.updateServerSetting(i.GuildID, "always_archive_first", inverse)

			guild, err := bot.DG.Guild(i.Interaction.GuildID)
			if err != nil {
				guild.Name = "None"
			}
			bot.createInteractionEvent(InteractionEvent{
				UserID:        i.Member.User.ID,
				Username:      i.Member.User.Username,
				InteractionId: i.Message.ID,
				ChannelId:     i.Message.ChannelID,
				ServerID:      i.Interaction.GuildID,
				ServerName:    guild.Name,
			})

			if !ok {
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.settingsFailureIntegrationResponse(),
				})
			} else {
				nc := bot.getServerConfig(i.GuildID)
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.SettingsIntegrationResponse(nc),
				})
			}

			if interactionErr != nil {
				log.Errorf("error responding to settings interaction, err: %v", interactionErr)
			}
		},
		globals.RemoveRetry: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			sc := bot.getServerConfig(i.GuildID)
			// If the value isn't valid, the setting should end up disabled
			inverse := sc.RemoveRetry.Valid && !sc.RemoveRetry.Bool
			var interactionErr error
			sc, ok := bot.updateServerSetting(i.GuildID, "remove_retry", inverse)

			guild, err := bot.DG.Guild(i.Interaction.GuildID)
			if err != nil {
				guild.Name = "None"
			}
			bot.createInteractionEvent(InteractionEvent{
				UserID:        i.Member.User.ID,
				Username:      i.Member.User.Username,
				InteractionId: i.Message.ID,
				ChannelId:     i.Message.ChannelID,
				ServerID:      i.Interaction.GuildID,
				ServerName:    guild.Name,
			})

			if !ok {
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.settingsFailureIntegrationResponse(),
				})
			} else {
				nc := bot.getServerConfig(i.GuildID)
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.SettingsIntegrationResponse(nc),
				})
			}

			if interactionErr != nil {
				log.Errorf("error responding to settings interaction, err: %v", interactionErr)
			}
		},
		globals.RetryAttempts: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			mcd := i.MessageComponentData()
			sc, ok := bot.updateServerSetting(i.GuildID, "retry_attempts", mcd.Values[0])
			var interactionErr error

			guild, err := bot.DG.Guild(i.Interaction.GuildID)
			if err != nil {
				guild.Name = "None"
			}
			bot.createInteractionEvent(InteractionEvent{
				UserID:        i.Member.User.ID,
				Username:      i.Member.User.Username,
				InteractionId: i.Message.ID,
				ChannelId:     i.Message.ChannelID,
				ServerID:      i.Interaction.GuildID,
				ServerName:    guild.Name,
			})

			if !ok {
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.settingsFailureIntegrationResponse(),
				})
			} else {
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.SettingsIntegrationResponse(sc),
				})
			}

			if interactionErr != nil {
				log.Errorf("error responding to settings interaction, err: %v", interactionErr)
			}
		},
		globals.RemoveRetryAfter: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			mcd := i.MessageComponentData()
			sc, ok := bot.updateServerSetting(i.GuildID, "remove_retries_delay", mcd.Values[0])
			var interactionErr error

			guild, err := bot.DG.Guild(i.Interaction.GuildID)
			if err != nil {
				guild.Name = "None"
			}
			bot.createInteractionEvent(InteractionEvent{
				UserID:        i.Member.User.ID,
				Username:      i.Member.User.Username,
				InteractionId: i.Message.ID,
				ChannelId:     i.Message.ChannelID,
				ServerID:      i.Interaction.GuildID,
				ServerName:    guild.Name,
			})

			if !ok {
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.settingsFailureIntegrationResponse(),
				})
			} else {
				interactionErr = bot.DG.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: bot.SettingsIntegrationResponse(sc),
				})
			}

			if interactionErr != nil {
				log.Errorf("error responding to settings interaction, err: %v", interactionErr)
			}
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
