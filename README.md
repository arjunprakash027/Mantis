# Mantis

Mantis is a high-performance market data collector and paper trading engine designed for [Polymarket](https://polymarket.com/). It bridges the gap between Polymarket's global API/WebSocket infrastructure and local high-frequency trading systems by piping live data and simulated order execution into a low-latency Redis backend.

## Architecture

Mantis is built in Go for speed and safety, utilizing a modular "One-Channel-Per-Stream" architecture. It runs multiple concurrent data pipelines:

1.  **Global Discovery Engine**: Periodically scans the entire exchange (pagination over ~28k+ active markets) to find the newest and most liquid opportunities.
2.  **Smart Metadata Registry**: Maps market slugs to underlying CLOB asset IDs and outcome names, allowing bots to perform discovery without hitting Polymarket APIs.
3.  **High-Speed Ingestion**: Maintains persistent WebSocket connections to the Polymarket CLOB for real-time L2 orderbook updates with automated heartbeats and fail-safe logging.
4.  **Atomic Paper Executor**: A high-fidelity trading simulator that executes orders against **live** orderbook prices using atomic Redis Lua scripts.

## Current Progress

- [x] **Full Exchange Discovery**: Automated pagination to fetch and index all active markets.
- [x] **Real-Time Paper Trading**: Integrated execution engine that tracks balances, fills orders at live BBA (Best Bid/Offer), and maintains an audit log.
- [x] **YAML-Based Management**: Configure multiple markets and toggle discovery/orderbook pipelines via `config.yaml`.
- [x] **Safe Execution**: Implementation of strict price caching (no assumptions) and 60-second stale price guards to prevent trading on "zombie" data.
- [x] **High-Performance Routing**: Sub-millisecond distribution of WebSocket updates into namespaced Redis Streams.

## Getting Started

### Prerequisites
- Go 1.21+
- Redis (running locally on port 6379)
- Python 3.x (for the sample order script)

### Setup & Configuration
Mantis is managed via `config.yaml`. Add the market slugs you want to track to the `orderbook.markets` list.

```yaml
pipelines:
  discovery:
    enabled: true
    interval_minutes: 10
  orderbook:
    enabled: true
    markets:
      - bitcoin-up-or-down-february-19-10am-et
```

### Running the Engine
```bash
# 1. Start your local Redis
brew services start redis

# 2. Fund your paper trading account (Optional - Default is $0)
redis-cli HSET portfolio:balance USD 1000

# 3. Start Mantis
go run main.go
```

## Paper Trading Guide

Mantis includes an Atomic Execution Engine. When you send a trade signal, it checks the **local price cache** (populated by the live WebSocket) and executes the trade only if the data is fresh and reliable.

### 1. Placing an Order
Use the included `order.py` script to send signals to the engine.

```bash
# Usage: python3 order.py <BUY/SELL> <TOKEN_ID> <AMOUNT>
python3 order.py BUY 538482956... 10
```

### 2. Managing your Account (Redis)
All portfolio data is stored in the `portfolio:balance` hash.

*   **View Balance & Positions**: `redis-cli HGETALL portfolio:balance`
*   **Add Funds**: `redis-cli HINCRBYFLOAT portfolio:balance USD 500`
*   **View Trade History**: `redis-cli XRANGE trade:log - +`

### 3. Execution Rules
- **No Assumptions**: Orders are only filled if the engine has received an explicit `best_bid` or `best_ask` from the exchange.
- **Stale Guard**: If a price hasn't been updated in **60 seconds**, the executor will reject the trade to prevent "slippage" against dead data.
- **Atomic Fills**: Using Lua scripts ensures that your balance update and trade logging happen as a single atomic unit—no partial fills or missed logs.

## Data Schema

### 1. Global Discovery (Stream)
`XREAD BLOCK 0 STREAMS discovery:stream:all $`
- Contains metadata for all 28k+ active markets.

### 2. Live Orderbook (Stream)
`XREAD BLOCK 0 STREAMS orderbook:stream:<asset_id> $`
- Namespaced L2 updates for markets defined in your `config.yaml`.

### 3. Execution Signals (Streams)
- **Inbound Signals**: `signals:inbound` (Format: `{"action": "BUY", "asset": "ID", "amount": 1.0}`)
- **Outbound Results**: `signals:outbound` (Contains fill price, timestamp, and any error messages).

---

*Note: The trading strategy implementations and execution modules remain internal and private (Simulated context).*
