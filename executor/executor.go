package executor

import (
	"context"
	_ "embed"
	"encoding/json"
	"log"
	"time"

	"github.com/arjunprakash027/Mantis/streamer"
	"github.com/arjunprakash027/Mantis/pkg/backend"
)

type Signal struct {
	Action     string  `json:"action"`
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
	backend  backend.ExecutorBackend
	engine *streamer.Engine
	ctx    context.Context
}

func NewExecutor(ctx context.Context, b backend.ExecutorBackend, engine *streamer.Engine) *Executor {
	return &Executor{
		backend:    b,
		engine: engine,
		ctx:    ctx,
	}
}

func (e *Executor) Start() {

	log.Println("Executor Started: Subscribing to inbound signals...")
	
	err := e.backend.SubscribeInboundSignals(e.ctx, func(msgID string, payload []byte) {
		e.processSignalPayload(msgID, payload)
	})

	if err != nil {
		log.Printf("Signal subscription finished: %v", err)
	}
}

func (e *Executor) processSignalPayload(msgID string, payload []byte) {
	var sig Signal

	if err := json.Unmarshal(payload, &sig); err != nil {
		log.Printf("Invalid JSON: %v", err)
		return
	}

	priceState, exists := e.engine.GetPrice(sig.Asset)

	if !exists {
		e.respond(sig, ExecutionResult{Success: false, ErrorMsg: "Asset not streamed"})
		_ = e.backend.AcknowledgeSignal(e.ctx, msgID)
		return
	}

	if time.Now().Unix()-priceState.LastUpdated > 60 {
		e.respond(sig, ExecutionResult{Success: false, ErrorMsg: "Stale price (stream lagging or dead)"})
		_ = e.backend.AcknowledgeSignal(e.ctx, msgID)
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
		_ = e.backend.AcknowledgeSignal(e.ctx, msgID)
		return
	}

	success, errorMsg, err := e.backend.ExecuteTrade(e.ctx, sig.Action, sig.Asset, sig.Amount, fillPrice, sig.StrategyID)
	if err != nil {
		log.Printf("Execution Error: %v", err)
		e.respond(sig, ExecutionResult{Success: false, ErrorMsg: "Internal DB Error"})
		_ = e.backend.AcknowledgeSignal(e.ctx, msgID)
		return
	}

	result := ExecutionResult{
		Success:      success,
		FilledPrice:  fillPrice,
		FilledAmount: sig.Amount,
		Timestamp:    time.Now().Unix(),
	}
	if !success {
		result.ErrorMsg = errorMsg
	}

	e.respond(sig, result)
}

func (e *Executor) respond(sig Signal, res ExecutionResult) {
	jsonRes, _ := json.Marshal(res)

	e.backend.PublishSignalResult(e.ctx, sig.StrategyID, jsonRes)

	if res.Success {
		log.Printf("%s %s | Price: %.2f | Amount: %.2f", sig.Action, sig.Asset, res.FilledPrice, sig.Amount)
	} else {
		log.Printf("%s REJECTED | Asset: %s | Reason: %s", sig.Action, sig.Asset, res.ErrorMsg)
	}
}
