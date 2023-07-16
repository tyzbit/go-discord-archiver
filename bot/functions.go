package bot

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/mvdan/xurls"
	log "github.com/sirupsen/logrus"
	goarchive "github.com/tyzbit/go-archive"
	"github.com/tyzbit/go-discord-archiver/globals"
)

const (
	archiveDomain     string = "web.archive.org"
	archiveRoot       string = "https://" + archiveDomain + "/web"
	originalURLSearch string = ".*(http.*)"
)

// handleArchiveRequest takes a Discord session and a message reaction and
// calls go-archiver with a []string of URLs parsed from the message.
// It then returns an embed with the resulting archived URLs,
func (bot *ArchiverBot) handleArchiveRequest(r *discordgo.MessageReactionAdd, newSnapshot bool) (
	embeds []*discordgo.MessageEmbed, errs []error) {

	typingStop := make(chan bool, 1)
	go bot.typeInChannel(typingStop, r.ChannelID)

	// If true, this is a DM
	if r.GuildID == "" {
		typingStop <- true
		return []*discordgo.MessageEmbed{
			{
				Description: "Use `/archive` or the `Get snapshots` menu item on the message instead of adding a reaction.",
			},
		}, errs
	}

	sc := bot.getServerConfig(r.GuildID)
	if sc.ArchiveEnabled.Valid && !sc.ArchiveEnabled.Bool {
		log.Info("URLs were not archived because automatic archive is not enabled")
		typingStop <- true
		return embeds, errs
	}

	// Do a lookup for the full guild object
	guild, gErr := bot.DG.Guild(r.GuildID)
	if gErr != nil {
		typingStop <- true
		return embeds, []error{fmt.Errorf("unable to look up server by id: %v", r.GuildID)}
	}

	message, err := bot.DG.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		typingStop <- true
		return embeds, []error{fmt.Errorf("unable to look up message by id: %v", r.MessageID)}
	}

	originalUrl := message.Content
	var messageUrls []string
	if originalUrl == "" {
		// If this was originally an already-sent embed, we have to get the
		// original URL back. Luckily the real actual URL is regexable
		// with originalURLSearch
		re := regexp.MustCompile(originalURLSearch)
		archiveUrlSuffix := re.ReplaceAllString(message.Embeds[0].Description, "$1")
		// If the URL still has 'web.archive.org', then we failed to ge the original URL
		fail, _ := regexp.MatchString(archiveDomain, originalUrl)
		if fail {
			return embeds, errs
		}

		// The suffix turned out to be a real URL
		messageUrls = []string{archiveUrlSuffix}
	} else {
		messageUrls, errs = bot.extractMessageUrls(message.Content)
		for index, err := range errs {
			if err != nil {
				log.Errorf("unable to extract message url: %s, err: %w", messageUrls[index], err)
			}
		}
	}

	archives, errs := bot.populateArchiveEventCache(messageUrls, newSnapshot, *guild)
	for _, err := range errs {
		if err != nil {
			log.Errorf("error populating archive cache: %w", err)
		}
	}

	archivedLinks, errs := bot.executeArchiveEventRequest(&archives, sc, newSnapshot)
	for _, err := range errs {
		if err != nil {
			log.Errorf("error populating archive cache: %w", err)
		}
	}

	if len(archivedLinks) < len(messageUrls) {
		log.Errorf("did not receive the same number of archived links as submitted URLs")
		if len(archivedLinks) == 0 {
			log.Errorf("did not receive any Archive.org links")
			archivedLinks = []string{"I was unable to get any Wayback Machine URLs. " +
				"Most of the time, this is " +
				"due to rate-limiting by Archive.org. " +
				"Please try again"}
		}
	}

	embeds, errs = bot.buildArchiveReply(archivedLinks, messageUrls, sc)

	// Create a call to Archiver API event
	tx := bot.DB.Create(&ArchiveEventEvent{
		UUID:           archives[0].ArchiveEventEventUUID,
		AuthorId:       message.Author.ID,
		AuthorUsername: message.Author.Username,
		ChannelId:      message.ChannelID,
		MessageId:      message.ID,
		ServerID:       guild.ID,
		ArchiveEvents:  archives,
	})

	if tx.RowsAffected != 1 {
		errs = append(errs, fmt.Errorf("unexpected number of rows affected inserting archive event: %v", tx.RowsAffected))
	}

	typingStop <- true
	return embeds, errs
}

