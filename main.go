package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/arjunprakash027/Mantis/market"
	"github.com/arjunprakash027/Mantis/streamer"
	"github.com/redis/go-redis/v9"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <slug>")
	}

	slug := os.Args[1]

	// 1. Setup Redis
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()

	// 2. Fetch Tokens
	tokens, eventTitle, err := market.GetTokens(slug)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Register Metadata (for the Python bot)
	fmt.Printf("📝 Registering metadata for %d tokens in: %s\n", len(tokens), eventTitle)
	if err := streamer.RegisterMetadata(ctx, rdb, slug, tokens); err != nil {
		log.Printf("Metadata warning: %v", err)
	}

	// 4. Start WebSocket Stream
	assetIds := make([]string, len(tokens))
	for i, t := range tokens {
		assetIds[i] = t.TokenID
	}

	// STARTING ALL THE STREAMS HERE
	// OrderBook Stream
	orderBookChan := make(chan []byte)
	if err := market.StartOrderBookStream(assetIds, orderBookChan); err != nil {
		log.Fatal(err)
	}
	fmt.Println("🚀 Streaming to Redis. Press Ctrl+C to stop.")
	go streamer.ProcessStream(ctx, rdb, "orderbook", orderBookChan)

	// Discovery Stream
	discoveryChan := make(chan []byte)
	if err := market.StartDiscoveryStream(discoveryChan); err != nil {
		log.Fatal(err)
	}
	go streamer.ProcessStream(ctx, rdb, "discovery", discoveryChan)

	select {}
}
