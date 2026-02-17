# Mantis

Mantis is a high-performance market data collector designed for [Polymarket](https://polymarket.com/). It bridges the gap between Polymarket's global API/WebSocket infrastructure and local high-frequency trading systems by piping live data into a low-latency Redis backend.

## Architecture

Mantis is built in Go for speed and safety, utilizing a modular "One-Channel-Per-Stream" architecture. It runs multiple concurrent data pipelines:

1.  **Global Discovery Engine**: Periodically scans the entire exchange (pagination over ~28k+ active markets) to find the newest and most liquid opportunities.
2.  **Smart Metadata Registry**: Maps market slugs to underlying CLOB asset IDs and outcome names, allowing bots to perform discovery without hitting Polymarket APIs.
3.  **High-Speed Ingestion**: Maintains persistent WebSocket connections to the Polymarket CLOB for real-time L2 orderbook updates.
4.  **Multi-Namespace Routing**: Dispatches raw data into isolated Redis Streams (`orderbook`, `discovery`, etc.) for specialized bot consumption.

## Current Progress

- [x] **Full Exchange Discovery**: Automated pagination to fetch and index all active markets every 10 minutes.
- [x] **Advanced Analytical Signals**: Discovery data now includes price momentum (1h/24h), liquidity depth, spreads, and creation timestamps.
- [x] **Modular Streamer**: Refactored `streamer` package with generalized `PushToStream` logic and namespaced keys.
- [x] **Concurrency Layer**: Multi-threaded execution allows simultaneous monitoring of specific slugs and global exchange discovery.
- [x] **Redis-Agnostic Routing**: Intelligent handling of both single-asset snapshots and global exchange-wide updates.

## Getting Started

### Prerequisites
- Go 1.21+
- Redis (running locally on port 6379, recommended with `appendonly yes` for persistence)

### Running the Collector
```bash
# Start your local Redis
brew services start redis

# Run the collector for a target market/event
# This will simultaneously stream the target and scan the whole exchange
go run main.go us-strikes-iran-by
```

## Data Schema & Discovery

### 1. Global Market Discovery (Stream)
The "Exchange Firehose" containing snapshots of all active markets:
`XREAD BLOCK 0 STREAMS discovery:stream:all $`

**Available Signals (JSON):**
*   `startDateIso` / `endDateIso`: Timing for age-based filtering.
*   `oneDayPriceChange` / `oneHourPriceChange`: Price momentum.
*   `spread`: Market efficiency indicator.
*   `clobTokenIds`: Instant routing IDs for orderbook streaming.

### 2. Real-Time Orderbook (Stream)
Low-latency per-asset L2 updates:
`XREAD BLOCK 0 STREAMS orderbook:stream:<asset_id> $`

### 3. Metadata & Indexing (Hashes & Sets)
*   **Token Lookup**: `HGETALL token:meta:<asset_id>`
*   **Slug Index**: `SMEMBERS slug:assets:<slug>`

## Trading Bot Integration

Mantis serves as the centralized data layer for distributed trading architectures. By decoupling data ingestion (Go) from strategy execution (Python/Rust), it ensures:
- **Zero Duplication**: One network connection serves unlimited local bots.
- **Micro-millisecond Latency**: Redis Streams provide an extremely fast inter-process communication (IPC) layer.
- **Durability**: Bots can "go offline" and catch up on missed market movements using Redis Stream IDs.

*Note: The trading strategy implementations and execution modules remain internal and private.*
