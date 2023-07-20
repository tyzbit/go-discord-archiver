package bot

import (
	"database/sql"
	"fmt"
)

// getGlobalStats calls the database to get global stats for the bot
// The output here is not appropriate to send to individual servers, except
// for ServersActive
func (bot *ArchiverBot) getGlobalStats() botStats {
	var ArchiveRequests, MessagesSent, Interactions, CallsToArchiveOrg, URLsArchived, ServersConfigured, ServersActive int64
	serverId := bot.DG.State.User.ID
	botId := bot.DG.State.User.ID
	var topDomains domainStats

	bot.DB.Model(&MessageEvent{}).Not(&MessageEvent{AuthorId: botId}).Count(&ArchiveRequests)
	bot.DB.Model(&MessageEvent{}).Where(&MessageEvent{AuthorId: serverId}).Count(&MessagesSent)
	bot.DB.Model(&InteractionEvent{}).Where(&InteractionEvent{}).Count(&Interactions)
	bot.DB.Model(&ArchiveEvent{}).Where(&ArchiveEvent{Cached: false}).Count(&CallsToArchiveOrg)
	bot.DB.Model(&ArchiveEvent{}).Count(&URLsArchived)
	bot.DB.Model(&ArchiveEvent{}).Select("request_domain_name, count(request_domain_name) as count").
		Group("request_domain_name").Order("count DESC").Find(&topDomains)
	bot.DB.Model(&ServerRegistration{}).Count(&ServersConfigured)
	bot.DB.Find(&ServerRegistration{}).Where(&ServerRegistration{
		Active: sql.NullBool{Valid: true, Bool: true}}).Count(&ServersActive)

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
		URLsArchived:      URLsArchived,
		Interactions:      Interactions,
		TopDomains:        topDomainsFormatted,
		ServersConfigured: ServersConfigured,
		ServersActive:     ServersActive,
	}
}

// getServerStats gets the stats for a particular server with ID serverId
// If you want global stats, use getGlobalStats()
func (bot *ArchiverBot) getServerStats(serverId string) botStats {
	var ArchiveRequests, MessagesSent, CallsToArchiveOrg, URLsArchived, Interactions, ServersActive int64
	botId := bot.DG.State.User.ID
	var topDomains domainStats

	bot.DB.Model(&MessageEvent{}).Where(&MessageEvent{ServerID: serverId}).
		Not(&MessageEvent{AuthorId: botId}).Count(&ArchiveRequests)
	bot.DB.Model(&MessageEvent{}).Where(&MessageEvent{ServerID: serverId, AuthorId: botId}).Count(&MessagesSent)
	bot.DB.Model(&InteractionEvent{}).Where(&InteractionEvent{ServerID: serverId}).Count(&Interactions)
	bot.DB.Model(&ArchiveEvent{}).Where(&ArchiveEvent{ServerID: serverId, Cached: false}).Count(&CallsToArchiveOrg)
	bot.DB.Model(&ArchiveEvent{}).Where(&ArchiveEvent{ServerID: serverId}).Count(&ArchiveRequests)
	bot.DB.Model(&ArchiveEvent{}).Where(&ArchiveEvent{ServerID: serverId}).
		Select("request_domain_name, count(request_domain_name) as count").Order("count DESC").
		Group("request_domain_name").Find(&topDomains)
	bot.DB.Model(&ServerRegistration{}).Where(&ServerRegistration{}).Count(&ServersActive)

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
		URLsArchived:      URLsArchived,
		Interactions:      Interactions,
		TopDomains:        topDomainsFormatted,
		ServersActive:     ServersActive,
	}
}
