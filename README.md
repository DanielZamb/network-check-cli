# netcheck

Agent-friendly network diagnostics CLI for macOS.

`netcheck` runs local and internet checks, scores results, prints human tables, and emits deterministic JSON/JSONL for automation.

## What It Checks

- local gateway health (loss/latency)
- internet reachability (loss, p95 RTT, jitter)
- DNS lookup timing
- HTTP/TLS timing
- path quality (`mtr`, with traceroute fallback)
- bandwidth (`speedtest-cli` and/or `iperf3`)
- bufferbloat delta (latency under load)

## Requirements

- macOS
- Go (build/install from source)
- tools on PATH:
  - `netstat`, `ping`, `ifconfig`, `dig`, `curl`, `openssl`
  - `mtr` (optional; traceroute fallback is used if `mtr` runtime fails)
  - `speedtest-cli` (optional if disabled in config)
  - `iperf3` (optional if disabled in config)

## Quick Start

1. Build:

```bash
make build
```

2. Install:

```bash
make install
```

3. Edit config:

```bash
$EDITOR netcheck.yaml
```

Set `bandwidth.iperf.target` to your receiver host/port, for example:

```yaml
bandwidth:
  iperf:
    enabled: true
    target: "192.168.40.29:5201"
```

4. Run:

```bash
netcheck run --config netcheck.yaml
```

## Commands

### `run`

One-shot run with summary and score.

```bash
netcheck run --config netcheck.yaml --format table
netcheck run --config netcheck.yaml --format json --out report.json
netcheck run --config netcheck.yaml --format both
netcheck run --config netcheck.yaml --verbose
netcheck run --config netcheck.yaml --select bandwidth,dns
netcheck run --config netcheck.yaml --skip path
```

Common flags:

- `--format table|json|jsonl|both`
- `--out <file>`
- `--verbose`, `--quiet`, `--no-color`
- `--timeout <sec>`
- `--fail-fast`
- `--strict-warn`
- `--select <groups>`, `--skip <groups>`
- `--id <run_id>`, `--labels key=value,key2=value2`

### `soak`

Repeated interval runs with JSONL event stream.

```bash
netcheck soak --config netcheck.yaml --interval 30 --duration 1800 --format jsonl --out soak.jsonl
```

Event types:

- `run_started`
- `check_result`
- `interval_summary`
- `run_summary`
- `run_finished`

### `compare`

Compare two JSON reports.

```bash
netcheck compare baseline.json candidate.json
netcheck compare --format json baseline.json candidate.json
```

### `man`

Built-in manuals (no system `man` required).

```bash
netcheck man
netcheck man run
netcheck man soak
netcheck man compare
netcheck man config
netcheck man exit-codes
netcheck man json-schema
netcheck man --export ./dist/man
```

## Exit Codes

- `0` all pass, or warn-only (unless strict warn mode)
- `1` at least one failing check
- `2` config/usage error
- `3` runtime/internal error
- `4` output/serialization error

## Installation Script

Install script is at `scripts/install.sh`.

Examples:

```bash
bash ./scripts/install.sh
bash ./scripts/install.sh --bin-dir "$HOME/bin"
bash ./scripts/install.sh --source-bin ./bin/netcheck --bin-dir "$HOME/.local/bin"
```

If destination is not in `PATH`, installer updates your shell rc file (`.zshrc`, `.bashrc`, or `.profile`) unless `--no-path-update` is set.

## Development

```bash
make help
make build
make test
make coverage
make fmt
make vet
make clean
```

## Troubleshooting

- `bandwidth.iperf` fails:
  - confirm receiver is running and reachable
  - confirm config `bandwidth.iperf.target` is `host:port`
- `path` fails with `mtr-packet` socket errors:
  - macOS may block raw socket access for `mtr`
  - tool falls back to traceroute metrics
- `speedtest` fails even with output:
  - parse may succeed but thresholds can still fail/warn (for example low upload vs `expected_plan`)
- checks timing out:
  - increase `--timeout` and/or `per_check_timeout_sec`

