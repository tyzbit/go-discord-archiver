package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/mvdan/xurls"
	log "github.com/sirupsen/logrus"
	goarchive "github.com/tyzbit/go-archive"
)

// handleMessageWithStats takes a discord session and a user ID and sends a
// message to the user with stats about the bot.
func (bot *ArchiverBot) handleMessageWithStats(s *discordgo.Session, m *discordgo.MessageCreate) error {
	directMessage := (m.GuildID == "")

	var stats botStats
	logMessage := ""
	if !directMessage {
		stats = bot.getServerStats(m.GuildID)
		guild, err := s.Guild(m.GuildID)
		if err != nil {
			return fmt.Errorf("unable to look up guild by id: %v", m.GuildID+", "+fmt.Sprintf("%v", err))
		}
		logMessage = "sending " + statsCommand + " response to " + m.Author.Username + "(" + m.Author.ID + ") in " +
			guild.Name + "(" + guild.ID + ")"
	} else {
		// We can be sure now the request was a direct message.
		// Deny by default.
		administrator := false

	out:
		for _, id := range bot.Config.AdminIds {
			if m.Author.ID == id {
				administrator = true

				// This prevents us from checking all IDs now that
				// we found a match but is a fairly ineffectual
				// optimization since config.AdminIds will probably
				// only have dozens of IDs at most.
				break out
			}
		}

		if !administrator {
			return fmt.Errorf("did not respond to %v(%v), command %v because user is not an administrator",
				m.Author.Username, m.Author.ID, statsCommand)
		}
		stats = bot.getGlobalStats()
		logMessage = "sending global " + statsCommand + " response to " + m.Author.Username + "(" + m.Author.ID + ")"
	}

	// write a new statsMessageEvent to the DB
	bot.createMessageEvent(statsCommand, m.Message)

	embed := &discordgo.MessageEmbed{
		Title:  "ArchiveEvent Stats",
		Fields: structToPrettyDiscordFields(stats),
	}

	// Respond to statsCommand command with the formatted stats embed
	log.Info(logMessage)
	bot.sendMessage(s, true, false, m.Message, embed)

	return nil
}

