package bot

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

type ServerRegistration struct {
	DiscordId string `gorm:"primaryKey"`
	Name      string `gorm:"default:default"`
	UpdatedAt time.Time
	Active    sql.NullBool `pretty:"Bot is active in the server" gorm:"default:true"`
	Config    ServerConfig `gorm:"foreignKey:DiscordId"`
}

type ServerConfig struct {
	DiscordId          string        `gorm:"primaryKey" pretty:"Server ID"`
	Name               string        `pretty:"Server Name" gorm:"default:default"`
	ArchiveEnabled     sql.NullBool  `pretty:"Bot enabled" gorm:"default:true"`
	AlwaysArchiveFirst sql.NullBool  `pretty:"Archive the page first (slower)" gorm:"default:false"`
	ShowDetails        sql.NullBool  `pretty:"Show extra details" gorm:"default:true"`
	RemoveRetry        sql.NullBool  `pretty:"Remove the retry button automatically" gorm:"default:true"`
	RetryAttempts      sql.NullInt32 `pretty:"Number of times to retry calling archive.org" gorm:"default:1"`
	RemoveRetriesDelay sql.NullInt32 `pretty:"Seconds to wait to remove retry button" gorm:"default:30"`
	UpdatedAt          time.Time
}

const archiverRepoUrl string = "https://github.com/tyzbit/go-discord-archiver"

// registerOrUpdateServer checks if a guild is already registered in the database. If not,
// it creates it with sensibile defaults
func (bot *ArchiverBot) registerOrUpdateServer(g *discordgo.Guild) error {
	// Do a lookup for the full guild object
	guild, err := bot.DG.Guild(g.ID)
	if err != nil {
		return fmt.Errorf("unable to look up server by id: %v", g.ID)
	}

	var registration ServerRegistration
	bot.DB.Find(&registration, g.ID)
	active := sql.NullBool{Valid: true, Bool: true}
	// The server registration does not exist, so we will create with defaults
	if registration.Name == "default" {
		log.Info("creating registration for new server: ", guild.Name, "(", g.ID, ")")
		tx := bot.DB.Create(&ServerRegistration{
			DiscordId: g.ID,
			Name:      guild.Name,
			Active:    active,
			UpdatedAt: time.Now(),
			Config: ServerConfig{
				Name: guild.Name,
			},
		})

		// We only expect one server to be updated at a time. Otherwise, return an error
		if tx.RowsAffected != 1 {
			return fmt.Errorf("did not expect %v rows to be affected updating "+
				"server registration for server: %v(%v)", fmt.Sprintf("%v", tx.RowsAffected), guild.Name, g.ID)
		}
	}

	// Sort of a migration and also a catch-all for registrations that
	// are not properly saved in the database
	if registration.Active != active {
		bot.DB.Model(&ServerRegistration{}).
			Where(&ServerRegistration{DiscordId: registration.DiscordId}).
			Updates(&ServerRegistration{Active: active})
	}

	return nil
}

// updateInactiveRegistrations goes through every server registration and
// updates the DB as to whether or not it's active
func (bot *ArchiverBot) updateInactiveRegistrations(activeGuilds []*discordgo.Guild) {
	var sr []ServerRegistration
	bot.DB.Find(&sr)
	var status sql.NullBool

	// Check all registrations for whether or not the server is active
	for _, reg := range sr {
		// If there is no guild in r.Guilds, then we havea config
		// for a server we're not in anymore
		status = sql.NullBool{Valid: true, Bool: false}
		for _, g := range activeGuilds {
			if g.ID == reg.DiscordId {
				status = sql.NullBool{Valid: true, Bool: true}
			}
		}

		// Now the registration is accurate, update the DB if needed
		if reg.Active != status {
			reg.Active = status
			tx := bot.DB.Model(&ServerRegistration{}).Where(&ServerRegistration{DiscordId: reg.DiscordId}).
				Updates(reg)

			if tx.RowsAffected != 1 {
				log.Errorf("unexpected number of rows affected updating server registration, id: %s, rows updated: %v",
					reg.DiscordId, tx.RowsAffected)
			}
		}
	}
}

// getServerConfig takes a guild ID and returns a ServerConfig object for that server
// If the config isn't found, it returns a default config
func (bot *ArchiverBot) getServerConfig(guildId string) ServerConfig {
	sc := ServerConfig{}
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

	// Now we get the current server config and return it
	sc = bot.getServerConfig(guildID)

	// We only expect one server to be updated at a time. Otherwise, return an error
	if tx.RowsAffected != 1 {
		log.Errorf("did not expect %v rows to be affected updating "+
			"server config for server: %v(%v)", fmt.Sprintf("%v", tx.RowsAffected), guild.Name, guild.ID)
		return sc, false
	}
	return sc, true
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
