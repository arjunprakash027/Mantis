package streamer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/arjunprakash027/Mantis/market"
	"github.com/redis/go-redis/v9"
)

type RouterMsg struct {
	AssetID string `json:"asset_id"`
}

func RegisterMetadata(ctx context.Context, rdb *redis.Client, slug string, tokens []market.Token) error {
	pipe := rdb.Pipeline()

	// Direct Index: Slug -> List of Asset IDs
	slugKey := fmt.Sprintf("slug:assets:%s", slug)

	for _, t := range tokens {
		// 1. Static Metadata
		key := fmt.Sprintf("token:meta:%s", t.TokenID)
		pipe.HSet(ctx, key, map[string]interface{}{
			"id":      t.TokenID,
			"outcome": t.Outcome,
			"market":  t.Market,
			"slug":    slug,
		})

		// 2. Index the ID under this slug
		pipe.SAdd(ctx, slugKey, t.TokenID)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func ProcessStream(ctx context.Context, rdb *redis.Client, namespace string, msgChan <-chan []byte) {
	for rawMsg := range msgChan {
		// If namespace is discovery, we don't route by AssetID
		if namespace == "discovery" {
			PushToStream(ctx, rdb, namespace, "all", rawMsg)
			continue
		}

		// Standard Per-Asset Routing (for orderbook, trades, etc.)
		if len(rawMsg) > 0 && rawMsg[0] == '[' {
			var batch []RouterMsg
			if err := json.Unmarshal(rawMsg, &batch); err == nil {
				for _, m := range batch {
					PushToStream(ctx, rdb, namespace, m.AssetID, rawMsg)
				}
			}
		} else {
			var m RouterMsg
			if err := json.Unmarshal(rawMsg, &m); err == nil {
				PushToStream(ctx, rdb, namespace, m.AssetID, rawMsg)
			}
		}
	}
}

func PushToStream(ctx context.Context, rdb *redis.Client, namespace string, identifier string, data []byte) {
	if identifier == "" {
		return
	}

	// Format: <namespace>:stream:<identifier> (e.g., market:stream:0x123)
	streamKey := fmt.Sprintf("%s:stream:%s", namespace, identifier)

	err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: 1000,
		Approx: true,
		Values: map[string]interface{}{"data": data},
	}).Err()

	if err != nil {
		log.Printf("Redis Stream Error [%s]: %v", streamKey, err)
	}
}
