package bot

import (
	"time"

	"github.com/google/uuid"
)

// A MessageEvent is created when we receive a message that
// requires our attention
type MessageEvent struct {
	CreatedAt          time.Time
	UUID               string `gorm:"primaryKey"`
	AuthorId           string
	AuthorUsername     string
	MessageId          string
	ChannelId          string
	ServerID           string
	ServerName         string
	ArchiveEventEvents []ArchiveEventEvent `gorm:"foreignKey:UUID"`
}

type InteractionEvent struct {
	CreatedAt          time.Time
	UUID               string `gorm:"primaryKey"`
	UserID             string
	Username           string
	InteractionId      string
	ChannelId          string
	ServerID           string
	ServerName         string
	ArchiveEventEvents []ArchiveEventEvent `gorm:"foreignKey:UUID"`
}

// Every successful ArchiveEventEvent will come from a message.
type ArchiveEventEvent struct {
	CreatedAt      time.Time
	UUID           string `gorm:"primaryKey"`
	AuthorId       string
	AuthorUsername string
	ChannelId      string
	MessageId      string
	ServerID       string
	ServerName     string
	ArchiveEvents  []ArchiveEvent `gorm:"foreignKey:ArchiveEventEventUUID"`
}

// This is the representation of request and response URLs from users or
// the Archiver API.
type ArchiveEvent struct {
	CreatedAt             time.Time
	UUID                  string `gorm:"primaryKey"`
	ArchiveEventEventUUID string
	ServerID              string
	RequestURL            string
	RequestDomainName     string
	ResponseURL           string
	ResponseDomainName    string
	Cached                bool
}

// createMessageEvent logs a given message event into the database.
func (bot *ArchiverBot) createMessageEvent(m MessageEvent) {
	m.UUID = uuid.New().String()
	bot.DB.Create(&m)
}

// createInteractionEvent logs a given message event into the database.
func (bot *ArchiverBot) createInteractionEvent(i InteractionEvent) {
	i.UUID = uuid.New().String()
	bot.DB.Create(&i)
}
