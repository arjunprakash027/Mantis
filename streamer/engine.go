package streamer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/arjunprakash027/Mantis/market"
	"github.com/redis/go-redis/v9"
)

type MarketState struct {
	BestAsk     float64
	BestBid     float64
	LastUpdated int64
}

type Engine struct {
	rdb    *redis.Client
	prices map[string]MarketState //the cache where our order placing paper engine will look at to determine the latest price of tokens
	mu     sync.RWMutex
	ctx    context.Context
}

type OrderbookUpdate struct {
	AssetID string `json:"asset_id"`
	BestBid string `json:"best_bid"` // Explicit best bid
	BestAsk string `json:"best_ask"` // Explicit best ask
	Bids    []struct {
		Price string `json:"price"`
		Size  string `json:"size"`
	} `json:"bids"`
	Asks []struct {
		Price string `json:"price"`
		Size  string `json:"size"`
	} `json:"asks"`
}

func NewEngine(ctx context.Context, rdb *redis.Client) *Engine {
	return &Engine{
		rdb:    rdb,
		prices: make(map[string]MarketState),
		ctx:    ctx,
	}
}

func (e *Engine) GetContext() context.Context {
	return e.ctx
}

func (e *Engine) GetPrice(assetID string) (MarketState, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	data, ok := e.prices[assetID]
	return data, ok
}

func (e *Engine) ProcessStream(namespace string, msgChan <-chan []byte) {
	for rawMsg := range msgChan {
		// Update internal price cache for the paper executor
		if namespace == "orderbook" {
			e.updateCache(rawMsg)
		}
		e.pushToRedis(namespace, rawMsg)
	}
}

func (e *Engine) RegisterMetadata(slug string, tokens []market.Token) error {
	pipe := e.rdb.Pipeline()
	slugKey := fmt.Sprintf("slug:assets:%s", slug)

	for _, t := range tokens {
		key := fmt.Sprintf("token:meta:%s", t.TokenID)
		pipe.HSet(e.ctx, key, map[string]interface{}{
			"id":      t.TokenID,
			"outcome": t.Outcome,
			"market":  t.Market,
			"slug":    slug,
		})
		pipe.SAdd(e.ctx, slugKey, t.TokenID)
	}
	_, err := pipe.Exec(e.ctx)
	return err
}

func (e *Engine) updateCache(rawMsg []byte) {
	var updates []OrderbookUpdate

	if len(rawMsg) > 0 && rawMsg[0] == '[' {
		json.Unmarshal(rawMsg, &updates)
	} else {
		var single OrderbookUpdate
		if json.Unmarshal(rawMsg, &single) == nil {
			updates = append(updates, single)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for _, u := range updates {
		if u.AssetID == "" {
			continue
		}

		state, exists := e.prices[u.AssetID]
		if !exists {
			state = MarketState{}
		}

		updated := false

		// 1. Try explicit BestBid/Ask fields (sent in price_change events)
		if u.BestBid != "" {
			if v, err := strconv.ParseFloat(u.BestBid, 64); err == nil {
				state.BestBid = v
				updated = true
			}
		} else if len(u.Bids) > 0 {
			// Find MAX bid price
			maxBid := -1.0
			for _, b := range u.Bids {
				if v, err := strconv.ParseFloat(b.Price, 64); err == nil {
					if v > maxBid {
						maxBid = v
					}
				}
			}
			if maxBid != -1.0 {
				state.BestBid = maxBid
				updated = true
			}
		}

		if u.BestAsk != "" {
			if v, err := strconv.ParseFloat(u.BestAsk, 64); err == nil {
				state.BestAsk = v
				updated = true
			}
		} else if len(u.Asks) > 0 {
			// Find MIN ask price
			minAsk := 2.0 // Prices are between 0 and 1
			for _, a := range u.Asks {
				if v, err := strconv.ParseFloat(a.Price, 64); err == nil {
					if v < minAsk {
						minAsk = v
					}
				}
			}
			if minAsk != 2.0 {
				state.BestAsk = minAsk
				updated = true
			}
		}

		if updated {
			state.LastUpdated = time.Now().Unix()
			e.prices[u.AssetID] = state
		}
	}
}

func (e *Engine) pushToRedis(namespace string, rawMsg []byte) {

	type RouterMsg struct {
		AssetID string `json:"asset_id"`
	}

	if namespace == "discovery" {
		e.streamAdd(namespace, "all", rawMsg)
		return
	}

	// Route by AssetID
	if len(rawMsg) > 0 && rawMsg[0] == '[' {
		var batch []RouterMsg
		if json.Unmarshal(rawMsg, &batch) == nil {
			for _, m := range batch {
				e.streamAdd(namespace, m.AssetID, rawMsg)
			}
		}
	} else {
		var m RouterMsg
		if json.Unmarshal(rawMsg, &m) == nil {
			e.streamAdd(namespace, m.AssetID, rawMsg)
		}
	}
}

func (e *Engine) streamAdd(namespace string, identifier string, data []byte) {
	if identifier == "" {
		return
	}

	// Format: <namespace>:stream:<identifier> (e.g., market:stream:0x123)
	streamKey := fmt.Sprintf("%s:stream:%s", namespace, identifier)

	err := e.rdb.XAdd(e.ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: 1000,
		Approx: true,
		Values: map[string]interface{}{"data": data},
	}).Err()

	if err != nil {
		log.Printf("Redis Stream Error [%s]: %v", streamKey, err)
	}
}
