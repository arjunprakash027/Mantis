package backend

import (
	"context"
	_ "embed"
	"log"
	"time"
	"github.com/arjunprakash027/Mantis/market"
	"github.com/arjunprakash027/Mantis/pkg/redismantis"
	"github.com/redis/go-redis/v9"
)

var _ StreamerBackend = (*RedisProvider)(nil)
var _ ExecutorBackend = (*RedisProvider)(nil)

type RedisProvider struct {
	rdb *redis.Client
}

func NewRedisProvider(rdb *redis.Client) *RedisProvider {
	return &RedisProvider{rdb: rdb}
}

//go:embed trade.lua
var tradeLua string

var tradeScript = redis.NewScript(tradeLua)

func (r *RedisProvider) PublishStream(ctx context.Context, namespace string, identifier string, data []byte) error {
	streamKey := redismantis.StreamNamespaceDynamic(namespace, identifier)

	return r.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: 1000,
		Approx: true,
		Values: map[string]interface{}{"data": data},
	}).Err()
}

func (r *RedisProvider) RegisterMetadata(ctx context.Context, slug string, tokens []market.Token) error {
	pipe := r.rdb.Pipeline()
	slugKey := redismantis.SetSlugAssets(slug)

	for _, t := range tokens {
		key := redismantis.HashTokenMeta(t.TokenID)
		pipe.HSet(ctx, key, map[string]interface{}{
			"id":      t.TokenID,
			"outcome": t.Outcome,
			"market":  t.Market,
			"slug":    slug,
		})
		pipe.SAdd(ctx, slugKey, t.TokenID)
	}
	
	_, err := pipe.Exec(ctx)
	return err

}

func (r *RedisProvider) ExecuteTrade(ctx context.Context, action string, asset string, amount float64, price float64, StrategyID string) (bool, string, error) {

	totalCost := price * amount

	res, err := tradeScript.Run(ctx, r.rdb,
		[]string{redismantis.HashPortfolioBalance, redismantis.HashTradeLog},
		action, asset, amount, price, totalCost, time.Now().Unix(), StrategyID,
	).Result()

	if err != nil {
		return false, "Internal DB Error", err
	}

	resSlice, ok := res.([]interface{})
	if !ok || len(resSlice) < 2 {
		return false, "Invalid Lua script response format", nil
	}
	
	success := resSlice[0].(int64) == 1

	var errorMsg string
	if !success {
		errorMsg = resSlice[1].(string)
	}

	return success, errorMsg, nil
}

func (r *RedisProvider) PublishSignalResult(ctx context.Context, stratergyID string, resultPayload []byte) error {
	return r.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: redismantis.StreamSignalsOutbound,
		Values: map[string]interface{}{
			"strategy_id": stratergyID,
			"data":        resultPayload,
		},
	}).Err()
}

func (r *RedisProvider) SubscribeInboundSignals(ctx context.Context, handler func(msgID string, payload []byte)) error {

	r.rdb.XGroupCreateMkStream(ctx, redismantis.StreamSignalsInbound, redismantis.GroupMantisExecutors, "$")
	for {
		streams, err := r.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    redismantis.GroupMantisExecutors,
			Consumer: redismantis.ConsumerWorker1,
			Streams:  []string{redismantis.StreamSignalsInbound, ">"},
			Count:    1,
			Block:    0,
		}).Result()

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			log.Printf("Redis Stream Error [%s]: %v", "signals:inbound", err)
			continue
		}

		for _, msg := range streams[0].Messages {
			dataStr, ok :=  msg.Values["data"].(string)
			if !ok {
				continue
			}
			handler(msg.ID, []byte(dataStr))
		}
	}

}

func (r *RedisProvider) AcknowledgeSignal(ctx context.Context, msgID string) error {
	return r.rdb.XAck(ctx, redismantis.StreamSignalsInbound, redismantis.GroupMantisExecutors, msgID).Err()
}


