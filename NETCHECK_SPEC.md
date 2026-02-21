# netcheck v1 Specification

## 1) Platform
- macOS only for v1.

## 2) Commands
- `netcheck run`
- `netcheck soak`
- `netcheck compare <baseline.json> <candidate.json>`

## 3) Core Goals
1. Run a full internet health check from Unix tools.
2. Show real-time verbose output for humans.
3. Emit machine-readable JSON for monitoring/automation.
4. Produce a final summarized table with pass/warn/fail.

## 4) Check Groups
1. Local network
- Interface up/down, local IP, default gateway detection.
- Gateway ping stats.

2. Reachability
- Ping public targets (default: `1.1.1.1`, `8.8.8.8`, configurable).

3. DNS
- Resolution through system resolver.
- Resolution through explicit resolvers where configured.
- Query time capture and failure tracking.

4. Path and routing
- `mtr` and/or traceroute summary.
- Hop count and loss near destination.

5. HTTP/TLS
- `curl` timing metrics (`dns`, `connect`, `tls`, `ttfb`, `total`).
- Optional TLS handshake metadata.

6. Bandwidth
- `speedtest-cli` check (attempt by default).
- `iperf3` check when target is configured.
- If `iperf3` target is missing/unreachable, mark as `skip` with reason.

7. Bufferbloat proxy
- Compare idle latency versus latency during load.
- Record loaded-latency delta.

## 5) Bandwidth Target Definition
- `iperf3` target is a remote machine running `iperf3 -s`.
- Not localhost/self-loop for ISP validation.
- Supported target examples:
  - LAN host for local bottleneck isolation.
  - Remote host (VPS/home server) for ISP path validation.

## 6) Output Formats
- Human-readable table.
- JSON (single final report object).
- JSONL stream (line-delimited events, especially for soak).

Recommended behavior:
- `run`: table + optional single JSON report.
- `soak`: JSONL stream + optional final aggregate summary.

## 7) Result Model (JSON)
Top-level fields:
- `timestamp`
- `host`
- `os`
- `version`
- `config`
- `checks[]`
- `summary`
- `score`

Per-check fields:
- `id`
- `group`
- `target`
- `status` (`pass|warn|fail|skip`)
- `metrics`
- `raw`
- `duration_ms`
- `error`

## 8) Status and Score
- Per-check status: `pass|warn|fail|skip`.
- Overall health score: `0-100`.

Weighting:
- Reliability (loss/outage): `35`
- Latency/Jitter: `25`
- DNS: `10`
- HTTP/TLS responsiveness: `10`
- Throughput (vs expected): `20`

## 9) Default Thresholds
- Packet loss:
  - pass `< 0.5%`
  - warn `0.5% - 2%`
  - fail `> 2%`

- Idle RTT p95:
  - pass `< 40ms`
  - warn `40ms - 80ms`
  - fail `> 80ms`

- Jitter:
  - pass `< 10ms`
  - warn `10ms - 25ms`
  - fail `> 25ms`

- DNS query time:
  - pass `< 50ms`
  - warn `50ms - 120ms`
  - fail `> 120ms`

- HTTP total time:
  - pass `< 800ms`
  - warn `800ms - 2000ms`
  - fail `> 2000ms`

- Loaded latency delta:
  - pass `< 30ms`
  - warn `30ms - 80ms`
  - fail `> 80ms`

- Throughput versus expected plan:
  - pass `>= 80%`
  - warn `60% - 79%`
  - fail `< 60%`

## 10) Tooling Strategy
- Measurement engines use external tools:
  - `ping`
  - `dig`
  - `mtr`
  - `curl`
  - `speedtest-cli`
  - `iperf3`

- Parser and orchestration implemented in Go.
- Graceful degradation:
  - missing dependency -> `skip` + explicit reason.

## 11) Soak Mode
- Interval-based repeated checks.
- Emit JSONL event per interval/check.
- Optional final summary object for aggregate metrics.

## 12) Testing Policy (Required for Every Feature)
Every feature must include tests at implementation time.

1. Unit tests
- Parsers for tool outputs.
- Threshold and status evaluation.
- Score calculation and reducers.

2. Golden tests
- Final table rendering snapshots.
- JSON output snapshots.

3. Integration tests (deterministic)
- Runner with fake executor.
- Simulate success/failure/timeout/missing tools.

4. Optional real-tool integration tests
- Behind build tag (example: `-tags=integration`).

## 13) Non-Goals for v1
- Linux support (deferred to v2).
- GUI dashboards in-process.
- Full synthetic traffic generation beyond selected tools.
