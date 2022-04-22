package bot

import (
	"time"

	"github.com/bwmarrin/discordgo"
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
	Command            string
	ChannelId          string
	ServerID           string
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
func (bot *ArchiverBot) createMessageEvent(c string, m *discordgo.Message) {
	uuid := uuid.New().String()
	bot.DB.Create(&MessageEvent{
		UUID:           uuid,
		AuthorId:       m.Author.ID,
		AuthorUsername: m.Author.Username,
		MessageId:      m.ID,
		Command:        c,
		ChannelId:      m.ChannelID,
		ServerID:       m.GuildID,
	})
}