// handleArchiveCommand takes a discordgo.InteractionCreate and
// calls go-archiver with a []string of URLs parsed from the message.
// It then returns an embed with the resulting archived URLs,
func (bot *ArchiverBot) handleArchiveCommand(i *discordgo.InteractionCreate) (
	embeds []*discordgo.MessageEmbed, errs []error) {

	message := &discordgo.Message{}
	var archives []ArchiveEvent
	commandInput := i.Interaction.ApplicationCommandData()
	if len(commandInput.Options) > 1 {
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title: "Too many options submitted",
		})
	} else {
		messageUrls, errs := bot.extractMessageUrls(commandInput.Options[0].StringValue())
		for index, err := range errs {
			if err != nil {
				log.Errorf("unable to extract message url: %s, err: %w", messageUrls[index], err)
			}
		}

		archives, errs := bot.populateArchiveEventCache(messageUrls, true, discordgo.Guild{ID: "", Name: "ArchiveCommand"})
		for _, err := range errs {
			if err != nil {
				log.Error("error populating archive cache: ", err)
			}
		}

		sc := bot.getServerConfig(i.GuildID)

		archivedLinks, errs := bot.executeArchiveEventRequest(&archives, sc,
			true)
		for _, err := range errs {
			if err != nil {
				log.Error("error populating archive cache: ", err)
			}
		}

		if len(archivedLinks) < len(messageUrls) {
			log.Errorf("did not receive the same number of archived links as submitted URLs")
			if len(archivedLinks) == 0 {
				log.Errorf("did not receive any Archive.org links")
				archivedLinks = []string{"I was unable to get any Wayback Machine URLs. " +
					"Most of the time, this is " +
					"due to rate-limiting by Archive.org. " +
					"Please try again"}
			}
		}

		embeds, errs = bot.buildArchiveReply(archivedLinks, messageUrls, sc)

		for _, err := range errs {
			if err != nil {
				log.Error("error building archive reply: ", err)
			}
		}
	}

	// Don't create an event if there were no archives
	if len(archives) > 0 {
		// Create a call to Archiver API event
		tx := bot.DB.Create(&ArchiveEventEvent{
			UUID:           archives[0].ArchiveEventEventUUID,
			AuthorId:       message.Author.ID,
			AuthorUsername: message.Author.Username,
			ChannelId:      message.ChannelID,
			MessageId:      message.ID,
			ServerID:       "DirectMessage",
			ArchiveEvents:  archives,
		})

		if tx.RowsAffected != 1 {
			errs = append(errs, fmt.Errorf("unexpected number of rows affected inserting archive event: %v", tx.RowsAffected))
		}
	}

	return embeds, errs
}

// handleArchiveMessage takes a discordgo.InteractionCreate and
// calls go-archiver with a []string of URLs parsed from the message.
// It then returns an embed with the resulting archived URLs,
func (bot *ArchiverBot) handleArchiveMessage(i *discordgo.InteractionCreate) (
	embeds []*discordgo.MessageEmbed, errs []error) {

	message := &discordgo.Message{}
	var archives []ArchiveEvent
	commandData := i.Interaction.ApplicationCommandData()
	if len(commandData.Options) > 1 {
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title: "Too many options submitted",
		})
	} else {
		for _, message := range commandData.Resolved.Messages {
			messageUrls, errs := bot.extractMessageUrls(message.Content)
			for index, err := range errs {
				if err != nil {
					log.Errorf("unable to extract message url: %s, err: %w", messageUrls[index], err)
				}
			}

			archives, errs := bot.populateArchiveEventCache(messageUrls, true, discordgo.Guild{ID: "", Name: "ArchiveCommand"})
			for _, err := range errs {
				if err != nil {
					log.Error("error populating archive cache: ", err)
				}
			}

			sc := bot.getServerConfig(i.GuildID)

			archivedLinks, errs := bot.executeArchiveEventRequest(&archives, sc,
				true)
			for _, err := range errs {
				if err != nil {
					log.Error("error populating archive cache: ", err)
				}
			}

			if len(archivedLinks) < len(messageUrls) {
				log.Errorf("did not receive the same number of archived links as submitted URLs")
				if len(archivedLinks) == 0 {
					log.Errorf("did not receive any Archive.org links")
					archivedLinks = []string{"I was unable to get any Wayback Machine URLs. " +
						"Most of the time, this is " +
						"due to rate-limiting by Archive.org. " +
						"Please try again"}
				}
			}

			embeds, errs = bot.buildArchiveReply(archivedLinks, messageUrls, sc)

			for _, err := range errs {
				if err != nil {
					log.Error("error building archive reply: ", err)
				}
			}
		}
	}

	// Don't create an event if there were no archives
	if len(archives) > 0 {
		// Create a call to Archiver API event
		tx := bot.DB.Create(&ArchiveEventEvent{
			UUID:           archives[0].ArchiveEventEventUUID,
			AuthorId:       message.Author.ID,
			AuthorUsername: message.Author.Username,
			ChannelId:      message.ChannelID,
			MessageId:      message.ID,
			ServerID:       "DirectMessage",
			ArchiveEvents:  archives,
		})

		if tx.RowsAffected != 1 {
			errs = append(errs, fmt.Errorf("unexpected number of rows affected inserting archive event: %v", tx.RowsAffected))
		}
	}

	return embeds, errs
}

