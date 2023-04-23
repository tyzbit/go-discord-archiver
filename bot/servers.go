package bot

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

type ServerRegistration struct {
	DiscordId string `gorm:"primaryKey"`
	Name      string
	UpdatedAt time.Time
	Config    ServerConfig `gorm:"foreignKey:DiscordId"`
}

type ServerConfig struct {
	DiscordId          string `gorm:"primaryKey" pretty:"Server ID"`
	Name               string `pretty:"Server Name"`
	ArchiveEnabled     bool   `pretty:"Bot enabled"`
	AlwaysArchiveFirst bool   `pretty:"Archive the page first (slower)"`
	ShowDetails        bool   `pretty:"Show extra details"`
	RemoveRetries      bool   `pretty:"Remove the retry button automatically"`
	RetryAttempts      uint   `pretty:"Number of attempts to archive a URL"`
	RemoveRetriesDelay uint   `pretty:"Seconds to wait to remove retry button"`
	UpdatedAt          time.Time
}

var (
	defaultServerConfig ServerConfig = ServerConfig{
		DiscordId:          "0",
		Name:               "default",
		ArchiveEnabled:     true,
		AlwaysArchiveFirst: false,
		ShowDetails:        true,
		RemoveRetries:      true,
		RetryAttempts:      1,
		RemoveRetriesDelay: 30,
	}

	archiverRepoUrl string = "https://github.com/tyzbit/go-discord-archiver"
)

// registerOrUpdateServer checks if a guild is already registered in the database. If not,
// it creates it with sensibile defaults.
func (bot *ArchiverBot) registerOrUpdateServer(g *discordgo.Guild) error {
	// Do a lookup for the full guild object
	guild, err := bot.DG.Guild(g.ID)
	if err != nil {
		return fmt.Errorf("unable to look up server by id: %v", g.ID)
	}

	var registration ServerRegistration
	bot.DB.Find(&registration, g.ID)
	// The server registration does not exist, so we will create with defaults
	if (registration == ServerRegistration{}) {
		log.Info("creating registration for new server: ", guild.Name, "(", g.ID, ")")
		sc := defaultServerConfig
		sc.Name = guild.Name
		tx := bot.DB.Create(&ServerRegistration{
			DiscordId: g.ID,
			Name:      guild.Name,
			UpdatedAt: time.Now(),
			Config:    sc,
		})

		// We only expect one server to be updated at a time. Otherwise, return an error.
		if tx.RowsAffected != 1 {
			return fmt.Errorf("did not expect %v rows to be affected updating "+
				"server registration for server: %v(%v)", fmt.Sprintf("%v", tx.RowsAffected), guild.Name, g.ID)
		}
	}

	err = bot.updateServersWatched()
	if err != nil {
		return fmt.Errorf("unable to update servers watched: %v", err)
	}

	return nil
}

// getServerConfig takes a guild ID and returns a ServerConfig object for that server.
// If the config isn't found, it returns a default config.
func (bot *ArchiverBot) getServerConfig(guildId string) ServerConfig {
	sc := ServerConfig{}
	bot.DB.Where(&ServerConfig{DiscordId: guildId}).Find(&sc)
	if (sc == ServerConfig{}) {
		return defaultServerConfig
	}
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

	// We only expect one server to be updated at a time. Otherwise, return an error.
	if tx.RowsAffected != 1 {
		log.Errorf("did not expect %v rows to be affected updating "+
			"server config for server: %v(%v)", fmt.Sprintf("%v", tx.RowsAffected), guild.Name, guild.ID)
		return sc, false
	}
	return sc, true
}

// updateServersWatched updates the servers watched value
// in both the local bot stats and in the database. It is allowed to fail.
func (bot *ArchiverBot) updateServersWatched() error {
	var serversWatched int64
	bot.DB.Model(&ServerRegistration{}).Where(&ServerRegistration{}).Count(&serversWatched)

	updateStatusData := &discordgo.UpdateStatusData{Status: "online"}
	updateStatusData.Activities = make([]*discordgo.Activity, 1)
	updateStatusData.Activities[0] = &discordgo.Activity{
		Name: fmt.Sprintf("%v servers", serversWatched),
		Type: discordgo.ActivityTypeWatching,
		URL:  archiverRepoUrl,
	}

	if !bot.StartingUp {
		log.Debug("updating discord bot status")
		err := bot.DG.UpdateStatusComplex(*updateStatusData)
		if err != nil {
			return fmt.Errorf("unable to update discord bot status: %w", err)
		}
	}

	return nil
}
