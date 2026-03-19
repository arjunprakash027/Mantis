package redismantis

import "fmt"

const (
	StreamDiscovery = "discovery:stream:all"
	StreamSignalsInbound  = "signals:inbound"
	StreamSignalsOutbound = "signals:outbound"
	HashPortfolioBalance = "portfolio:balance"
	HashTradeLog = "trade:log"
	GroupMantisExecutors = "mantis_executors"
	ConsumerWorker1 = "worker_1"
)

func HashTokenMeta(id string) string {
	return fmt.Sprintf("token:meta:%s", id)
}

func StreamOrderbook(assetID string) string {
	return fmt.Sprintf("orderbook:stream:%s", assetID)
}

func SetSlugAssets(slug string) string {
	return fmt.Sprintf("slug:assets:%s", slug)
}

func StreamNamespaceDynamic(namespace string, identifier string) string {
	return fmt.Sprintf("%s:stream:%s", namespace, identifier)
}