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
	prices map[string]MarketState
	mu     sync.RWMutex
	ctx    context.Context
}

type OrderbookUpdate struct {
	AssetID string `json:"asset_id"`
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
	if len(rawMsg) == 0 {
		return
	}

	var updates []OrderbookUpdate
	if rawMsg[0] == '[' {
		if err := json.Unmarshal(rawMsg, &updates); err != nil {
			return
		}
	} else {
		var single OrderbookUpdate
		if err := json.Unmarshal(rawMsg, &single); err == nil {
			updates = append(updates, single)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range updates {
		u := &updates[i]
		if u.AssetID == "" {
			continue
		}

		state := e.prices[u.AssetID]
		updated := false

		if n := len(u.Bids); n > 0 {
			if price, err := strconv.ParseFloat(u.Bids[n-1].Price, 64); err == nil {
				state.BestBid = price
				updated = true
			}
		}

		if n := len(u.Asks); n > 0 {
			if price, err := strconv.ParseFloat(u.Asks[n-1].Price, 64); err == nil {
				state.BestAsk = price
				updated = true
			}
		}

		if updated {
			state.LastUpdated = time.Now().UnixNano()
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
