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

// handleArchiveRequest takes a Discord session and a message string and
// calls go-archiver with a []string of URLs parsed from the message.
// It then sends an embed with the resulting archived URLs.
// TODO: break out into more functions?
func (bot *ArchiverBot) handleArchiveRequest(r *discordgo.MessageReactionAdd, newSnapshot bool) (
	replies []*discordgo.MessageSend, errs []error) {

	typingStop := make(chan bool, 1)
	go bot.typeInChannel(typingStop, r.ChannelID)

	// If true, this is a DM.
	if r.GuildID == "" {
		typingStop <- true
		replies = []*discordgo.MessageSend{
			{
				Content: "The bot must be added to a server.",
			},
		}
		return replies, errs
	}

	sc := bot.getServerConfig(r.GuildID)
	if !sc.ArchiveEnabled {
		log.Info("URLs were not archived because automatic archive is not enabled")
		typingStop <- true
		return replies, errs
	}

	// Do a lookup for the full guild object
	guild, gErr := bot.DG.Guild(r.GuildID)
	if gErr != nil {
		typingStop <- true
		return replies, []error{fmt.Errorf("unable to look up guild by id: %v", r.GuildID)}
	}

	message, err := bot.DG.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		typingStop <- true
		return replies, []error{fmt.Errorf("unable to look up message by id: %v", r.MessageID)}
	}

	var messageUrls []string
	originalUrl := message.Content
	if originalUrl == "" {
		// If this was originally an already-sent embed, we have to get the
		// original URL back. Luckily the real actual URL is regexable
		// with originalURLSearch
		re := regexp.MustCompile(originalURLSearch)
		archiveUrlSuffix := re.ReplaceAllString(message.Embeds[0].Description, "$1")
		// If the URL still has 'web.archive.org', then we failed to ge the original URL
		fail, _ := regexp.MatchString(archiveDomain, originalUrl)
		if fail {
			return replies, errs
		}

		// The suffix turned out to be a real URL
		messageUrls = []string{archiveUrlSuffix}
	} else {
		xurlsStrict := xurls.Strict
		messageUrls = xurlsStrict.FindAllString(originalUrl, -1)
	}

	if len(messageUrls) == 0 {
		errs = append(errs, fmt.Errorf("found 0 URLs in message"))
		typingStop <- true
		return replies, errs
	}

	log.Debug("URLs parsed from message: ", strings.Join(messageUrls, ", "))

	// This UUID will be used to tie together the ArchiveEventEvent,
	// the archiveRequestUrls and the archiveResponseUrls.
	archiveEventUUID := uuid.New().String()

	var archives []ArchiveEvent
	for _, url := range messageUrls {
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
		// for doing so.
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

	var archivedLinks []string
	for i, archive := range archives {
		if archive.ResponseURL == "" {
			log.Debug("need to call archive.org api for ", archive.RequestURL)

			// This will always try to archive the page if not found.
			url, err := goarchive.GetLatestURL(archive.RequestURL, sc.RetryAttempts,
				sc.AlwaysArchiveFirst || newSnapshot, bot.Config.Cookie)
			if err != nil {
				log.Errorf("error archiving url: %v", err)
				url = fmt.Sprint("%w", errors.Unwrap(err))
			}

			if url != "" {
				domainName, err := getDomainName(url)
				if err != nil {
					log.Errorf("unable to get domain name for url: %v", archive.ResponseURL)
				}
				archives[i].ResponseDomainName = domainName
			} else {
				log.Info("could not get latest archive.org url for url: ", url)
			}

			archives[i].ResponseURL = url
			archivedLinks = append(archivedLinks, url)
		} else {
			// We have a response URL, so add that to the links to be used
			// in the message.
			archivedLinks = append(archivedLinks, archive.ResponseURL)
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

	for i := 0; i < len(archivedLinks); i++ {
		originalUrl := messageUrls[i]
		link := archivedLinks[i]
		sparkline, err := goarchive.CheckArchiveSparkline(originalUrl)
		if err != nil {
			log.Errorf("unable to get sparkline for url: %v", originalUrl)
		}
		oldest, err := time.Parse(globals.ArchiveOrgTimestampLayout, sparkline.FirstTs)
		if err != nil {
			log.Errorf("unable to parse oldest timestamp for url: %v, timestamp: %v", originalUrl, sparkline.FirstTs)
		}
		newest, err := time.Parse(globals.ArchiveOrgTimestampLayout, sparkline.LastTs)
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

		embeds := []*discordgo.MessageEmbed{
			{
				Title:       "🏛️ Archive.org Snapshot",
				Description: link,
				Color:       globals.FrenchGray,
			},
		}

		components := []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Request new snapshot",
						Style:    discordgo.PrimaryButton,
						CustomID: globals.TakeCurrentSnapshot},
				},
			},
		}

		if link != "" {
			if sc.ShowDetails {
				embeds[0].Fields = []*discordgo.MessageEmbedField{
					{
						Name: "Oldest Archived Copy",
						Value: fmt.Sprintf("[%s](%s/%s/%s)",
							oldest.Format(time.RFC1123), archiveRoot, sparkline.FirstTs, originalUrl),
						Inline: true,
					},
					{
						Name: "Newest Archived Copy",
						Value: fmt.Sprintf("[%s](%s/%s/%s)",
							newest.Format(time.RFC1123), archiveRoot, sparkline.LastTs, originalUrl),
						Inline: true,
					},
					{
						Name: "Total Number of Snapshots",
						Value: fmt.Sprintf("[%s](%s/%s0000000000*/%s)",
							fmt.Sprint(snapshotCount), archiveRoot, fmt.Sprint(time.Now().Year()), originalUrl),
					},
				}
			}
			embeds[0].Footer = &discordgo.MessageEmbedFooter{
				Text: "⚙️ Customize this message with /settings",
			}
		}

		reply := &discordgo.MessageSend{
			Embeds:     embeds,
			Components: components,
		}
		replies = append(replies, reply)

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
			errs = append(errs, fmt.Errorf("unexpected number of rows affected inserting archive event: %v", tx.RowsAffected))
		}
	}

	typingStop <- true
	return replies, errs
}