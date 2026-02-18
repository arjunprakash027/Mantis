package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/arjunprakash027/Mantis/config"
	"github.com/arjunprakash027/Mantis/executor"
	"github.com/arjunprakash027/Mantis/market"
	"github.com/arjunprakash027/Mantis/streamer"
	"github.com/redis/go-redis/v9"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Setup Redis
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("🐝 Mantis Data Engine Starting...")

	marketEngine := streamer.NewEngine(ctx, rdb)

	// 3. Start Orderbook Pipelines
	if cfg.Pipelines.Orderbook.Enabled {
		fmt.Printf("🚀 Starting Orderbook Pipelines for %d markets...\n", len(cfg.Pipelines.Orderbook.Markets))
		for _, slug := range cfg.Pipelines.Orderbook.Markets {
			go startOrderbookForSlug(marketEngine, slug)
		}
	}

	// 4. Start Discovery Pipeline
	if cfg.Pipelines.Discovery.Enabled {
		fmt.Printf("🚀 Starting Discovery Pipeline for every %d minutes...\n", cfg.Pipelines.Discovery.IntervalMinutes)
		discoveryChan := make(chan []byte)
		if err := market.StartDiscoveryStream(discoveryChan); err != nil {
			log.Printf("Discovery Error: %v", err)
		} else {
			go marketEngine.ProcessStream("discovery", discoveryChan)
		}
	}

	exec := executor.NewExecutor(ctx, rdb, marketEngine)
	go exec.Start()

	fmt.Println("✅ Pipelines & Executor active. Press Ctrl+C to stop.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down Mantis...")
	cancel()
	rdb.Close()
}

func startOrderbookForSlug(engine *streamer.Engine, slug string) {
	// A. Fetch Tokens
	tokens, eventTitle, err := market.GetTokens(slug)
	if err != nil {
		log.Printf("[%s] Lookup Error: %v", slug, err)
		return
	}

	// B. Register Metadata
	if err := engine.RegisterMetadata(slug, tokens); err != nil {
		log.Printf("[%s] Metadata warning: %v", slug, err)
	}

	// C. Start Stream
	assetIds := make([]string, len(tokens))
	for i, t := range tokens {
		assetIds[i] = t.TokenID
	}

	msgChan := make(chan []byte)
	if err := market.StartOrderBookStream(engine.GetContext(), assetIds, msgChan); err != nil {
		log.Printf("[%s] Stream Error: %v", slug, err)
		return
	}

	fmt.Printf("✅ Streaming %s (%d tokens)\n", eventTitle, len(tokens))

	engine.ProcessStream("orderbook", msgChan)
}
