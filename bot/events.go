package bot

import (
	"time"

	"github.com/google/uuid"
)

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

// createMessageEvent logs a given message event into the database
func (bot *ArchiverBot) createMessageEvent(m MessageEvent) {
	m.UUID = uuid.New().String()
	bot.DB.Create(&m)
}

// createInteractionEvent logs a given message event into the database
func (bot *ArchiverBot) createInteractionEvent(i InteractionEvent) {
	i.UUID = uuid.New().String()
	bot.DB.Create(&i)
}
