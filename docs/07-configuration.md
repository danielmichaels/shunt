# Configuration Reference

Configuration is provided via a YAML config file. The `serve` command exposes a subset of commonly-used options as CLI flags, each with a corresponding `SHUNT_`-prefixed environment variable. CLI flags override config file values.

Settings without a CLI flag can only be set in the YAML config file.

## CLI Flag Overrides

These flags (and their env vars) are available on the `serve` command and override the corresponding config file values:

| CLI Flag | Env Var | Config Key | Default | Description |
|---|---|---|---|---|
| `-c` / `--config` | `SHUNT_CONFIG` | — | `shunt.yaml` | Path to config file |
| `--nats-url` | `SHUNT_NATS_URL` | `nats.urls` | `nats://localhost:4222` | NATS server URLs (comma-separated) |
| `--log-level` | `SHUNT_LOG_LEVEL` | `logging.level` | `info` | `debug`, `info`, `warn`, `error` |
| `--metrics-enabled` | `SHUNT_METRICS_ENABLED` | `metrics.enabled` | `true` | Enable Prometheus metrics endpoint |
| `--metrics-addr` | `SHUNT_METRICS_ADDR` | `metrics.address` | `:2112` | Metrics server listen address |
| `--metrics-path` | `SHUNT_METRICS_PATH` | `metrics.path` | `/metrics` | Metrics endpoint path |
| `--gateway-enabled` | `SHUNT_GATEWAY_ENABLED` | `gateway.enabled` | `false` | Enable HTTP gateway subsystem |
| `--kv-enabled` | `SHUNT_KV_ENABLED` | `kv.enabled` | `false` | Enable KV data enrichment |
| `--worker-count` | `SHUNT_WORKER_COUNT` | `nats.consumers.workerCount` | `2` | Concurrent workers per subscription |

## NATS Connection

| Config Key | Type | Default | Description |
|---|---|---|---|
| `nats.urls` | `[]string` | `["nats://localhost:4222"]` | NATS server URLs |
| `nats.username` | `string` | | Username authentication |
| `nats.password` | `string` | | Password authentication |
| `nats.token` | `string` | | Token authentication |
| `nats.nkey` | `string` | | NKey seed file path |
| `nats.credsFile` | `string` | | Path to JWT credentials file |
| `nats.tls.enable` | `bool` | `false` | Enable TLS |
| `nats.tls.certFile` | `string` | | Client certificate path |
| `nats.tls.keyFile` | `string` | | Client key path |
| `nats.tls.caFile` | `string` | | CA certificate path |
| `nats.tls.insecure` | `bool` | `false` | Skip TLS verification |
| `nats.connection.maxReconnects` | `int` | `-1` | Max reconnection attempts (-1 = unlimited) |
| `nats.connection.reconnectWait` | `duration` | `50ms` | Delay between reconnection attempts |

Only one authentication method may be used at a time.

## JetStream Consumers

| Config Key | Type | Default | Description |
|---|---|---|---|
| `nats.consumers.consumerPrefix` | `string` | `shunt` | Prefix for JetStream consumer names |
| `nats.consumers.workerCount` | `int` | `2` | Concurrent workers per subscription (max: 1000). Also settable via `--worker-count` / `SHUNT_WORKER_COUNT`. |
| `nats.consumers.fetchBatchSize` | `int` | `1` | Messages per pull request (max: 10000) |
| `nats.consumers.fetchTimeout` | `duration` | `5s` | Max wait time when fetching messages |
| `nats.consumers.maxAckPending` | `int` | `1000` | Max unacknowledged messages (max: 100000) |
| `nats.consumers.ackWaitTimeout` | `duration` | `30s` | Time before redelivery of unacked messages |
| `nats.consumers.maxDeliver` | `int` | `3` | Max redelivery attempts |
| `nats.consumers.deliverPolicy` | `string` | `new` | `all`, `new`, `last`, `by_start_time`, `by_start_sequence` |
| `nats.consumers.replayPolicy` | `string` | `instant` | `instant` or `original` |

## Publish

