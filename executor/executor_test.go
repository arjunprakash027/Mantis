package executor

import (
	"context"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/arjunprakash027/Mantis/pkg/backend"
	"github.com/arjunprakash027/Mantis/streamer"
	"github.com/arjunprakash027/Mantis/market"
	"github.com/redis/go-redis/v9"
)

// no operation streamer for benchmarking
type NoOpStreamer struct{}

func (n *NoOpStreamer) PublishStream(ctx context.Context, namespace string, identifier string, data []byte) error {
	return nil // No-Op
}

func (n *NoOpStreamer) RegisterMetadata(ctx context.Context, slug string, tokens []market.Token) error {
	return nil // No-Op
}

// no operation executor for benchmarking
type NoOpExecutor struct{}

func (n *NoOpExecutor) ExecuteTrade(ctx context.Context, action string, asset string, amount float64, price float64, strategyID string) (bool, string, error) {
	return true, "", nil
}

func (n *NoOpExecutor) PublishSignalResult(ctx context.Context, strategyID string, resultPayload []byte) error {
	return nil
}

func (n *NoOpExecutor) SubscribeInboundSignals(ctx context.Context, handler func(msgID string, payload []byte)) error {
	return nil
}

func (n *NoOpExecutor) AcknowledgeSignal(ctx context.Context, msgID string) error {
	return nil
}


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
	provider := backend.NewRedisProvider(rdb)
	engine := streamer.NewEngine(ctx, provider)
	exec := NewExecutor(ctx, provider, engine)

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

	msg := streams[0].Messages[0]
	exec.processSignalPayload(msg.ID, []byte(msg.Values["data"].(string)))

	balance, _ := rdb.HGet(ctx, "portfolio:balance", "USD").Float64()
	if balance != 5.00 {
		t.Errorf("Expected balance 5.00, got %.2f", balance)
	}
}

func TestInsufficientFunds(t *testing.T) {
	rdb.FlushAll(ctx)
	provider := backend.NewRedisProvider(rdb)
	engine := streamer.NewEngine(ctx, provider)
	exec := NewExecutor(ctx, provider, engine)

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

	msg := streams[0].Messages[0]
	exec.processSignalPayload(msg.ID, []byte(msg.Values["data"].(string)))

	balance, _ := rdb.HGet(ctx, "portfolio:balance", "USD").Float64()
	if balance != 1.00 {
		t.Errorf("Balance changed despite insufficient funds: got %.2f", balance)
	}
}

func TestAssetNotStreamed(t *testing.T) {
	rdb.FlushAll(ctx)
	provider := backend.NewRedisProvider(rdb)
	engine := streamer.NewEngine(ctx, provider)
	exec := NewExecutor(ctx, provider, engine)

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

	msg := streams[0].Messages[0]
	exec.processSignalPayload(msg.ID, []byte(msg.Values["data"].(string)))

	balance, _ := rdb.HGet(ctx, "portfolio:balance", "USD").Float64()
	if balance != 100.00 {
		t.Errorf("Trade processed for unknown asset")
	}
}

func BenchmarkProcessSignalWithBackend(b *testing.B) {
	log.SetOutput(io.Discard)
	rdb.FlushAll(ctx)
	provider := backend.NewRedisProvider(rdb)
	engine := streamer.NewEngine(ctx, provider)
	exec := NewExecutor(ctx, provider, engine)

	priceChan := make(chan []byte, 1)
	priceChan <- []byte(`{"asset_id":"Asset_123","bids":[{"price":"0.48"}],"asks":[{"price":"0.50"}]}`)
	close(priceChan)

	go engine.ProcessStream("orderbook", priceChan)

	rdb.HSet(ctx, "portfolio:balance", "USD", 1e18)
	rdb.HSet(ctx, "token:meta:Asset_123", map[string]interface{}{
		"market":  "Bitcoin Moon",
		"outcome": "Yes",
	})

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

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := streams[0].Messages[0]
		exec.processSignalPayload(msg.ID, []byte(msg.Values["data"].(string)))
	}

}

func BenchmarkProcessSignaNoBackend(b *testing.B) {
	log.SetOutput(io.Discard)
	rdb.FlushAll(ctx)
	provider := &NoOpStreamer{}
	engine := streamer.NewEngine(ctx, provider)

	executorBackend := &NoOpExecutor{}
	exec := NewExecutor(context.Background(), executorBackend, engine)

	priceChan := make(chan []byte, 1)
	priceChan <- []byte(`{"asset_id":"Asset_123","bids":[{"price":"0.48"}],"asks":[{"price":"0.50"}]}`)
	close(priceChan)

	go engine.ProcessStream("orderbook", priceChan)

	rdb.HSet(ctx, "portfolio:balance", "USD", 1e18)
	rdb.HSet(ctx, "token:meta:Asset_123", map[string]interface{}{
		"market":  "Bitcoin Moon",
		"outcome": "Yes",
	})

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

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := streams[0].Messages[0]
		exec.processSignalPayload(msg.ID, []byte(msg.Values["data"].(string)))
	}

}