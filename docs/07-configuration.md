# Configuration Reference

All configuration can be provided via YAML config file, environment variables (`SHUNT_` prefix), or CLI flags. Environment variables use `_` as a nesting separator (e.g. `nats.consumers.workerCount` → `SHUNT_NATS_CONSUMERS_WORKERCOUNT`).

## NATS Connection

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `nats.urls` | `SHUNT_NATS_URLS` | `[]string` | `["nats://localhost:4222"]` | NATS server URLs. Env var accepts comma-separated values. |
| `nats.username` | `SHUNT_NATS_USERNAME` | `string` | | Username authentication |
| `nats.password` | `SHUNT_NATS_PASSWORD` | `string` | | Password authentication |
| `nats.token` | `SHUNT_NATS_TOKEN` | `string` | | Token authentication |
| `nats.nkey` | `SHUNT_NATS_NKEY` | `string` | | NKey seed authentication |
| `nats.credsFile` | `SHUNT_NATS_CREDSFILE` | `string` | | Path to JWT credentials file |
| `nats.tls.enable` | `SHUNT_NATS_TLS_ENABLE` | `bool` | `false` | Enable TLS |
| `nats.tls.certFile` | `SHUNT_NATS_TLS_CERTFILE` | `string` | | Client certificate path |
| `nats.tls.keyFile` | `SHUNT_NATS_TLS_KEYFILE` | `string` | | Client key path |
| `nats.tls.caFile` | `SHUNT_NATS_TLS_CAFILE` | `string` | | CA certificate path |
| `nats.tls.insecure` | `SHUNT_NATS_TLS_INSECURE` | `bool` | `false` | Skip TLS verification |
| `nats.connection.maxReconnects` | `SHUNT_NATS_CONNECTION_MAXRECONNECTS` | `int` | `-1` | Max reconnection attempts (-1 = unlimited) |
| `nats.connection.reconnectWait` | `SHUNT_NATS_CONNECTION_RECONNECTWAIT` | `duration` | `50ms` | Delay between reconnection attempts |

Only one authentication method may be used at a time.

## JetStream Consumers

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `nats.consumers.consumerPrefix` | `SHUNT_NATS_CONSUMERS_CONSUMERPREFIX` | `string` | `shunt` | Prefix for JetStream consumer names |
| `nats.consumers.workerCount` | `SHUNT_NATS_CONSUMERS_WORKERCOUNT` | `int` | `2` | Concurrent workers per subscription (max: 1000) |
| `nats.consumers.fetchBatchSize` | `SHUNT_NATS_CONSUMERS_FETCHBATCHSIZE` | `int` | `1` | Messages per pull request (max: 10000) |
| `nats.consumers.fetchTimeout` | `SHUNT_NATS_CONSUMERS_FETCHTIMEOUT` | `duration` | `5s` | Max wait time when fetching messages |
| `nats.consumers.maxAckPending` | `SHUNT_NATS_CONSUMERS_MAXACKPENDING` | `int` | `1000` | Max unacknowledged messages (max: 100000) |
| `nats.consumers.ackWaitTimeout` | `SHUNT_NATS_CONSUMERS_ACKWAITTIMEOUT` | `duration` | `30s` | Time before redelivery of unacked messages |
| `nats.consumers.maxDeliver` | `SHUNT_NATS_CONSUMERS_MAXDELIVER` | `int` | `3` | Max redelivery attempts |
| `nats.consumers.deliverPolicy` | `SHUNT_NATS_CONSUMERS_DELIVERPOLICY` | `string` | `new` | `all`, `new`, `last`, `by_start_time`, `by_start_sequence` |
| `nats.consumers.replayPolicy` | `SHUNT_NATS_CONSUMERS_REPLAYPOLICY` | `string` | `instant` | `instant` or `original` |

## Publish

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `nats.publish.mode` | `SHUNT_NATS_PUBLISH_MODE` | `string` | `jetstream` | `jetstream` (durable) or `core` (fire-and-forget) |
| `nats.publish.ackTimeout` | `SHUNT_NATS_PUBLISH_ACKTIMEOUT` | `duration` | `5s` | JetStream publish ack timeout |
| `nats.publish.maxRetries` | `SHUNT_NATS_PUBLISH_MAXRETRIES` | `int` | `3` | Max publish retry attempts |
| `nats.publish.retryBaseDelay` | `SHUNT_NATS_PUBLISH_RETRYBASEDELAY` | `duration` | `50ms` | Base delay for exponential backoff |

## KV Store

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `kv.enabled` | `SHUNT_KV_ENABLED` | `bool` | `false` | Enable KV data enrichment |
| `kv.buckets` | — | `[]string` | `[]` | KV buckets to watch (requires config file for lists) |
| `kv.localCache.enabled` | `SHUNT_KV_LOCALCACHE_ENABLED` | `bool` | `true` (when kv.enabled) | Enable in-memory KV cache |

