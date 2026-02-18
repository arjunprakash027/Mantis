package executor

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/arjunprakash027/Mantis/streamer"
	"github.com/redis/go-redis/v9"
)

type Signal struct {
	Action     string  `json:"action"` // "BUY" or "SELL"
	Asset      string  `json:"asset"`
	Amount     float64 `json:"amount"`
	StrategyID string  `json:"strategy_id"`
}

type ExecutionResult struct {
	Success      bool    `json:"success"`
	FilledPrice  float64 `json:"filled_price"`
	FilledAmount float64 `json:"filled_amount"`
	Fee          float64 `json:"fee"`
	ErrorMsg     string  `json:"error_msg,omitempty"`
	Timestamp    int64   `json:"timestamp"`
}

type Executor struct {
	rdb    *redis.Client
	engine *streamer.Engine
	ctx    context.Context
}

var tradeScript = redis.NewScript(`
    local portfolio_key = KEYS[1]
    local trade_log_key = KEYS[2]
    
    local action = ARGV[1]
    local asset = ARGV[2]
    local amount = tonumber(ARGV[3])
    local price = tonumber(ARGV[4])
    local total_cost = tonumber(ARGV[5]) 
    local timestamp = ARGV[6]

    -- Validation
    if action == "BUY" then
        local usd_balance = tonumber(redis.call('HGET', portfolio_key, 'USD') or 0)
        if usd_balance < total_cost then
            return {0, "Insufficient USD funds"}
        end
        redis.call('HINCRBYFLOAT', portfolio_key, 'USD', -total_cost)
        redis.call('HINCRBYFLOAT', portfolio_key, asset, amount)
        
    elseif action == "SELL" then
        local asset_balance = tonumber(redis.call('HGET', portfolio_key, asset) or 0)
        if asset_balance < amount then
            return {0, "Insufficient asset balance"}
        end
        redis.call('HINCRBYFLOAT', portfolio_key, asset, -amount)
        redis.call('HINCRBYFLOAT', portfolio_key, 'USD', total_cost)
    end

    -- Audit Log
    redis.call('XADD', trade_log_key, '*', 
        'action', action, 'asset', asset, 'amount', amount, 
        'price', price, 'total', total_cost, 'timestamp', timestamp
    )

    return {1, "Success"}
`)

func NewExecutor(ctx context.Context, rdb *redis.Client, engine *streamer.Engine) *Executor {
	return &Executor{
		rdb:    rdb,
		engine: engine,
		ctx:    context.Background(),
	}
}

func (e *Executor) Start() {

	log.Println("🚀 Executor Started: Listening on signals:inbound")
	// Ensure consumer group exists
	e.rdb.XGroupCreateMkStream(e.ctx, "signals:inbound", "mantis_executors", "$")

	for {
		streams, err := e.rdb.XReadGroup(e.ctx, &redis.XReadGroupArgs{
			Group:    "mantis_executors",
			Consumer: "worker_1",
			Streams:  []string{"signals:inbound", ">"},
			Count:    1,
			Block:    0,
		}).Result()

		if err != nil {
			log.Printf("Redis Stream Error [%s]: %v", "signals:inbound", err)
			continue
		}

		for _, msg := range streams[0].Messages {
			e.processSignal(msg)
			e.rdb.XAck(e.ctx, "signals:inbound", "mantis_executors", msg.ID)
		}
	}
}

func (e *Executor) processSignal(msg redis.XMessage) {
	var sig Signal

	dataStr, ok := msg.Values["data"].(string)
	if !ok {
		log.Printf("❌ Invalid signal format: missing 'data' field")
		return
	}

	if err := json.Unmarshal([]byte(dataStr), &sig); err != nil {
		log.Printf("❌ Invalid JSON: %v", err)
		return
	}

	priceState, exists := e.engine.GetPrice(sig.Asset)

	if !exists {
		e.respond(sig, ExecutionResult{Success: false, ErrorMsg: "Asset not streamed"})
		return
	}

	// If price hasn't been updated in 60s, it's a "zombie" price
	if time.Now().Unix()-priceState.LastUpdated > 60 {
		e.respond(sig, ExecutionResult{Success: false, ErrorMsg: "Stale price (stream lagging or dead)"})
		return
	}

	fillPrice := 0.0
	if sig.Action == "BUY" {
		fillPrice = priceState.BestAsk
	} else {
		fillPrice = priceState.BestBid
	}

	if fillPrice <= 0 {
		e.respond(sig, ExecutionResult{Success: false, ErrorMsg: "No liquidity (price 0)"})
		return
	}

	totalCost := fillPrice * sig.Amount

	res, err := tradeScript.Run(e.ctx, e.rdb,
		[]string{"portfolio:balance", "trade:log"},
		sig.Action, sig.Asset, sig.Amount, fillPrice, totalCost, time.Now().Unix(),
	).Result()

	if err != nil {
		log.Printf("Redis Lua Error: %v", err)
		e.respond(sig, ExecutionResult{Success: false, ErrorMsg: "Internal DB Error"})
		return
	}

	resSlice := res.([]interface{})
	success := resSlice[0].(int64) == 1

	result := ExecutionResult{
		Success:      success,
		FilledPrice:  fillPrice,
		FilledAmount: sig.Amount,
		Timestamp:    time.Now().Unix(),
	}
	if !success {
		result.ErrorMsg = resSlice[1].(string)
	}

	e.respond(sig, result)
}

func (e *Executor) respond(sig Signal, res ExecutionResult) {
	jsonRes, _ := json.Marshal(res)

	e.rdb.XAdd(e.ctx, &redis.XAddArgs{
		Stream: "signals:outbound",
		Values: map[string]interface{}{
			"strategy_id": sig.StrategyID,
			"data":        jsonRes,
		},
	})

	if res.Success {
		log.Printf("✅ %s %s | Price: %.2f | Amount: %.2f", sig.Action, sig.Asset, res.FilledPrice, sig.Amount)
	} else {
		log.Printf("❌ %s REJECTED | Asset: %s | Reason: %s", sig.Action, sig.Asset, res.ErrorMsg)
	}
}