// extractMessageUrls takes a string and returns a slice of URLs parsed from the string
func (bot *ArchiverBot) extractMessageUrls(message string) (messageUrls []string, errs []error) {
	xurlsStrict := xurls.Strict
	messageUrls = xurlsStrict.FindAllString(message, -1)

	if len(messageUrls) == 0 {
		errs = append(errs, fmt.Errorf("found 0 URLs in message"))
	}

	log.Debug("URLs parsed from message: ", strings.Join(messageUrls, ", "))
	return messageUrls, errs
}

// populateArchiveCache takes an slice of messageUrls and returns a slice of ArchiveEvents
func (bot *ArchiverBot) populateArchiveEventCache(messageUrls []string, newSnapshot bool, guild discordgo.Guild) (archives []ArchiveEvent, errs []error) {
	// This UUID will be used to tie together the ArchiveEventEvent,
	// the archiveRequestUrls and the archiveResponseUrls
	archiveEventUUID := uuid.New().String()

	for _, url := range messageUrls {
		domainName, err := getDomainName(url)
		if err != nil {
			log.Error("unable to get domain name for url: ", url)
		}

		// See if there is a response URL for a given request URL in the database
		cachedArchiveEvents := []ArchiveEvent{}
		bot.DB.Model(&ArchiveEvent{}).Where(&ArchiveEvent{RequestURL: url, Cached: false}).Find(&cachedArchiveEvents)
		var responseUrl, responseDomainName string

		// If we have a response, create a new ArchiveEvent with it,
		// marking it as cached
		for _, cachedArchiveEvent := range cachedArchiveEvents {
			if cachedArchiveEvent.ResponseURL != "" && cachedArchiveEvent.ResponseDomainName != "" {
				responseUrl = cachedArchiveEvent.ResponseURL
				responseDomainName = cachedArchiveEvent.ResponseDomainName
			}
		}

		if responseUrl != "" && responseDomainName != "" && !newSnapshot {
			log.Debug("url was already cached: ", url)
			// We have already archived this URL, so save the response
			archives = append(archives, ArchiveEvent{
				UUID:                  uuid.New().String(),
				ArchiveEventEventUUID: archiveEventUUID,
				ServerID:              guild.ID,
				ServerName:            guild.Name,
				RequestURL:            url,
				RequestDomainName:     domainName,
				ResponseURL:           responseUrl,
				ResponseDomainName:    responseDomainName,
				Cached:                true,
			})
			continue
		}

		// We have not already archived this URL, so build an object
		// for doing so
		log.Debug("url was not cached: ", url)
		archives = append(archives, ArchiveEvent{
			UUID:                  uuid.New().String(),
			ArchiveEventEventUUID: archiveEventUUID,
			ServerID:              guild.ID,
			ServerName:            guild.Name,
			RequestURL:            url,
			RequestDomainName:     domainName,
			Cached:                false,
		})
	}
	return archives, errs
}

