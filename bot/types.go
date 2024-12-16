package bot

import (
	"database/sql"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// Events
// // Every successful ArchiveEventEvent will come from a message
// type ArchiveEventEvent struct {
// 	CreatedAt      time.Time
// 	UUID           string `gorm:"primaryKey;uniqueIndex"`
// 	AuthorId       string
// 	AuthorUsername string
// 	ChannelId      string
// 	MessageId      string
// 	ServerID       string `gorm:"index"`
// 	ServerName     string
// 	ArchiveEvents  []ArchiveEvent `gorm:"foreignKey:ArchiveEventEventUUID"`
// }

// This is the representation of request and response URLs from users or
// the Archiver API
type ArchiveEvent struct {
	CreatedAt             time.Time
	UUID                  string `gorm:"primaryKey;uniqueIndex"`
	ArchiveEventEventUUID string
	ServerID              string `gorm:"index"`
	ServerName            string
	RequestURL            string
	RequestDomainName     string `gorm:"index"`
	ResponseURL           string
	ResponseDomainName    string `gorm:"index"`
	Cached                bool
}

// Handlers
// ArchiverBot is the main type passed around throughout the code
// It has many functions for overall bot management
type ArchiverBot struct {
	DB     *gorm.DB
	DG     *discordgo.Session
	Config ArchiverBotConfig
}

// ArchiverBotConfig is attached to ArchiverBot so config settings can be
// accessed easily
type ArchiverBotConfig struct {
	DBHost                string `env:"DB_HOST"`
	DBName                string `env:"DB_NAME"`
	DBPassword            string `env:"DB_PASSWORD"`
	DBUser                string `env:"DB_USER"`
	ReregisterAllCommands bool   `env:"REREGISTER_COMMANMDS"`
	LogLevel              string `env:"LOG_LEVEL"`
	Token                 string `env:"TOKEN"`
	Cookie                string `env:"COOKIE"`
}

// Servers
type ServerRegistration struct {
	DiscordId string `gorm:"primaryKey;uniqueIndex"`
	Name      string
	UpdatedAt time.Time
	JoinedAt  time.Time
	Active    sql.NullBool `pretty:"Bot is active in the server" gorm:"default:true"`
	Config    ServerConfig `gorm:"foreignKey:DiscordId"`
}

type ServerConfig struct {
	DiscordId          string         `gorm:"primaryKey;uniqueIndex" pretty:"Server ID"`
	Name               string         `pretty:"Server Name" gorm:"default:default"`
	ArchiveEnabled     sql.NullBool   `pretty:"Bot enabled" gorm:"default:true"`
	AlwaysArchiveFirst sql.NullBool   `pretty:"Archive the page first (slower)" gorm:"default:false"`
	ShowDetails        sql.NullBool   `pretty:"Show extra details" gorm:"default:true"`
	RetryAttempts      sql.NullInt32  `pretty:"Number of times to retry calling archive.org" gorm:"default:1"`
	RemoveRetriesDelay sql.NullInt32  `pretty:"Seconds to wait to remove retry button" gorm:"default:30"`
	UTCOffset          sql.NullInt32  `pretty:"UTC Offset" gorm:"default:4"`
	UTCSign            sql.NullString `pretty:"UTC Sign (Negative if west of Greenwich)" gorm:"default:-"`
	UpdatedAt          time.Time
}
