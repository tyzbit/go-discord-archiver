package bot

import (
	"fmt"
)

type botStats struct {
	ArchiveRequests   int64  `pretty:"Times the bot has been called"`
	MessagesSent      int64  `pretty:"Messages Sent"`
	CallsToArchiveOrg int64  `pretty:"Calls to Archive.org"`
	URLsArchived      int64  `pretty:"URLs Archived"`
	Interactions      int64  `pretty:"Interactions with the bot"`
	TopDomains        string `pretty:"Top 5 Domains" inline:"false"`
	ServersWatched    int64  `pretty:"Servers Watched"`
}

type domainStats struct {
	RequestDomainName string
	Count             int
}

// getGlobalStats calls the database to get global stats for the bot.
// The output here is not appropriate to send to individual servers, except
// for ServersWatched.
func (bot *ArchiverBot) getGlobalStats() botStats {
	var ArchiveRequests, MessagesSent, CallsToArchiveOrg, Interactions, ServersWatched int64
	serverId := bot.DG.State.User.ID
	botId := bot.DG.State.User.ID
	archiveRows := []ArchiveEventEvent{}
	var topDomains []domainStats

	bot.DB.Model(&MessageEvent{}).Not(&MessageEvent{AuthorId: botId}).Count(&ArchiveRequests)
	bot.DB.Model(&MessageEvent{}).Where(&MessageEvent{AuthorId: serverId}).Count(&MessagesSent)
	bot.DB.Model(&InteractionEvent{}).Where(&InteractionEvent{}).Count(&Interactions)
	bot.DB.Model(&ArchiveEvent{}).Where(&ArchiveEvent{Cached: false}).Count(&CallsToArchiveOrg)
	bot.DB.Model(&ArchiveEvent{}).Scan(&archiveRows)
	bot.DB.Model(&ArchiveEvent{}).Select("request_domain_name, count(request_domain_name) as count").
		Group("request_domain_name").Order("count DESC").Find(&topDomains)
	bot.DB.Model(&ServerRegistration{}).Where(&ServerRegistration{}).Count(&ServersWatched)

	var topDomainsFormatted string
	for i := 0; i < 5 && i < len(topDomains); i++ {
		topDomainsFormatted = topDomainsFormatted + topDomains[i].RequestDomainName + ": " +
			fmt.Sprintf("%v", topDomains[i].Count) + "\n"
	}

	if topDomainsFormatted == "" {
		topDomainsFormatted = "none"
	}

	return botStats{
		ArchiveRequests:   ArchiveRequests,
		MessagesSent:      MessagesSent,
		CallsToArchiveOrg: CallsToArchiveOrg,
		URLsArchived:      int64(len(archiveRows)),
		Interactions:      Interactions,
		TopDomains:        topDomainsFormatted,
		ServersWatched:    ServersWatched,
	}
}

// getServerStats gets the stats for a particular server with ID serverId.
// If you want global stats, use getGlobalStats()
func (bot *ArchiverBot) getServerStats(serverId string) botStats {
	var ArchiveRequests, MessagesSent, CallsToArchiveOrg, Interactions, ServersWatched int64
	botId := bot.DG.State.User.ID
	archiveRows := []ArchiveEventEvent{}
	var topDomains []domainStats

	bot.DB.Model(&MessageEvent{}).Where(&MessageEvent{ServerID: serverId}).
		Not(&MessageEvent{AuthorId: botId}).Count(&ArchiveRequests)
	bot.DB.Model(&MessageEvent{}).Where(&MessageEvent{ServerID: serverId, AuthorId: botId}).Count(&MessagesSent)
	bot.DB.Model(&InteractionEvent{}).Where(&InteractionEvent{ServerID: serverId}).Count(&Interactions)
	bot.DB.Model(&ArchiveEvent{}).Where(&ArchiveEvent{ServerID: serverId, Cached: false}).Count(&CallsToArchiveOrg)
	bot.DB.Model(&ArchiveEvent{}).Where(&ArchiveEvent{ServerID: serverId}).Scan(&archiveRows)
	bot.DB.Model(&ArchiveEvent{}).Where(&ArchiveEvent{ServerID: serverId}).
		Select("request_domain_name, count(request_domain_name) as count").Order("count DESC").
		Group("request_domain_name").Find(&topDomains)
	bot.DB.Model(&ServerRegistration{}).Where(&ServerRegistration{}).Count(&ServersWatched)

	var topDomainsFormatted string
	for i := 0; i < 5 && i < len(topDomains); i++ {
		topDomainsFormatted = topDomainsFormatted + topDomains[i].RequestDomainName + ": " +
			fmt.Sprintf("%v", topDomains[i].Count) + "\n"
	}

	if topDomainsFormatted == "" {
		topDomainsFormatted = "none"
	}

	return botStats{
		ArchiveRequests:   ArchiveRequests,
		MessagesSent:      MessagesSent,
		CallsToArchiveOrg: CallsToArchiveOrg,
		URLsArchived:      int64(len(archiveRows)),
		Interactions:      Interactions,
		TopDomains:        topDomainsFormatted,
		ServersWatched:    ServersWatched,
	}
}
