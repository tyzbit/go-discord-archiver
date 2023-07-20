package bot

import (
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// createMessageEvent logs a given message event into the database
func (bot *ArchiverBot) createMessageEvent(m MessageEvent) {
	m.UUID = uuid.New().String()
	tx := bot.DB.Create(&m)
	if tx.RowsAffected != 1 {
		log.Errorf("unexpected number of rows affected inserting archive event: %v", tx.RowsAffected)
	}
}

// createInteractionEvent logs a given message event into the database
func (bot *ArchiverBot) createInteractionEvent(i InteractionEvent) {
	i.UUID = uuid.New().String()
	tx := bot.DB.Create(&i)
	if tx.RowsAffected != 1 {
		log.Errorf("unexpected number of rows affected inserting archive event: %v", tx.RowsAffected)
	}
}

// createInteractionEvent logs a given message event into the database
func (bot *ArchiverBot) createArchiveEvent(i InteractionEvent) {
	i.UUID = uuid.New().String()
	tx := bot.DB.Create(&i)
	if tx.RowsAffected != 1 {
		log.Errorf("unexpected number of rows affected inserting archive event: %v", tx.RowsAffected)
	}
}
