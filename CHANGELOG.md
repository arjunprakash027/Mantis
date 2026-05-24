# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.2] - 2026-05-24

### Added
- **Pluggable Backend Architecture (`pkg/backend`)**: Abstracted the data transport layer and state transaction layer into two decoupled interfaces: `StreamerBackend` and `ExecutorBackend`. This completely frees the application core from depending on Redis.
- **Extensible Data Transports**: Designed contracts so that data pipelines and transaction logging can swap Redis with alternatives like NATS JetStream, Apache Kafka, relational SQL databases, or raw **In-Memory** data structures.
- **Inbound Signal Subscription Pattern**: Refactored the executor to ingest inbound trading signals via a reactive callback closure (`SubscribeInboundSignals`), removing all Redis stream polling loops (`XReadGroup`) and message formats (`redis.XMessage`) from the executor business logic.
- **Executor Performance Benchmarks**: Added a high-fidelity transaction throughput benchmark suite in `executor/executor_test.go` (`BenchmarkProcessSignal`) to measure successful buy execution latencies and memory allocations under sandbox `miniredis` conditions.
- **Pipeline Ingestion Benchmarks**: Implemented `BenchmarkProcessStreamE2E` in `streamer/engine_test.go` to measure maximum ingestion throughput, including Go channel allocations, goroutine thread-switching overhead, and stream writing.

### Changed
- **Decoupled Injection**: Refactored constructors `streamer.NewEngine` and `executor.NewExecutor` to accept their respective backend interfaces rather than concrete `*redis.Client` pointers.
- **Compile-Time Safe Assertions**: Implemented implicit interface safety checks inside `pkg/backend/redisprovider.go` to ensure compilation errors occur immediately if the concrete provider breaks structural typing rules.

## [0.0.11] - 2026-05-21

### Added
- **Benchmarking Suite**: Added a new benchmark file (`streamer/engine_test.go`) focusing on the hottest path in the system (`updateCache`).
  - `BenchmarkUpdateCacheSingle`: Measures single-threaded JSON parsing and string-to-float conversion latency.
  - `BenchmarkUpdateCacheMultiple`: A multi-threaded `RunParallel` benchmark simulating highly concurrent WebSocket ticks, complete with a pre-allocated payload pool to accurately measure `RWMutex` contention limits without memory-allocation bias.
- **Documentation**: Added a `## Benchmarking` section to the `README.md` to guide developers on testing system limits.

### Changed
- **Lock-Free Parsing Optimization**: Drastically optimized `updateCache` in `streamer/engine.go` to reduce `sync.RWMutex` contention. 
  - The heavy lifting of JSON unmarshaling and `strconv.ParseFloat` was moved *outside* of the write-lock. 
  - A read-lock (`RLock`) is now used to quickly fetch the current state.
  - The write-lock (`Lock`) is only acquired for nanoseconds at the very end to apply the calculated updates to the map. This prevents the streaming engine from blocking the executor (`GetPrice`) during active parsing.
- **Gitignore Update**: Added `.DS_STORE` to `.gitignore`.
