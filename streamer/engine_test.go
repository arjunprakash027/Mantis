package streamer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/arjunprakash027/Mantis/market"
	"github.com/arjunprakash027/Mantis/pkg/backend"
	"github.com/redis/go-redis/v9"
)

type NoOpStreamer struct{}

func (n *NoOpStreamer) PublishStream(ctx context.Context, namespace string, identifier string, data []byte) error {
	return nil
}

func (n *NoOpStreamer) RegisterMetadata(ctx context.Context, slug string, tokens []market.Token) error {
	return nil
}

func TestFullSystemEndToEnd(t *testing.T) {

	s, _ := miniredis.Run()
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	provider := backend.NewRedisProvider(rdb)
	engine := NewEngine(ctx, provider)
	t.Log("System Initialized with Sandbox Redis")

	slug := "will-trump-pardon-ghislaine-maxwell"
	t.Logf("Connecting to real-world market: %s", slug)

	tokens, eventTitle, err := market.GetTokens(slug)
	if err != nil {
		t.Fatalf("LookUp Error: %v", err)
	}
	t.Logf("Discovered Market: %s", eventTitle)

	assetIds := make([]string, len(tokens))
	for i, t := range tokens {
		assetIds[i] = t.TokenID
	}

	msgChan := make(chan []byte, 100)
	err = market.StartOrderBookStream(ctx, assetIds, msgChan)
	if err != nil {
		t.Fatalf("Stream Error: %v", err)
	}

	go engine.ProcessStream("orderbook", msgChan)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	foundData := false
	for {
		select {
		case <-ctx.Done():
			if !foundData {
				t.Fatal("Timeout: Did not receive any live prices from Polymarket after 20s")
			}
			return
		case <-ticker.C:
			engine.mu.RLock()
			count := len(engine.prices)
			engine.mu.RUnlock()

			if count > 0 {
				t.Logf("Success: Engine is receiving live data for %d assets!", count)
				foundData = true
				return
			}
			t.Log("... waiting for network packets ...")
		}
	}
}

func BenchmarkUpdateCacheSingle(b *testing.B) {
	s, _ := miniredis.Run()
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	provider := backend.NewRedisProvider(rdb)
	engine := NewEngine(ctx, provider)
	
	rawMsg := []byte(`[{"asset_id":"Asset_123","bids":[{"price":"0.48","size":"100"}],"asks":[{"price":"0.50","size":"100"}]}]`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.updateCache(rawMsg)
	}
}

func BenchmarkUpdateCacheMultiple(b *testing.B) {
	s, _ := miniredis.Run()
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	provider := backend.NewRedisProvider(rdb)
	engine := NewEngine(ctx, provider)

	numAssests := 100
	var msgPool [][]byte

	for i := 0; i < numAssests; i++ {
		assetID := fmt.Sprintf("Asset_%d", i)
		msg := fmt.Sprintf(`[{"asset_id":"%s","bids":[{"price":"0.48","size":"100"}],"asks":[{"price":"0.50","size":"100"}]}]`, assetID)
		msgPool = append(msgPool, []byte(msg))
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			engine.updateCache(msgPool[idx%numAssests]) //cycling throuhg pool of 100 messages again and again
			idx ++
		}
	})
	
}

func BenchmarkGetPriceWithBackend(b *testing.B) {
	s, _ := miniredis.Run()
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	provider := backend.NewRedisProvider(rdb)
	engine := NewEngine(context.Background(), provider)
	
	rawMsg := []byte(`[{"asset_id":"Asset_123","bids":[{"price":"0.48","size":"100"}],"asks":[{"price":"0.50","size":"100"}]}]`)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			select {
			case <- ctx.Done():
				return
			default:
				engine.updateCache(rawMsg)
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = engine.GetPrice("Asset_123")
		}
	})
}

func BenchmarkPushToBackend(b *testing.B) {
	s, _ := miniredis.Run()
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	provider := backend.NewRedisProvider(rdb)
	engine := NewEngine(context.Background(), provider)
	
	rawMsg := []byte(`[{
		"asset_id": "Asset_123",
		"bids": [
			{"price":"0.48","size":"100"},{"price":"0.47","size":"200"},{"price":"0.46","size":"500"},
			{"price":"0.45","size":"100"},{"price":"0.44","size":"200"},{"price":"0.43","size":"500"},
			{"price":"0.42","size":"100"},{"price":"0.41","size":"200"},{"price":"0.40","size":"500"}
		],
		"asks": [
			{"price":"0.50","size":"100"},{"price":"0.51","size":"200"},{"price":"0.52","size":"500"},
			{"price":"0.53","size":"100"},{"price":"0.54","size":"200"},{"price":"0.55","size":"500"},
			{"price":"0.56","size":"100"},{"price":"0.57","size":"200"},{"price":"0.58","size":"500"}
		]
	}]`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.pushToRedis("orderbook", rawMsg)
	}
}