## Rules

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `rules.kvBucket` | `SHUNT_RULES_KVBUCKET` | `string` | `rules` | KV bucket for rule definitions |

## Security

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `security.verification.enabled` | `SHUNT_SECURITY_VERIFICATION_ENABLED` | `bool` | `false` | Enable NKey signature verification |
| `security.verification.publicKeyHeader` | `SHUNT_SECURITY_VERIFICATION_PUBLICKEYHEADER` | `string` | `Nats-Public-Key` | Header containing signer's public key |
| `security.verification.signatureHeader` | `SHUNT_SECURITY_VERIFICATION_SIGNATUREHEADER` | `string` | `Nats-Signature` | Header containing Ed25519 signature |

## Logging

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `logging.level` | `SHUNT_LOGGING_LEVEL` | `string` | `info` | `debug`, `info`, `warn`, `error` |
| `logging.encoding` | `SHUNT_LOGGING_ENCODING` | `string` | `json` | `json` or `console` |
| `logging.outputPath` | `SHUNT_LOGGING_OUTPUTPATH` | `string` | `stdout` | Output destination |

## Metrics

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `metrics.enabled` | `SHUNT_METRICS_ENABLED` | `bool` | `false` | Enable Prometheus metrics endpoint |
| `metrics.address` | `SHUNT_METRICS_ADDRESS` | `string` | `:2112` | Metrics server listen address |
| `metrics.path` | `SHUNT_METRICS_PATH` | `string` | `/metrics` | Metrics endpoint path |
| `metrics.updateInterval` | `SHUNT_METRICS_UPDATEINTERVAL` | `string` | `15s` | System metrics update interval |

## HTTP Gateway

These settings apply when `gateway.enabled` is `true`.

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `gateway.enabled` | `SHUNT_GATEWAY_ENABLED` | `bool` | `false` | Enable HTTP gateway subsystem |
| `http.server.address` | `SHUNT_HTTP_SERVER_ADDRESS` | `string` | `:8080` | HTTP server listen address |
| `http.server.readTimeout` | `SHUNT_HTTP_SERVER_READTIMEOUT` | `duration` | `30s` | HTTP read timeout |
| `http.server.writeTimeout` | `SHUNT_HTTP_SERVER_WRITETIMEOUT` | `duration` | `30s` | HTTP write timeout |
| `http.server.idleTimeout` | `SHUNT_HTTP_SERVER_IDLETIMEOUT` | `duration` | `120s` | HTTP idle timeout |
| `http.server.maxHeaderBytes` | `SHUNT_HTTP_SERVER_MAXHEADERBYTES` | `int` | `1048576` | Max header size (1MB) |
| `http.server.shutdownGracePeriod` | `SHUNT_HTTP_SERVER_SHUTDOWNGRACEPERIOD` | `duration` | `30s` | Graceful shutdown timeout |
| `http.server.inboundWorkerCount` | `SHUNT_HTTP_SERVER_INBOUNDWORKERCOUNT` | `int` | `10` | Workers processing inbound HTTP requests |
| `http.server.inboundQueueSize` | `SHUNT_HTTP_SERVER_INBOUNDQUEUESIZE` | `int` | `1000` | Inbound request queue size (max: 100000) |
| `http.client.timeout` | `SHUNT_HTTP_CLIENT_TIMEOUT` | `duration` | `30s` | Outbound HTTP request timeout |
| `http.client.maxIdleConns` | `SHUNT_HTTP_CLIENT_MAXIDLECONNS` | `int` | `100` | Max idle connections |
| `http.client.maxIdleConnsPerHost` | `SHUNT_HTTP_CLIENT_MAXIDLECONNSPERHOST` | `int` | `10` | Max idle connections per host |
| `http.client.idleConnTimeout` | `SHUNT_HTTP_CLIENT_IDLECONNTIMEOUT` | `duration` | `90s` | Idle connection timeout |
| `http.client.tls.insecureSkipVerify` | `SHUNT_HTTP_CLIENT_TLS_INSECURESKIPVERIFY` | `bool` | `false` | Skip outbound TLS verification |

## Auth Manager

These settings apply when `authManager.enabled` is `true`.

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `authManager.enabled` | `SHUNT_AUTHMANAGER_ENABLED` | `bool` | `false` | Enable auth manager subsystem |
| `authManager.storage.bucket` | `SHUNT_AUTHMANAGER_STORAGE_BUCKET` | `string` | `tokens` | KV bucket for token storage |
| `authManager.storage.keyPrefix` | `SHUNT_AUTHMANAGER_STORAGE_KEYPREFIX` | `string` | | Key prefix in token bucket |

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

| Config Key | Env Var | Type | Default | Description |
|---|---|---|---|---|
| `forEach.maxIterations` | `SHUNT_FOREACH_MAXITERATIONS` | `int` | `100` | Max iterations per forEach operation (max: 10000) |