| Config Key | Type | Default | Description |
|---|---|---|---|
| `nats.publish.mode` | `string` | `jetstream` | `jetstream` (durable) or `core` (fire-and-forget) |
| `nats.publish.ackTimeout` | `duration` | `5s` | JetStream publish ack timeout |
| `nats.publish.maxRetries` | `int` | `3` | Max publish retry attempts |
| `nats.publish.retryBaseDelay` | `duration` | `50ms` | Base delay for exponential backoff |

### Per-Rule Publish Mode Override

Each NATS action can override the global `nats.publish.mode` by setting `mode` directly on the action block. When omitted, the global setting is used. Valid values: `core` (fire-and-forget, low-latency) or `jetstream` (durable, acknowledged).

The right delivery guarantee depends on the action, not the input. A single consumed JetStream message can trigger multiple rules with different output guarantees.

#### Notifications to simple subscribers

Notification sidecars (ntfy, Gotify, Slack webhook agents) are typically plain core NATS subscribers — they don't consume from streams. Core publish works without provisioning a stream for the output subject.

```yaml
# Message: {"temperature": 47.3, "device_id": "sensor-12", "location": "warehouse-b"}
- trigger:
    nats:
      subject: sensors.temperature.>
  conditions:
    operator: and
    items:
      - field: "{temperature}"
        operator: gt
        value: 45
  action:
    nats:
      subject: notify.slack.alerts
      mode: core
      payload: '{"text": "High temp: {temperature}C on {device_id}"}'
```

#### Real-time dashboards and WebSocket bridges

Sensor data published to subjects a WebSocket gateway fans out to browsers. If no browser is connected, persisting these messages is pointless — the next reading supersedes it.

```yaml
- trigger:
    nats:
      subject: sensors.energy.>
  action:
    nats:
      subject: dashboard.energy.{@subject.2}
      mode: core
      passthrough: true
```

#### Fan-out with mixed guarantees

One consumed message triggers two rules: a durable audit log and an ephemeral live status display.

```yaml
# Message: {"door_id": "front", "event": "opened", "user": "alice"}
# Audit log — must persist
- trigger:
    nats:
      subject: access.door.>
  action:
    nats:
      subject: audit.access.log
      mode: jetstream
      payload: '{"door": "{door_id}", "action": "{event}", "time": "{@timestamp()}"}'

# Live status board — ephemeral
- trigger:
    nats:
      subject: access.door.>
  action:
    nats:
      subject: display.door.status.{door_id}
      mode: core
      payload: '{"door": "{door_id}", "state": "{event}"}'
```

#### Metrics re-publishing

Enriched telemetry re-published to a subject a metrics collector (Telegraf, Prometheus pushgateway) subscribes on. Missing a single data point is acceptable — the next one arrives in seconds.

```yaml
- trigger:
    nats:
      subject: zigbee2mqtt.>
  action:
    nats:
      subject: metrics.zigbee.{@subject.1}
      mode: core
      payload: '{"device": "{@subject.1}", "battery": {battery}, "linkquality": {linkquality}}'
```

#### Global core, selective durability

A homelab where most rules are simple forwarding (`nats.publish.mode: core` globally), but one rule publishes safety-critical events that must not be lost.

```yaml
# Global config: nats.publish.mode: core
# This rule overrides to jetstream for safety-critical events
- trigger:
    nats:
      subject: sensors.gas.>
  conditions:
    operator: and
    items:
      - field: "{ppm}"
        operator: gt
        value: 500
  action:
    nats:
      subject: safety.gas.alarm
      mode: jetstream
      payload: '{"sensor": "{sensor_id}", "ppm": {ppm}, "time": "{@timestamp()}"}'
```

## KV Store

| Config Key | Type | Default | Description |
|---|---|---|---|
| `kv.enabled` | `bool` | `false` | Enable KV data enrichment. Also settable via `--kv-enabled` / `SHUNT_KV_ENABLED`. |
| `kv.buckets` | `[]string` | `[]` | KV buckets to watch (config file only — lists cannot be set via env var) |
| `kv.autoProvision` | `bool` | `true` | Auto-create KV buckets if missing |
| `kv.localCache.enabled` | `bool` | `true` (when kv.enabled) | Enable in-memory KV cache |

## Rules

| Config Key | Type | Default | Description |
|---|---|---|---|
| `rules.kvBucket` | `string` | `rules` | KV bucket for rule definitions |

### Per-Rule Fields

These fields are set on individual rules in rule YAML files, not in the server config.

