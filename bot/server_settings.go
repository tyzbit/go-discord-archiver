package bot

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

const archiverRepoUrl string = "https://github.com/tyzbit/go-discord-archiver"

// registerOrUpdateServer checks if a guild is already registered in the database. If not,
// it creates it with sensibile defaults
func (bot *ArchiverBot) registerOrUpdateServer(g *discordgo.Guild, delete bool) error {
	status := sql.NullBool{Valid: true, Bool: true}
	if delete {
		status = sql.NullBool{Valid: true, Bool: false}
	}

	var registration ServerRegistration
	bot.DB.Find(&registration, g.ID)
	// The server registration does not exist, so we will create with defaults
	if registration.Name == "" {
		log.Infof("creating registration for new server: %s(%s)", g.Name, g.ID)
		registration = ServerRegistration{
			DiscordId: g.ID,
			Name:      g.Name,
			Active:    sql.NullBool{Valid: true, Bool: true},
			UpdatedAt: time.Now(),
			JoinedAt:  g.JoinedAt,
			Config: ServerConfig{
				Name: g.Name,
			},
		}
		tx := bot.DB.Create(&registration)

		// We only expect one server to be updated at a time. Otherwise, return an error
		if tx.RowsAffected != 1 {
			return fmt.Errorf("did not expect %v rows to be affected updating "+
				"server registration for server: %v(%v)", fmt.Sprintf("%v", tx.RowsAffected), g.Name, g.ID)
		}
	}

	// Update the registration if the DB is wrong or if the server
	// was deleted (the bot left) or if JoinedAt is not set
	// (field was added later so early registrations won't have it)
	if registration.Active != status || registration.JoinedAt.IsZero() {
		log.Debugf("updating server %s", g.Name)
		bot.DB.Model(&ServerRegistration{}).
			Where(&ServerRegistration{DiscordId: registration.DiscordId}).
			Updates(&ServerRegistration{Active: status, JoinedAt: g.JoinedAt})
		_ = bot.updateServersWatched()
	}

	return nil
}

// updateInactiveRegistrations goes through every server registration and
// updates the DB as to whether or not it's active
func (bot *ArchiverBot) updateInactiveRegistrations(activeGuilds []*discordgo.Guild) {
	var sr []ServerRegistration
	var inactiveRegistrations []string
	bot.DB.Find(&sr)

	// Check all registrations for whether or not the server is active
	for _, reg := range sr {
		active := false
		for _, g := range activeGuilds {
			if g.ID == reg.DiscordId {
				active = true
				break
			}
		}

		// If the server isn't found in activeGuilds, then we have a config
		// for a server we're not in anymore
		if !active {
			inactiveRegistrations = append(inactiveRegistrations, reg.DiscordId)
		}
	}

	// Since active servers will set Active to true, we will
	// set the rest of the servers as inactive.
	registrations := int64(len(inactiveRegistrations))
	if registrations > 0 {
		tx := bot.DB.Model(&ServerRegistration{}).Where(inactiveRegistrations).
			Updates(&ServerRegistration{Active: sql.NullBool{Valid: true, Bool: false}})

		if tx.RowsAffected != registrations {
			log.Errorf("unexpected number of rows affected updating %v inactive "+
				"server registrations, rows updated: %v",
				registrations, tx.RowsAffected)
		}
	}
}

// getServerConfig takes a guild ID and returns a ServerConfig object for that server
// If the config isn't found, it returns a default config
func (bot *ArchiverBot) getServerConfig(guildId string) ServerConfig {
	// Default server config in case guild lookup fails, these are used for DMs
	sc := ServerConfig{
		DiscordId:          "",
		Name:               "",
		ArchiveEnabled:     sql.NullBool{Bool: true, Valid: true},
		AlwaysArchiveFirst: sql.NullBool{Bool: false, Valid: true},
		ShowDetails:        sql.NullBool{Bool: true, Valid: true},
		RetryAttempts:      sql.NullInt32{Int32: 1, Valid: true},
		RemoveRetriesDelay: sql.NullInt32{Int32: 30, Valid: true},
		UTCOffset:          sql.NullInt32{Int32: 4, Valid: true},
		UTCSign:            sql.NullString{String: "-", Valid: true},
		UpdatedAt:          time.Now(),
	}
	// If this fails, we'll return a default server
	// config, which is expected
	bot.DB.Where(&ServerConfig{DiscordId: guildId}).Find(&sc)
	return sc
}

// updateServerSetting updates a server setting according to the
// column name (setting) and the value
func (bot *ArchiverBot) updateServerSetting(guildID string, setting string,
	value interface{}) (sc ServerConfig, success bool) {
	guild, err := bot.DG.Guild(guildID)
	if err != nil {
		log.Errorf("unable to look up server by id: %v", guildID)
		return sc, false
	}

	tx := bot.DB.Model(&ServerConfig{}).Where(&ServerConfig{DiscordId: guild.ID}).
		Update(setting, value)

	ok := true
	// We only expect one server to be updated at a time. Otherwise, return an error
	if tx.RowsAffected != 1 {
		log.Errorf("did not expect %v rows to be affected updating "+
			"server config for server: %v(%v)", fmt.Sprintf("%v", tx.RowsAffected), guild.Name, guild.ID)
		ok = false
	}
	return bot.getServerConfig(guildID), ok
}

// respondToSettingsChoice updates a server setting according to the
// column name (setting) and the value
func (bot *ArchiverBot) respondToSettingsChoice(i *discordgo.InteractionCreate,
	setting string, value interface{}) {
	sc, ok := bot.updateServerSetting(i.Interaction.GuildID, setting, value)
	var interactionErr error

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
}

// updateServersWatched updates the servers watched value
// in both the local bot stats and in the database. It is allowed to fail
func (bot *ArchiverBot) updateServersWatched() error {
	var serversConfigured, serversActive int64
	bot.DB.Model(&ServerRegistration{}).Where(&ServerRegistration{}).Count(&serversConfigured)
	serversActive = int64(len(bot.DG.State.Ready.Guilds))
	log.Debugf("total number of servers configured: %v, connected servers: %v", serversConfigured, serversActive)

	updateStatusData := &discordgo.UpdateStatusData{Status: "online"}
	updateStatusData.Activities = make([]*discordgo.Activity, 1)
	updateStatusData.Activities[0] = &discordgo.Activity{
		Name: fmt.Sprintf("%v servers", serversActive),
		Type: discordgo.ActivityTypeWatching,
		URL:  archiverRepoUrl,
	}

	log.Debug("updating discord bot status")
	err := bot.DG.UpdateStatusComplex(*updateStatusData)
	if err != nil {
		return fmt.Errorf("unable to update discord bot status: %w", err)
	}

	return nil
}
