package streamer

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/arjunprakash027/Mantis/market"
	"github.com/redis/go-redis/v9"
)

func TestFullSystemEndToEnd(t *testing.T) {

	s, _ := miniredis.Run()
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	engine := NewEngine(ctx, rdb)
	t.Log("🐝 System Initialized with Sandbox Redis")

	slug := "will-trump-pardon-ghislaine-maxwell"
	t.Logf("🚀 Connecting to real-world market: %s", slug)

	tokens, eventTitle, err := market.GetTokens(slug)
	if err != nil {
		t.Fatalf("LookUp Error: %v", err)
	}
	t.Logf("✅ Discovered Market: %s", eventTitle)

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
				t.Fatal("❌ Timeout: Did not receive any live prices from Polymarket after 20s")
			}
			return
		case <-ticker.C:
			engine.mu.RLock()
			count := len(engine.prices)
			engine.mu.RUnlock()

			if count > 0 {
				t.Logf("📈 Success: Engine is receiving live data for %d assets!", count)
				foundData = true
				return
			}
			t.Log("... waiting for network packets ...")
		}
	}
}