| Field | Type | Default | Description |
|---|---|---|---|
| `debounce` | `duration string` | *(disabled)* | Suppress rapid re-fires within a time window (e.g., `"30s"`, `"5m"`). See [Debounce](./01-core-concepts.md#4-debounce). |

## Security

| Config Key | Type | Default | Description |
|---|---|---|---|
| `security.verification.enabled` | `bool` | `false` | Enable NKey signature verification |
| `security.verification.publicKeyHeader` | `string` | `Nats-Public-Key` | Header containing signer's public key |
| `security.verification.signatureHeader` | `string` | `Nats-Signature` | Header containing Ed25519 signature |

## Logging

| Config Key | Type | Default | Description |
|---|---|---|---|
| `logging.level` | `string` | `info` | `debug`, `info`, `warn`, `error`. Also settable via `--log-level` / `SHUNT_LOG_LEVEL`. |
| `logging.encoding` | `string` | `json` | `json` or `console` |
| `logging.outputPath` | `string` | `stdout` | Output destination |

## Metrics

| Config Key | Type | Default | Description |
|---|---|---|---|
| `metrics.enabled` | `bool` | `true` | Enable Prometheus metrics endpoint. Also settable via `--metrics-enabled` / `SHUNT_METRICS_ENABLED`. |
| `metrics.address` | `string` | `:2112` | Metrics server listen address. Also settable via `--metrics-addr` / `SHUNT_METRICS_ADDR`. |
| `metrics.path` | `string` | `/metrics` | Metrics endpoint path. Also settable via `--metrics-path` / `SHUNT_METRICS_PATH`. |
| `metrics.updateInterval` | `string` | `15s` | System metrics update interval |

## HTTP Gateway

These settings apply when `gateway.enabled` is `true`. The gateway toggle is also settable via `--gateway-enabled` / `SHUNT_GATEWAY_ENABLED`.

| Config Key | Type | Default | Description |
|---|---|---|---|
| `gateway.enabled` | `bool` | `false` | Enable HTTP gateway subsystem |
| `http.server.address` | `string` | `:8080` | HTTP server listen address |
| `http.server.readTimeout` | `duration` | `30s` | HTTP read timeout |
| `http.server.writeTimeout` | `duration` | `30s` | HTTP write timeout |
| `http.server.idleTimeout` | `duration` | `120s` | HTTP idle timeout |
| `http.server.maxHeaderBytes` | `int` | `1048576` | Max header size (1MB) |
| `http.server.shutdownGracePeriod` | `duration` | `30s` | Graceful shutdown timeout |
| `http.server.inboundWorkerCount` | `int` | `10` | Workers processing inbound HTTP requests |
| `http.server.inboundQueueSize` | `int` | `1000` | Inbound request queue size (max: 100000) |
| `http.client.timeout` | `duration` | `30s` | Outbound HTTP request timeout |
| `http.client.maxIdleConns` | `int` | `100` | Max idle connections |
| `http.client.maxIdleConnsPerHost` | `int` | `10` | Max idle connections per host |
| `http.client.idleConnTimeout` | `duration` | `90s` | Idle connection timeout |
| `http.client.tls.insecureSkipVerify` | `bool` | `false` | Skip outbound TLS verification |

## Auth Manager

These settings apply when `authManager.enabled` is `true`. All auth manager settings require a config file.

| Config Key | Type | Default | Description |
|---|---|---|---|
| `authManager.enabled` | `bool` | `false` | Enable auth manager subsystem |
| `authManager.storage.bucket` | `string` | `tokens` | KV bucket for token storage |
| `authManager.storage.keyPrefix` | `string` | | Key prefix in token bucket |

Providers are configured as a list under `authManager.providers` and require a config file:

```yaml
authManager:
  enabled: true
  storage:
    bucket: tokens
  providers:
    - id: my-api
      type: oauth2           # or "custom-http"
      kvKey: my-api-token
      tokenUrl: https://auth.example.com/token
      clientId: my-client
      clientSecret: secret
      refreshBefore: 5m
      scopes: ["read", "write"]
```

## ForEach

| Config Key | Type | Default | Description |
|---|---|---|---|
| `forEach.maxIterations` | `int` | `100` | Max iterations per forEach operation (max: 10000) |
