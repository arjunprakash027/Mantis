package executor

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/arjunprakash027/Mantis/streamer"
	"github.com/redis/go-redis/v9"
)

func TestAtomicTrade(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ctx := context.Background()
	engine := streamer.NewEngine(ctx, rdb)
	exec := NewExecutor(ctx, rdb, engine)

	priceChan := make(chan []byte, 1)
	go engine.ProcessStream("orderbook", priceChan)

	rdb.HSet(ctx, "portfolio:balance", "USD", 10.00)
	rdb.HSet(ctx, "token:meta:Asset_123", map[string]interface{}{
		"market":  "Bitcoin Moon",
		"outcome": "Yes",
	})

	priceChan <- []byte(`{"asset_id":"Asset_123","bids":[{"price":"0.48"}],"asks":[{"price":"0.50"}]}`)

	time.Sleep(10 * time.Millisecond)

	rdb.XGroupCreateMkStream(ctx, "signals:inbound", "mantis_executors", "$")
	rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "signals:inbound",
		Values: map[string]interface{}{
			"data": `{"action":"BUY", "asset":"Asset_123", "amount": 10.0}`,
		},
	})

	streams, _ := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    "mantis_executors",
		Consumer: "test_worker",
		Streams:  []string{"signals:inbound", ">"},
		Count:    1,
	}).Result()

	exec.processSignal(streams[0].Messages[0])

	balance, _ := rdb.HGet(ctx, "portfolio:balance", "USD").Float64()
	if balance != 5.00 {
		t.Errorf("Expected balance 5.00, got %.2f", balance)
	}
}