// handleArchiveRequest takes a Discord session and a message string and
// calls go-archiver with a []string of URLs parsed from the message.
// It then sends an embed with the resulting archived URLs.
func (bot *ArchiverBot) handleArchiveRequest(s *discordgo.Session, r *discordgo.MessageReactionAdd, rearchive bool) error {
	ServerConfig := bot.getServerConfig(r.GuildID)
	if !ServerConfig.ArchiveEnabled {
		log.Info("URLs were not archived because automatic archive is not enabled")
		return nil
	}

	// Do a lookup for the full guild object
	guild, gErr := s.Guild(r.GuildID)
	if gErr != nil {
		return fmt.Errorf("unable to look up guild by id: %v", r.GuildID)
	}

	message, err := s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return fmt.Errorf("unable to look up message by id: %v", r.MessageID)
	}
	xurlsStrict := xurls.Strict
	urls := xurlsStrict.FindAllString(message.Content, -1)
	if len(urls) == 0 {
		return fmt.Errorf("found 0 URLs in message")
	}

	log.Debug("URLs parsed from message: ", strings.Join(urls, ", "))

	// This UUID will be used to tie together the ArchiveEventEvent,
	// the archiveRequestUrls and the archiveResponseUrls.
	archiveEventUUID := uuid.New().String()

	var archives []ArchiveEvent
	for _, url := range urls {
		domainName, err := getDomainName(url)
		if err != nil {
			log.Error("unable to get domain name for url: ", url)
		}

		// See if there is a response URL for a given request URL in the database.
		cachedArchiveEvents := []ArchiveEvent{}
		bot.DB.Model(&ArchiveEvent{}).Where(&ArchiveEvent{RequestURL: url, Cached: false}).Find(&cachedArchiveEvents)
		var responseUrl, responseDomainName string

		// If we have a response, create a new ArchiveEvent with it,
		// marking it as cached.
		for _, cachedArchiveEvent := range cachedArchiveEvents {
			if cachedArchiveEvent.ResponseURL != "" && cachedArchiveEvent.ResponseDomainName != "" {
				responseUrl = cachedArchiveEvent.ResponseURL
				responseDomainName = cachedArchiveEvent.ResponseDomainName
			}
		}

		if responseUrl != "" && responseDomainName != "" {
			log.Debug("url was already cached: ", url)
			// We have already archived this URL, so save the response
			archives = append(archives, ArchiveEvent{
				UUID:                  uuid.New().String(),
				ArchiveEventEventUUID: archiveEventUUID,
				ServerID:              guild.ID,
				RequestURL:            url,
				RequestDomainName:     domainName,
				ResponseURL:           responseUrl,
				ResponseDomainName:    responseDomainName,
				Cached:                true,
			})
			continue
		}

		// We have not already archived this URL, so build an object
		// for doing so.
		log.Debug("url was not cached: ", url)
		archives = append(archives, ArchiveEvent{
			UUID:                  uuid.New().String(),
			ArchiveEventEventUUID: archiveEventUUID,
			ServerID:              guild.ID,
			RequestURL:            url,
			RequestDomainName:     domainName,
			Cached:                false,
		})
	}

	var archivedLinks []string
	for _, archive := range archives {
		if archive.ResponseURL == "" {
			log.Debug("need to call archive.org api for ", archive.RequestURL)

			urls = []string{}
			if rearchive {
				url, err := goarchive.ArchiveURL(archive.RequestURL)
				if err != nil || url == "" {
					log.Warnf("unable to refresh archived page for url: %v, err: %v", url, err)
					continue
				}
				urls = append(urls, url)
			} else {
				var errs []error
				urls, errs = goarchive.GetLatestURLs([]string{archive.RequestURL}, ServerConfig.AutoArchive)
				for _, err := range errs {
					if err != nil {
						log.Errorf("error archiving url: %v", err)
					}
				}
			}

			for i, url := range urls {
				if url == "" {
					log.Info("could not get latest archive.org url for url: ", url)
					continue
				}
				domainName, err := getDomainName(url)
				if err != nil {
					log.Errorf("unable to get domain name for url: %v", archive.ResponseURL)
				}
				archives[i].ResponseURL = url
				archives[i].ResponseDomainName = domainName
				archivedLinks = append(archivedLinks, url)
			}
		} else {
			// We have a response URL, so add that to the links to be used
			// in the message.
			archivedLinks = append(archivedLinks, archive.ResponseURL)
		}
	}

	plural := ""
	if len(archivedLinks) > 1 {
		plural = "s"
	}
	title := fmt.Sprintf("Archived Link%v", plural)

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: strings.Join(archivedLinks, "\n"),
	}

	username := ""
	user, err := s.User(r.UserID)
	if err != nil {
		log.Errorf("unable to look up user with ID %v, err: %v", r.UserID, err)
		username = "unknown"
	} else {
		username = user.Username
	}
	log.Debug("sending archive message response in ",
		guild.Name, "(", guild.ID, "), calling user: ",
		username, "(", r.UserID, ")")
	bot.sendMessage(s, ServerConfig.UseEmbed, ServerConfig.ReplyToOriginalMessage, message, embed)

	// Create a call to Archiver API event
	tx := bot.DB.Create(&ArchiveEventEvent{
		UUID:           archiveEventUUID,
		AuthorId:       message.Author.ID,
		AuthorUsername: message.Author.Username,
		ChannelId:      message.ChannelID,
		MessageId:      message.ID,
		ServerID:       guild.ID,
		ArchiveEvents:  archives,
	})

	if tx.RowsAffected != 1 {
		return fmt.Errorf("unexpected number of rows affected inserting archive event: %v", tx.RowsAffected)
	}

	return nil
}
