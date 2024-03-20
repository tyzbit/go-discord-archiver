package bot

import (
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
	waybackPrefix     string = "http(s)?://web.archive.org"
)

// buildMessageResponse takes a Discord session an original message and
// calls go-archiver with a []string of URLs parsed from the message.
// It then returns a slice of *discordgo.MessageSend with the resulting
// archived URLs.
func (bot *ArchiverBot) buildMessageResponse(m *discordgo.Message, newSnapshot bool) (
	messagesToSend []*discordgo.MessageSend, errs []error) {

	// If true, this is a DM
	if m.GuildID == "" {
		messagesToSend = append(messagesToSend, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{
				{Description: "Use `/archive` or the `Get snapshot` menu item on the message instead of adding a reaction."},
			},
		})
		return messagesToSend, errs
	}

	sc := bot.getServerConfig(m.GuildID)
	if sc.ArchiveEnabled.Valid && !sc.ArchiveEnabled.Bool {
		log.Info("URLs were not archived because automatic archive is not enabled")
		return messagesToSend, errs
	}

	// Do a lookup for the full guild object
	guild, gErr := bot.DG.Guild(m.GuildID)
	if gErr != nil {
		return messagesToSend, []error{fmt.Errorf("unable to look up server by id: %v", m.GuildID)}
	}

	var messageUrls []string
	// The embed's URL is the original request, so we can just yoink it from there
	if m.Embeds != nil {
		messageUrls = append(messageUrls, m.Embeds[0].URL)
	} else {
		var previousMessageUrl string
		message, err := bot.DG.ChannelMessage(m.ChannelID, m.ID)
		if err != nil {

			return messagesToSend, []error{fmt.Errorf("unable to look up message by id: %v", m.ID)}
		}
		previousMessageUrl = message.Content

		archiveRegex := regexp.MustCompile(waybackPrefix)
		if archiveRegex.MatchString(previousMessageUrl) {
			// If this was originally an already-sent embed, we have to get the
			// original URL back. Luckily the real actual URL is regexable
			// with originalURLSearch
			re := regexp.MustCompile(originalURLSearch)
			originalUrl := re.ReplaceAllString(previousMessageUrl, "$1")
			// If the URL still has 'web.archive.org', then we failed to ge the original URL
			match, _ := regexp.MatchString(archiveDomain, originalUrl)
			if match {
				log.Error("failed to get original URL from previous archive.org link")

				return messagesToSend, errs
			}
			// The suffix turned out to be a real URL
			messageUrls = []string{originalUrl}
		} else {
			messageUrls, errs = bot.extractMessageUrls(previousMessageUrl)
			for index, url := range messageUrls {
				// The original URL might not have a slash, but
				// both the response from Archive.org and
				// xurlxStrict keep the slash, so for consistency,
				// we add one if it wasn't there already.
				// Don't think about how this might break sites
				if !strings.HasSuffix(url, "/") {
					messageUrls[index] = url + "/"
				}
			}
			for index, err := range errs {
				if err != nil {
					log.Errorf("unable to extract message url: %s, err: %s", messageUrls[index], err)
				}
			}
		}
	}

	archives, errs := bot.populateArchiveEventCache(messageUrls, newSnapshot, *guild)
	for _, err := range errs {
		if err != nil {
			log.Errorf("error populating archive cache: %s", err)
		}
	}

	archivedLinks, errs := bot.executeArchiveEventRequest(&archives, sc, newSnapshot)
	for _, err := range errs {
		if err != nil {
			log.Errorf("error populating archive cache: %s", err)
			archivedLinks = append(archivedLinks, fmt.Sprintf("Error: %+v", err))
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

	messagesToSend, errs = bot.buildArchiveReply(archivedLinks, messageUrls, sc, false)

	// Interactions don't populate the Author field, so we add that manually
	if m.Author == nil {
		if m.Member != nil {
			m.Author = &discordgo.User{
				ID:       m.Member.User.ID,
				Username: m.Member.User.Username,
			}
		} else {
			log.Errorf("missing user information, message: %+v", m)
			return messagesToSend, errs
		}
	}

	for index := range archivedLinks {
		if len(archives) == index+1 {
			archives[index].ResponseURL = archivedLinks[index]
		}
	}

	// Create a call to Archiver API event
	tx := bot.DB.Create(&archives)

	if tx.RowsAffected != 1 {
		errs = append(errs, fmt.Errorf("unexpected number of rows affected inserting archive event: %v", tx.RowsAffected))
	}

	return messagesToSend, errs
}

// buildInteractionResponse takes a discordgo.InteractionCreate and
// calls go-archiver with a []string of URLs parsed from the message.
// It then returns a slice of *discordgo.MessageSend with the resulting
// archived URLs.
func (bot *ArchiverBot) buildInteractionResponse(i *discordgo.InteractionCreate, newSnapshot bool) (
	messagesToSend []*discordgo.MessageSend, errs []error) {

	var archives []ArchiveEvent
	commandData := i.Interaction.ApplicationCommandData()
	var messageUrls []string

	// The message content is in different places depending on
	// how the bot was called
	if commandData.Name == globals.Archive {
		for _, command := range commandData.Options {
			if command.Name == globals.UrlOption {
				messageUrls, errs = bot.extractMessageUrls(command.StringValue())
			}
			if command.Name == globals.TakeNewSnapshotOption {
				newSnapshot = command.BoolValue()
			}
		}
	} else if commandData.Name == globals.ArchiveMessage || commandData.Name == globals.ArchiveMessageNewSnapshot {
		for _, message := range commandData.Resolved.Messages {
			urlGroup, urlErrs := bot.extractMessageUrls(message.Content)
			messageUrls = append(messageUrls, urlGroup...)
			errs = append(errs, urlErrs...)
		}
	} else {
		log.Errorf("unexpected command name: %s", commandData.Name)
	}

	for index, err := range errs {
		if err != nil {
			log.Errorf("unable to extract message url: %s, err: %s", messageUrls[index], err)
		}
	}

	guild, err := bot.DG.Guild(i.Interaction.GuildID)
	if err != nil {
		guild.Name = "GuildLookupError"
	}
	archives, errs = bot.populateArchiveEventCache(messageUrls, false, *guild)
	for _, err := range errs {
		if err != nil {
			log.Error("error populating archive cache: ", err)
		}
	}

	sc := bot.getServerConfig(i.GuildID)

	archivedLinks, errs := bot.executeArchiveEventRequest(&archives, sc, newSnapshot)
	for _, err := range errs {
		if err != nil {
			log.Error("error populating archive cache: ", err)
			archivedLinks = append(archivedLinks, fmt.Sprintf("Error: %+v", err))
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

	messagesToSend, errs = bot.buildArchiveReply(archivedLinks, messageUrls, sc, true)

	for _, err := range errs {
		if err != nil {
			log.Error("error building archive reply: ", err)
		}
	}

	// Don't create an event if there were no archives
	if len(archives) > 0 {
		// Create a call to Archiver API event
		tx := bot.DB.Create(&archives)

		if tx.RowsAffected != 1 {
			errs = append(errs, fmt.Errorf("unexpected number of rows affected inserting archive event: %v", tx.RowsAffected))
		}
	}

	return messagesToSend, errs
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
		} else {
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
	}

	return archives, errs
}

// executeArchiveRequest takes a slice of ArchiveEvents and returns a slice of strings of successfully archived links
func (bot *ArchiverBot) executeArchiveEventRequest(archiveEvents *[]ArchiveEvent, sc ServerConfig, newSnapshot bool) (archivedLinks []string, errs []error) {
	for i, archive := range *archiveEvents {
		if archive.ResponseURL == "" || newSnapshot {
			var url string
			log.Debug("need to call archive.org api for ", archive.RequestURL)

			// This will always try to archive the page if not found
			url, err := goarchive.GetLatestURL(archive.RequestURL, uint(sc.RetryAttempts.Int32),
				sc.AlwaysArchiveFirst.Bool || newSnapshot, bot.Config.Cookie)
			if err != nil {
				return archivedLinks, []error{err}
			}

			if url != "" {
				domainName, err := getDomainName(url)
				if err != nil {
					log.Errorf("unable to get domain name for url: %v", archive.ResponseURL)
				}
				(*archiveEvents)[i].ResponseDomainName = domainName
				(*archiveEvents)[i].ResponseURL = url
			} else {
				log.Info("could not get latest archive.org url for url: ", url)
				continue
			}
			if strings.HasPrefix(url, "http://") {
				newUrl := strings.SplitN(url, "http://", 2)[1]
				url = "https://" + newUrl
			}
			archivedLinks = append(archivedLinks, url)
		} else {
			// We have a response URL, so add that to the links to be used
			// in the message
			archivedLinks = append(archivedLinks, archive.ResponseURL)
		}
	}
	return archivedLinks, errs
}

// executeArchiveRequest takes a slice of archive links and returns a slice of
// messages to send
func (bot *ArchiverBot) buildArchiveReply(archivedLinks []string, messageUrls []string, sc ServerConfig, ephemeral bool) (messagesToSend []*discordgo.MessageSend, errs []error) {
	var embeds []*discordgo.MessageEmbed
	var components []discordgo.MessageComponent

	if !ephemeral {
		components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Request new snapshot",
						Style:    discordgo.PrimaryButton,
						CustomID: globals.Retry},
				},
			},
		}
	}

	for i := 0; i < len(archivedLinks); i++ {
		originalUrl := messageUrls[i]
		link := archivedLinks[i]

		embed := discordgo.MessageEmbed{
			Title:       "🏛️ Archive.org Snapshot",
			Description: link,
			Color:       globals.FrenchGray,
		}

		sparkline, err := goarchive.CheckArchiveSparkline(originalUrl)
		if err != nil {
			log.Errorf("unable to get sparkline for url: %v", originalUrl)
			embed.Fields = []*discordgo.MessageEmbedField{{
				Name:  "Details",
				Value: "Snapshot details are not currently available, most of the time this is because the link was just archived.",
			}}
		} else {
			// If there was an error, the extra fields won't be useful anyway
			if sparkline.FirstTs != "" && sparkline.LastTs != "" && sparkline.Years != nil {
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
				}
			}
		}
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: "⚙️ Customize this message with /settings",
		}

		embed.URL = originalUrl

		embeds = append(embeds, &embed)
	}

	reply := &discordgo.MessageSend{
		Embeds:     embeds,
		Components: components,
	}

	messagesToSend = append(messagesToSend, reply)
	return messagesToSend, errs
}
