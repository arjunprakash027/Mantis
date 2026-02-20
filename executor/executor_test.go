package executor

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/arjunprakash027/Mantis/streamer"
	"github.com/redis/go-redis/v9"
)

var (
	rdb *redis.Client
	ctx = context.Background()
)

func TestMain(m *testing.M) {
	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	rdb = redis.NewClient(&redis.Options{Addr: s.Addr()})

	code := m.Run()
	s.Close()
	os.Exit(code)
}

func TestAtomicTrade(t *testing.T) {
	rdb.FlushAll(ctx)
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

func TestInsufficientFunds(t *testing.T) {
	rdb.FlushAll(ctx)
	engine := streamer.NewEngine(ctx, rdb)
	exec := NewExecutor(ctx, rdb, engine)

	priceChan := make(chan []byte, 1)
	go engine.ProcessStream("orderbook", priceChan)

	rdb.HSet(ctx, "portfolio:balance", "USD", 1.00)
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
	if balance != 1.00 {
		t.Errorf("Balance changed despite insufficient funds: got %.2f", balance)
	}
}

func TestAssetNotStreamed(t *testing.T) {
	rdb.FlushAll(ctx)
	engine := streamer.NewEngine(ctx, rdb)
	exec := NewExecutor(ctx, rdb, engine)

	rdb.HSet(ctx, "portfolio:balance", "USD", 100.00)

	rdb.XGroupCreateMkStream(ctx, "signals:inbound", "mantis_executors", "$")
	rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "signals:inbound",
		Values: map[string]interface{}{
			"data": `{"action":"BUY", "asset":"Unknown_Asset", "amount": 1.0}`,
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
	if balance != 100.00 {
		t.Errorf("Trade processed for unknown asset")
	}
}