func BenchmarkProcessStreamE2EWithBackend(b *testing.B) {
	s, _ := miniredis.Run()
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	provider := backend.NewRedisProvider(rdb)
	engine := NewEngine(context.Background(), provider)
	
	rawMsg := []byte(`[{
		"asset_id": "Asset_123",
		"bids": [
			{"price":"0.48","size":"100"},{"price":"0.47","size":"200"},{"price":"0.46","size":"500"},
			{"price":"0.45","size":"100"},{"price":"0.44","size":"200"},{"price":"0.43","size":"500"},
			{"price":"0.42","size":"100"},{"price":"0.41","size":"200"},{"price":"0.40","size":"500"}
		],
		"asks": [
			{"price":"0.50","size":"100"},{"price":"0.51","size":"200"},{"price":"0.52","size":"500"},
			{"price":"0.53","size":"100"},{"price":"0.54","size":"200"},{"price":"0.55","size":"500"},
			{"price":"0.56","size":"100"},{"price":"0.57","size":"200"},{"price":"0.58","size":"500"}
		]
	}]`)

	msgChan := make(chan []byte, 1000)
	done := make(chan struct{})

	go func() {
		engine.ProcessStream("orderbook", msgChan)
		close(done)
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msgChan <- rawMsg
	}
	close(msgChan)
	<-done
}

func BenchmarkGetPriceNoBackend(b *testing.B) {
	s, _ := miniredis.Run()
	defer s.Close()

	provider := &NoOpStreamer{}
	engine := NewEngine(context.Background(), provider)
	
	rawMsg := []byte(`[{"asset_id":"Asset_123","bids":[{"price":"0.48","size":"100"}],"asks":[{"price":"0.50","size":"100"}]}]`)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			select {
			case <- ctx.Done():
				return
			default:
				engine.updateCache(rawMsg)
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = engine.GetPrice("Asset_123")
		}
	})
}

func BenchmarkPushNoBackend(b *testing.B) {
	s, _ := miniredis.Run()
	defer s.Close()
	
	provider := &NoOpStreamer{}
	engine := NewEngine(context.Background(), provider)
	
	rawMsg := []byte(`[{
		"asset_id": "Asset_123",
		"bids": [
			{"price":"0.48","size":"100"},{"price":"0.47","size":"200"},{"price":"0.46","size":"500"},
			{"price":"0.45","size":"100"},{"price":"0.44","size":"200"},{"price":"0.43","size":"500"},
			{"price":"0.42","size":"100"},{"price":"0.41","size":"200"},{"price":"0.40","size":"500"}
		],
		"asks": [
			{"price":"0.50","size":"100"},{"price":"0.51","size":"200"},{"price":"0.52","size":"500"},
			{"price":"0.53","size":"100"},{"price":"0.54","size":"200"},{"price":"0.55","size":"500"},
			{"price":"0.56","size":"100"},{"price":"0.57","size":"200"},{"price":"0.58","size":"500"}
		]
	}]`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.pushToRedis("orderbook", rawMsg)
	}
}

func BenchmarkProcessStreamE2ENoBackend(b *testing.B) {
	s, _ := miniredis.Run()
	defer s.Close()
	
	provider := &NoOpStreamer{}
	engine := NewEngine(context.Background(), provider)
	
	rawMsg := []byte(`[{
		"asset_id": "Asset_123",
		"bids": [
			{"price":"0.48","size":"100"},{"price":"0.47","size":"200"},{"price":"0.46","size":"500"},
			{"price":"0.45","size":"100"},{"price":"0.44","size":"200"},{"price":"0.43","size":"500"},
			{"price":"0.42","size":"100"},{"price":"0.41","size":"200"},{"price":"0.40","size":"500"}
		],
		"asks": [
			{"price":"0.50","size":"100"},{"price":"0.51","size":"200"},{"price":"0.52","size":"500"},
			{"price":"0.53","size":"100"},{"price":"0.54","size":"200"},{"price":"0.55","size":"500"},
			{"price":"0.56","size":"100"},{"price":"0.57","size":"200"},{"price":"0.58","size":"500"}
		]
	}]`)

	msgChan := make(chan []byte, 1000)
	done := make(chan struct{})

	go func() {
		engine.ProcessStream("orderbook", msgChan)
		close(done)
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msgChan <- rawMsg
	}
	close(msgChan)
	<-done
}