# Mantis

Mantis is a high-performance market data collector designed for [Polymarket](https://polymarket.com/). It bridges the gap between Polymarket's real-time WebSocket infrastructure and local high-frequency trading systems by piping live orderbook data into a low-latency Redis backend.

## Architecture

Mantis is built in Go for speed and safety. It follows a clean pipeline:
1.  **Discovery**: Resolves market or event slugs into a complete list of CLOB asset IDs.
2.  **Registration**: Stores static metadata (market names, outcomes, slugs) in Redis for instant "offline" lookup by downstream bots.
3.  **Ingestion**: Maintains a high-speed WebSocket connection to the Polymarket CLOB.
4.  **Routing**: Quick-parses incoming L2 orderbook updates and routes them into individual per-asset Redis Streams.

## Current Progress

- [x] **Smart Slug Resolution**: Automatically detects if a slug is a single Market or a large Event with multiple internal markets.
- [x] **Redis Integration**: Implemented a modular `streamer` package using Redis Streams and Hashes.
- [x] **High-Speed Routing**: Real-time distribution of WebSocket messages into filtered Redis keys.
- [x] **Metadata Mapping**: Indexed slug-to-asset relationships to allow bots to "discover" available tokens without API calls.

## Getting Started

### Prerequisites
- Go 1.21+
- Redis (running locally on port 6379)

### Running the Collector
```bash
# Start your local Redis
brew services start redis

# Build and run the collector for a specific slug
go run main.go us-strikes-iran-by
```

## Data Schema

### Metadata (Hashes)
Standard lookup for token descriptions:
`HGETALL token:meta:<asset_id>`

### Asset Indexing (Sets)
Retrieve all tokens belonging to a specific slug:
`SMEMBERS slug:assets:<slug>`

### Live Streams (Streams)
The high-speed pipeline for L2 snapshots:
`XREAD BLOCK 0 STREAMS market:stream:<asset_id> $`

## Trading Bot Integration

Mantis is designed to be the foundational "data layer" for a suite of quantitative trading tools. By standardizing the data into Redis, it allows multiple specialized trading bots (written in Python, Rust, or C++) to consume isolated, high-speed price streams without duplicating API overhead or network connections.

*Note: The trading strategy implementations and execution modules remain internal and private.*
