package bot

import (
	"fmt"
)

type botStats struct {
	MessagesActedOn   int64  `pretty:"Messages Acted On"`
	MessagesSent      int64  `pretty:"Messages Sent"`
	CallsToArchiveOrg int64  `pretty:"Calls to Archive.org"`
	URLsArchived      int64  `pretty:"URLs Archived"`
	TopDomains        string `pretty:"Top 5 Domains"`
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
	var MessagesActedOn, MessagesSent, CallsToArchiveOrg, ServersWatched int64
	serverId := bot.DG.State.User.ID
	archiveRows := []ArchiveEventEvent{}
	var topDomains []domainStats

	bot.DB.Model(&MessageEvent{}).Count(&MessagesActedOn)
	bot.DB.Model(&MessageEvent{}).Where(&MessageEvent{AuthorId: serverId}).Count(&MessagesSent)
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
		MessagesActedOn:   MessagesActedOn,
		MessagesSent:      MessagesSent,
		CallsToArchiveOrg: CallsToArchiveOrg,
		URLsArchived:      int64(len(archiveRows)),
		TopDomains:        topDomainsFormatted,
		ServersWatched:    ServersWatched,
	}
}

// getServerStats gets the stats for a particular server with ID serverId.
// If you want global stats, use getGlobalStats()
func (bot *ArchiverBot) getServerStats(serverId string) botStats {
	var MessagesActedOn, MessagesSent, CallsToArchiveOrg, ServersWatched int64
	botId := bot.DG.State.User.ID
	archiveRows := []ArchiveEventEvent{}
	var topDomains []domainStats

	bot.DB.Model(&MessageEvent{}).Where(&MessageEvent{ServerID: serverId}).Count(&MessagesActedOn)
	bot.DB.Model(&MessageEvent{}).Where(&MessageEvent{AuthorId: botId, ServerID: serverId}).Count(&MessagesSent)
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
		MessagesActedOn:   MessagesActedOn,
		MessagesSent:      MessagesSent,
		CallsToArchiveOrg: CallsToArchiveOrg,
		URLsArchived:      int64(len(archiveRows)),
		TopDomains:        topDomainsFormatted,
		ServersWatched:    ServersWatched,
	}
}
