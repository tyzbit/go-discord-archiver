package bot

import (
	"database/sql"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// Events
// A MessageEvent is created when we receive a message that
// requires our attention
type MessageEvent struct {
	CreatedAt          time.Time
	UUID               string `gorm:"primaryKey" gorm:"uniqueIndex"`
	AuthorId           string `gorm:"index"`
	AuthorUsername     string
	MessageId          string
	ChannelId          string
	ServerID           string `gorm:"index"`
	ServerName         string
	ArchiveEventEvents []ArchiveEventEvent `gorm:"foreignKey:UUID"`
}

// A InteractionEvent when a user interacts with an Embed
type InteractionEvent struct {
	CreatedAt          time.Time
	UUID               string `gorm:"primaryKey" gorm:"uniqueIndex"`
	UserID             string `gorm:"index"`
	Username           string
	InteractionId      string
	ChannelId          string
	ServerID           string `gorm:"index"`
	ServerName         string
	ArchiveEventEvents []ArchiveEventEvent `gorm:"foreignKey:UUID"`
}

// Every successful ArchiveEventEvent will come from a message
type ArchiveEventEvent struct {
	CreatedAt      time.Time
	UUID           string `gorm:"primaryKey;uniqueIndex"`
	AuthorId       string
	AuthorUsername string
	ChannelId      string
	MessageId      string
	ServerID       string `gorm:"index"`
	ServerName     string
	ArchiveEvents  []ArchiveEvent `gorm:"foreignKey:ArchiveEventEventUUID"`
}

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
	AdminIds   []string `env:"ADMINISTRATOR_IDS"`
	DBHost     string   `env:"DB_HOST"`
	DBName     string   `env:"DB_NAME"`
	DBPassword string   `env:"DB_PASSWORD"`
	DBUser     string   `env:"DB_USER"`
	LogLevel   string   `env:"LOG_LEVEL"`
	Token      string   `env:"TOKEN"`
	Cookie     string   `env:"COOKIE"`
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

// Stats
// botStats is read by structToPrettyDiscordFields and converted
// into a slice of *discordgo.MessageEmbedField
type botStats struct {
	ArchiveRequests   int64  `pretty:"Times the bot has been called"`
	MessagesSent      int64  `pretty:"Messages Sent"`
	CallsToArchiveOrg int64  `pretty:"Calls to Archive.org"`
	URLsArchived      int64  `pretty:"URLs Archived"`
	Interactions      int64  `pretty:"Interactions with the bot"`
	TopDomains        string `pretty:"Top 5 Domains" inline:"false"`
	ServersActive     int64  `pretty:"Active servers"`
	ServersConfigured int64  `pretty:"Configured servers" global:"true"`
}

// domainStats is a slice of simple objects that specify a domain name
// and a count, for use in stats commands to determine most
// popular domains
type domainStats []struct {
	RequestDomainName string
	Count             int
}
