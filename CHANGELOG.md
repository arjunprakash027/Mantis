# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
