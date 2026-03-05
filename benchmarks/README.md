# Performance Harness

This harness establishes reproducible baseline metrics and captures pprof profiles.

## Prerequisites

- Running gateway process.
- `k6` installed and available in PATH.
- `go` toolchain available for `go tool pprof`.

## Enable pprof

Run gateway with:

```bash
GOCLAW_PPROF_ADDR=127.0.0.1:6060 ./goclaw gateway
```

## Quick Start

```bash
make bench-baseline
make bench-pprof
```

Artifacts are saved under `benchmarks/results/<timestamp>/`.

## Main environment variables

- `BASE_URL` (default `http://127.0.0.1:8080`)
- `PPROF_URL` (default `http://127.0.0.1:6060`)
- `GATEWAY_TOKEN` (optional; required if gateway token auth is enabled)
- `BENCH_SCENARIOS` (comma-separated: `health,tools_invoke_web_fetch`)
- `VUS` (default `20`)
- `DURATION` (default `60s`)
- `WEB_FETCH_TARGET` (default `https://example.com`)

## Compare two runs

```bash
make bench-compare BASE=benchmarks/results/<old_ts> NEW=benchmarks/results/<new_ts>
```

The compare report includes p50/p95 latency, req rate, failure rate, and a delta (%).
