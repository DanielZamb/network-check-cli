# netcheck soak

Run checks periodically and emit JSONL events.

## Flags
- `--interval`
- `--duration`
- `--format jsonl|both|table`
- `--timeout`

If `--interval` or `--duration` are omitted, values come from config (`soak.interval_sec`, `soak.duration_sec`).
