# RFGuard

NFC/RFID Security System

RFGuard is a vendor-agnostic, passive NFC brute-force and anomaly detection engine. It ingests NFC reader event streams and log sources, normalizes them into a unified schema, and detects brute-force and anomalous reader behavior in real time.

## Key Features

- Hardware-agnostic, vendor-independent
- Passive monitoring only (no reader interference)
- Real-time sliding window detection (1s/10s/60s)
- Log ingestion: syslog, file tail, JSON/CSV/plain text parsing
- REST ingestion endpoint
- TCP JSON stream ingestion (newline-delimited)
- Optional Kafka adapter
- Structured JSON alerts
- Access-control policy checks (whitelist/blacklist)
- Partial hot-reload (detection + access control)
- Optional SQLite/PostgreSQL persistence

## Architecture

Input adapters → Normalization → Sliding windows per reader → Pattern recognition & rules → Alerts/metrics storage + API

## Quick Start

```bash
go version # requires Go 1.25+
go build -o rfguard ./cmd/rfguard
go build -o loggen ./cmd/loggen
./rfguard -config config/example.yaml
```

### Ingest via REST

```bash
curl -X POST http://localhost:8080/events \
  -H 'Content-Type: application/json' \
  -d '{"timestamp":"2026-02-23T12:34:56Z","reader":"reader01","card":"04AABBCC","status":"denied","error":"AUTH_FAIL"}'
```

### Tail a Log File

Update `config/example.yaml`:

```yaml
ingest:
  file_tail:
    enabled: true
    files:
      - "./nfc.log"
```

### TCP JSON Stream

Enable TCP stream in `config/example.yaml`:

```yaml
ingest:
  tcp_stream:
    enabled: true
    addr: ":9000"
```

```bash
nc localhost 9000 <<'EOF'
{"timestamp":"2026-02-23T12:34:56Z","reader":"reader01","card":"04AABBCC","status":"denied"}
EOF
```

## API

- `GET /status`
- `GET /metrics/{reader_id}`
- `GET /metrics`
- `GET /alerts`
- `GET /config/access_control`
- `POST /config/access_control`
- `POST /admin/clear`
- `POST /admin/restart`
- `GET /ui/`
- `GET /ui/access.html`
- `GET /ui/alerts.html`

See `docs/api.md` for details.

## Configuration

All thresholds are configurable via YAML/JSON and support hot reload. See `config/example.yaml` for a full example.

Access-control rules are passive. When enabled, RFGuard will emit alerts for blacklisted UIDs and for non‑whitelisted UIDs if `whitelist_only` is set.

Access-control changes made from the UI are applied immediately and persisted back to the active config file.

## Detection Logic (Pattern Recognition)

RFGuard normalizes all inputs into:

```
NormalizedEvent {
  timestamp (UTC)
  reader_id (string)
  uid (string, nullable)
  result (success/failure)
  error_code (optional)
}
```

### Sliding Window Metrics (per reader)

Computed for 1s / 10s / 60s windows:

- **Attempts per second (APS)**: `attempts / window_duration`
- **Failure ratio (FR)**: `failures / attempts`
- **UID diversity score (UDS)**: `unique_uids / attempts`
- **Timing variance (TV)**: variance of inter‑arrival `Δt` between events

### Rules

1. **Excessive Attempt Rate**  
   Trigger when `APS > aps_threshold`.

2. **Failure Spike**  
   Trigger when `FR > failure_ratio_threshold` **and** `attempts >= min_attempts`.

3. **UID Spraying / Enumeration**  
   Trigger when `UDS > uid_diversity_threshold` **and** `APS > aps_elevated_threshold`.

4. **Machine‑Like Timing**  
   Trigger when `TV < timing_variance_threshold` **and** `APS > aps_elevated_threshold`.

5. **Composite Attack Score**  
   ```
   AttackScore =
     w1 * APS +
     w2 * FR +
     w3 * UDS +
     w4 * (1 / (TV + epsilon))
   ```
   Trigger when:
   - `AttackScore > attack_score_threshold`
   - `attempts >= min_attempts`
   - `APS > aps_elevated_threshold`

6. **Repeated Auth Failure (per UID)**  
   Trigger when the **same UID on the same reader** produces **two or more consecutive** events with:
   - `result = failure` **and**
   - `error_code` present (e.g., `AUTH_FAIL`)

7. **Access Control Policies**  
   - **Blacklisted UID**: alert when UID is in `access_control.blacklist` or per‑reader blacklist.  
   - **Whitelist Only**: alert when UID is **not** in whitelist while `whitelist_only: true`.

### Parsing & Normalization Heuristics

- Supports JSON, CSV, and plain text logs (including syslog‑style timestamps).  
- Timestamp parsing: RFC3339, `YYYY‑MM‑DD HH:MM:SS`, syslog (`Jan 02 15:04:05`), and numeric UNIX seconds/milliseconds.  
- Result inference uses explicit `result/status` fields or any `error_code` as failure.

### Edge‑Case Handling

- **Out‑of‑order timestamps**: clamped within `max_clock_skew` / `max_future_skew`.  
- **Duplicate events**: suppressed within `dedupe_window`.  
- **Missing UID**: UID‑dependent rules (spray, access control) safely skip.

## Log Generator

```bash
go build -o loggen ./cmd/loggen
./loggen -format json -pattern mixed -rate 50 -follow
```

`loggen` appends to `./nfc.log` by default, which matches `config/example.yaml` for live tailing.

## Web UI

Start RFGuard and open the console:

```
http://localhost:8081/ui/
```

The UI shows live status, metrics, and alerts. Access control is available at:

```
http://localhost:8081/ui/access.html
```

Alerts detail view:

```
http://localhost:8081/ui/alerts.html
```

Admin controls are available on the Access Control page to clear metrics, clear alerts, or restart the engine state.

## Tests

```bash
go test ./...
```

## Docker

```bash
docker build -t rfguard .
docker run --rm -p 8080:8080 -p 8081:8081 -v $(pwd)/config:/config rfguard -config /config/example.yaml
```

## Notes

- Out-of-order timestamps are clamped within configured skew thresholds before windowing.
- Duplicate events are suppressed within `detection.dedupe_window`.
- Log parsing supports JSON/CSV/plain text and common syslog timestamp formats.
