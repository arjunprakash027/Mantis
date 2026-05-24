# Mantis (v0.0.2)

> **Note:** To view the latest updates and release notes, please see the [CHANGELOG.md](CHANGELOG.md) file.

**Mantis** is an ultra-low latency, high-performance market data ingestion and atomic paper-trading engine designed for [Polymarket](https://polymarket.com/). It bridges the gap between Polymarket's global Central Limit Order Book (CLOB) infrastructure and local high-frequency trading (HFT) models. 

By employing advanced lock-free optimization patterns, parallel stream pipelines, and a fully decoupled interface-driven architecture, Mantis is capable of processing thousands of ticks per second, enabling real-time quantitative modeling and risk-free strategy execution.

---

## Architecture & Design Philosophy

Mantis is written in Go and utilizes a highly modular **"One-Channel-Per-Stream"** design. The system is split into independent subsystems that communicate entirely through abstract transport interfaces, isolating the complex domain logic from infrastructure details.

### 1. Transport-Agnostic Pluggable Backends
The core of Mantis's extensibility lies in the `pkg/backend` module. The application's core engines (`streamer.Engine` and `executor.Executor`) do not depend on any specific database or message broker. 

Instead, they rely on two strictly defined interfaces:
- `StreamerBackend`: For fast caching, state updates, and outbound event streaming.
- `ExecutorBackend`: For atomic balance validation, signal processing, and trade logging.

**What does this mean for you?**
Mantis ships with a production-grade **RedisProvider** (utilizing atomic Lua scripts and Redis Streams) out of the box. However, because of this pluggable design, developers can seamlessly inject alternative data transports:
- **NATS JetStream** or **Apache Kafka** for massive scale out.
- **Relational SQL** for persistent analytics.
- **In-Memory (NoOp) structs** for absolute zero-latency local testing.

### 2. High-Speed Ingestion Engine
Mantis maintains persistent WebSocket connections to the Polymarket CLOB. The ingestion pipeline has been heavily optimized to eliminate thread contention:
- **Zero-Blocking Parsing**: Heavy JSON deserialization and string-to-float conversions (`strconv.ParseFloat`) are executed strictly outside of state write-locks.
- **RWMutex Optimization**: State commits hold `sync.RWMutex` locks for mere nanoseconds, ensuring the Execution Engine is never blocked from reading the `best_bid`/`best_ask` during periods of extreme market volatility.

### 3. Atomic Paper Executor
A high-fidelity trading simulator that fully replicates real-world constraints.
- **No Assumptions**: Orders are evaluated against the *exact* microsecond state of the local pricing cache.
- **Stale Guards**: Trades are immediately rejected if the underlying pricing data hasn't seen a tick in the last **60 seconds**, protecting against "zombie" markets.
- **Atomicity**: Trade finality and balance mutations happen synchronously. In the Redis implementation, this is achieved via pre-compiled `//go:embed` Lua scripts executed within single server ticks.

### 4. Smart Metadata Registry
A background discovery service systematically paginates through Polymarket's 28k+ active markets. It builds a localized, highly queryable mapping of Human-Readable Market Slugs to raw CLOB Asset IDs, allowing your downstream models to operate cleanly without hitting third-party REST APIs.

---

## Performance & Benchmarking

Mantis is designed for quantitative scale. To ensure the engine never becomes the bottleneck, it includes a rigorous built-in benchmarking suite that segregates **Micro** (CPU path efficiency) from **Macro** (Database I/O) performance.

You can stress-test the application on your own hardware:
```bash
# Benchmark Ingestion (JSON Parsing & Lock Contention)
cd streamer
go test -bench=. -benchmem

# Benchmark Transaction Executor (TPS limits)
cd ../executor
go test -bench=. -benchmem
```

### The Power of "NoOp" (Zero-I/O) Benchmarking
Because of the pluggable interface architecture, our benchmark suite includes **NoOp (No-Operation) Providers**. These mock backends instantly discard database writes, allowing the benchmarks to completely bypass network latency and Redis I/O. 

By running the NoOp benchmarks (`BenchmarkProcessSignal_NoOp`), developers can isolate and measure the **pure raw speed of Go's CPU execution** (payload unmarshaling, context switching, state evaluation) without external noise.

---

## Getting Started

### Prerequisites
- **Go 1.21+**
- **Redis** (running locally on port `6379`)
- **Python 3.x** (for interacting via included scripts)

### Setup & Configuration
Mantis relies on a declarative `config.yaml`. Define the Polymarket slugs you want to track under the `orderbook` configuration.

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

# 2. Reset the environment (Wipe DB to start fresh)
redis-cli FLUSHALL

# 3. Fund your simulated portfolio (e.g., $1000)
redis-cli HSET portfolio:balance USD 1000

# 4. Boot the Mantis Engine
go run main.go
```

---

## Interfacing with Mantis

Mantis acts as the spine of your trading stack. Your proprietary quantitative models and bots simply read the streams and publish signals.

### Data Schema (Redis Provider)
- **Global Discovery Stream**: `XREAD BLOCK 0 STREAMS discovery:stream:all $`
- **Live Orderbook Stream**: `XREAD BLOCK 0 STREAMS orderbook:stream:<asset_id> $`
- **Inbound Trade Signals**: Publish to `signals:inbound`
  - *Format*: `{"action": "BUY", "asset": "ID", "amount": 1.0}`
- **Outbound Fill Results**: `signals:outbound`

### Python SDK & Scripts
We provide sample Python scripts in the `scripts/` directory to demonstrate integration.

```bash
cd scripts
pip install -r requirements.txt

# View Tracked Markets & Portfolio Balance
python3 get_redis_information.py

# Listen to the Live BBO Stream for an Asset
python3 get_order_book.py <TOKEN_ID>

# Run a sample Random Noise Trader
python3 random_trader.py
```

---

## Automated Deployment

Mantis includes a streamlined script (`deploy.sh`) to cross-compile for Linux (AMD64) and push the binary to remote VPS environments via SCP.

1. Create an SSH alias (`server-alias`) in your `~/.ssh/config`.
2. Configure passwordless login (`ssh-copy-id server-alias`).
3. Run the deployer:

```bash
chmod +x deploy.sh
./deploy.sh
```

---
*Built for extreme latency conditions. Safe for simulations. The core strategy implementations are entirely abstracted for your own proprietary integration.*