// executeArchiveRequest takes a slice of ArchiveEvents and returns a slice of strings of successfully archived links
func (bot *ArchiverBot) executeArchiveEventRequest(archiveEvents *[]ArchiveEvent, sc ServerConfig, newSnapshot bool) (archivedLinks []string, errs []error) {
	for i, archive := range *archiveEvents {
		if archive.ResponseURL == "" {
			log.Debug("need to call archive.org api for ", archive.RequestURL)

			// This will always try to archive the page if not found
			url, err := goarchive.GetLatestURL(archive.RequestURL, uint(sc.RetryAttempts.Int32),
				sc.AlwaysArchiveFirst.Bool || newSnapshot, bot.Config.Cookie)
			if err != nil {
				log.Errorf("error archiving url: %v", err)
				url = fmt.Sprint("%w", errors.Unwrap(err))
			}

			if url != "" {
				domainName, err := getDomainName(url)
				if err != nil {
					log.Errorf("unable to get domain name for url: %v", archive.ResponseURL)
				}
				(*archiveEvents)[i].ResponseDomainName = domainName
			} else {
				log.Info("could not get latest archive.org url for url: ", url)
			}

			(*archiveEvents)[i].ResponseURL = url
			archivedLinks = append(archivedLinks, url)
		} else {
			// We have a response URL, so add that to the links to be used
			// in the message
			archivedLinks = append(archivedLinks, archive.ResponseURL)
		}
	}
	return archivedLinks, errs
}

// executeArchiveRequest takes a slice of ArchiveEvents and returns a slice of strings of successfully archived links
func (bot *ArchiverBot) buildArchiveReply(archivedLinks []string, messageUrls []string, sc ServerConfig) (embeds []*discordgo.MessageEmbed, errs []error) {
	for i := 0; i < len(archivedLinks); i++ {
		originalUrl := messageUrls[i]
		link := archivedLinks[i]
		sparkline, err := goarchive.CheckArchiveSparkline(originalUrl)
		if err != nil {
			log.Errorf("unable to get sparkline for url: %v", originalUrl)
		}
		oldest, err := time.ParseInLocation(globals.ArchiveOrgTimestampLayout, sparkline.FirstTs, time.UTC)
		if err != nil {
			log.Errorf("unable to parse oldest timestamp for url: %v, timestamp: %v", originalUrl, sparkline.FirstTs)
		}
		newest, err := time.ParseInLocation(globals.ArchiveOrgTimestampLayout, sparkline.LastTs, time.UTC)
		if err != nil {
			log.Errorf("unable to parse newest timestamp for url: %v, timestamp: %v", originalUrl, sparkline.LastTs)
		}

		snapshotCount := 0
		// Each year (_) has an array of 12 integers (correspondes to months)
		// of how many snapshots there are for that month
		for _, v := range sparkline.Years {
			for _, monthCount := range v {
				snapshotCount = snapshotCount + monthCount
			}
		}

		embed := discordgo.MessageEmbed{
			Title:       "🏛️ Archive.org Snapshot",
			Description: link,
			Color:       globals.FrenchGray,
		}

		if link != "" {
			if sc.ShowDetails.Valid && sc.ShowDetails.Bool {
				if !sc.UTCSign.Valid {
					log.Errorf("Invalid UTC setting for %s(%s)", sc.DiscordId, sc.Name)
					continue
				}
				if err != nil {
					log.Errorf("Unable to load timezone UTC%s%v", sc.UTCSign.String, sc.UTCOffset)
					continue
				}
				sign := map[string]int{
					"-": -1,
					"+": 1,
				}
				location := time.FixedZone("UTC", sign[sc.UTCSign.String]*int(sc.UTCOffset.Int32)*60*60)
				embed.Fields = []*discordgo.MessageEmbedField{
					{
						Name: "Oldest Archived Copy",
						Value: fmt.Sprintf("[%s](%s/%s/%s)",
							// oldest.In(location).Format(time.RFC1123), archiveRoot, sparkline.FirstTs, originalUrl),
							oldest.In(location).Format(time.RFC1123Z), archiveRoot, sparkline.FirstTs, originalUrl),
						Inline: true,
					},
					{
						Name: "Newest Archived Copy",
						Value: fmt.Sprintf("[%s](%s/%s/%s)",
							newest.In(location).Format(time.RFC1123Z), archiveRoot, sparkline.LastTs, originalUrl),
						Inline: true,
					},
					{
						Name: "Total Number of Snapshots",
						Value: fmt.Sprintf("[%s](%s/%s0000000000*/%s)",
							fmt.Sprint(snapshotCount), archiveRoot, fmt.Sprint(time.Now().Year()), originalUrl),
					},
				}
			}
			embed.Footer = &discordgo.MessageEmbedFooter{
				Text: "⚙️ Customize this message with /settings",
			}
		}
		embeds = append(embeds, &embed)
	}

	return embeds, errs
}
