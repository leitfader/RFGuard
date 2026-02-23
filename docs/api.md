# RFGuard API

## GET /status

Returns basic health and runtime information.

Response:

```json
{
  "status": "ok",
  "time": "2026-02-23T12:34:56Z",
  "version": "0.1.0",
  "config_path": "/path/to/config.yaml",
  "access_control": {
    "enabled": true,
    "whitelist_only": false,
    "whitelist": ["04AABBCC"],
    "blacklist": ["DEADBEEF"],
    "reader_whitelists": {
      "reader01": ["A1B2C3D4"]
    },
    "reader_blacklists": {
      "reader02": ["BAD0BAD0"]
    }
  },
  "ingest": {
    "rest": true,
    "syslog": true,
    "file_tail": true,
    "tcp_stream": false,
    "kafka": false
  },
  "api": {
    "enabled": true,
    "addr": ":8081"
  },
  "detection": {
    "windows": ["1s", "10s", "60s"]
  }
}
```

## GET /metrics/{reader_id}

Returns the latest computed metrics for a reader across all windows.

Response:

```json
{
  "reader_id": "reader01",
  "updated_at": "2026-02-23T12:34:56Z",
  "metrics": [
    {
      "window_sec": 1,
      "attempts": 12,
      "failures": 10,
      "aps": 12.0,
      "fr": 0.83,
      "uds": 0.66,
      "tv": 0.01
    }
  ]
}
```

## GET /metrics

Returns metrics for all readers.

Response:

```json
{
  "metrics": {
    "reader01": [
      {
        "window_sec": 1,
        "attempts": 12,
        "failures": 10,
        "aps": 12.0,
        "fr": 0.83,
        "uds": 0.66,
        "tv": 0.01
      }
    ]
  },
  "count": 1
}
```

## GET /alerts

Returns recent alerts (in-memory buffer).

Query parameters:

- `limit`: maximum number of alerts returned

Response:

```json
{
  "alerts": [
    {
      "timestamp": "2026-02-23T12:34:56Z",
      "reader_id": "reader01",
      "severity": "high",
      "alert_type": "possible_bruteforce",
      "window_sec": 10,
      "metrics": {
        "window_sec": 10,
        "attempts": 200,
        "failures": 190,
        "aps": 20.0,
        "fr": 0.95,
        "uds": 0.90,
        "tv": 0.001
      },
      "score": 175.0,
      "rules": ["excessive_attempt_rate", "failure_spike", "attack_score"],
      "context": {
        "engine": "rfguard"
      }
    }
  ],
  "count": 1
}
```

## GET /config/access_control

Returns access control policy.

Response:

```json
{
  "access_control": {
    "enabled": true,
    "whitelist_only": false,
    "whitelist": ["04AABBCC"],
    "blacklist": ["DEADBEEF"],
    "reader_whitelists": {
      "reader01": ["A1B2C3D4"]
    },
    "reader_blacklists": {
      "reader02": ["BAD0BAD0"]
    }
  }
}
```

## POST /config/access_control

Updates access control policy and writes it to the active config file.

Request:

```json
{
  "enabled": true,
  "whitelist_only": true,
  "whitelist": ["04AABBCC"],
  "blacklist": ["DEADBEEF"],
  "reader_whitelists": {
    "reader01": ["A1B2C3D4"]
  },
  "reader_blacklists": {
    "reader02": ["BAD0BAD0"]
  }
}
```

## POST /admin/clear

Clears in-memory metrics or alerts.

Request:

```json
{
  "target": "metrics" // alerts | metrics | all
}
```

## POST /admin/restart

Resets in-memory engine state (windows, cooldowns, dedupe), metrics, and alerts.

## Web UI

- `GET /ui/` serves the live console.
- `GET /ui/access.html` serves access control and admin actions.
- `GET /ui/alerts.html` serves the alert detail view.
