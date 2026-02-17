# KV Enrichment

Rules evaluate one message at a time, but sometimes a routing decision depends on data that isn't in the message — customer tiers, device thresholds, feature flags. KV enrichment lets rules pull that context from NATS KV stores during evaluation. NATS KV is a persistent, replicated key-value store built into JetStream. Use it for slow-changing reference data (configuration, permissions, flags), not for per-message transactional state.

## Setup

### Creating buckets

KV buckets must exist before Shunt starts. If a configured bucket is missing, startup fails with a clear error:

```
configured KV bucket not found: 'device_config'. Please create it before starting the application using 'nats kv add device_config'
```

Create buckets using the NATS CLI:

```bash
nats kv add device_config
nats kv add feature_flags
nats kv add user_permissions
```

### Loading data

Values should be JSON objects so you can access individual fields with path syntax:

```bash
nats kv put device_config sensor-001 '{
  "max_temp": 35,
  "location": "building-a",
  "active": true
}'

nats kv put feature_flags dark_mode '{
  "enabled": true,
  "rollout_percent": 50
}'
```

### Configuring Shunt

Add the `kv` block to your config file:

```yaml
kv:
  enabled: true
  buckets:
    - device_config
    - feature_flags
    - user_permissions
  localCache:
    enabled: true
```

The `kv.buckets` field requires a config file — you cannot pass a list via a single environment variable. See the [Configuration Reference](./07-configuration.md#kv-store) for the full option table.

## Syntax quick reference

| Form | Example | Description |
|------|---------|-------------|
| Whole value | `{@kv.bucket.key}` | Returns entire JSON object |
| Field access | `{@kv.bucket.key:field}` | Returns a single field |
| Nested field | `{@kv.bucket.key:nested.field}` | Dot-separated JSON path after colon |
| Dynamic key from message | `{@kv.bucket.{msg_field}:field}` | Key resolved from message payload |
| Dynamic key from subject | `{@kv.bucket.{@subject.1}:field}` | Key resolved from NATS subject token |

**The colon rule:** everything before `:` is the key (dots included), everything after `:` is the JSON path into the value. This is the most important syntax detail — without the colon, dots in the key name would be ambiguous.

```
@kv.device_config.sensor.temp.001:thresholds.max
     └─ bucket ─┘ └──── key ────┘ └─ json path ─┘
```

See the [System Variables Reference](./02-system-variables.md#key-value-store) for the full syntax specification.

## How the cache works

### Watch-based, not TTL

Shunt does not poll or expire entries on a timer. It uses the NATS KV Watch API (`store.Watch(ctx, ">")`) which delivers all current entries at startup, then streams real-time updates as they happen. Deletes and purges are also reflected immediately.

### Read path

Every KV lookup follows this path:

1. **Local cache** — in-memory map lookup, no network hop
2. **Cache miss** — falls back to a direct NATS `Get` with a 5-second timeout
3. **Lazy-populate** — on a successful direct read, the value is written back into the local cache for future lookups

### High availability

Each Shunt instance watches KV buckets independently. NATS handles replication across the cluster. No coordination is needed between Shunt instances.

### Disabling the cache

When `kv.enabled` is true, the local cache is always enabled — the conditional default in configuration forces `localCache.enabled: true`. To bypass the cache for debugging, set the log level to `debug` and watch for cache hit/miss messages, or use `nats kv get` to verify values directly in the bucket.

### Summary

| Aspect | Behavior |
|--------|----------|
| Consistency model | Eventually consistent (watch-based) |
| Staleness window | Milliseconds (NATS push latency) |
| Cache miss behavior | Direct NATS Get, 5s timeout, then lazy-populates cache |
| Key not found | Returns empty value, does not error |
| HA | Each instance watches independently; NATS replicates |

## Patterns

### Dynamic configuration

Per-device thresholds — compare a sensor reading against its device-specific maximum stored in KV.

```bash
nats kv put device_config sensor-001 '{"max_temp": 35, "location": "building-a"}'
```

```yaml
- trigger:
    nats:
      subject: "sensors.temperature.>"
  conditions:
    operator: and
    items:
      - field: "{temperature}"
        operator: gt
        value: "{@kv.device_config.{@subject.2}:max_temp}"
  action:
    nats:
      subject: "alerts.temperature"
      payload: |
        {
          "sensor": "{@subject.2}",
          "temperature": {temperature},
          "threshold": "{@kv.device_config.{@subject.2}:max_temp}",
          "location": "{@kv.device_config.{@subject.2}:location}"
        }
```

### Feature flags

Enable or disable processing paths without redeployment. Toggle the flag in KV and every instance picks up the change in milliseconds.

```bash
nats kv put feature_flags new_pipeline '{"enabled": true, "target": "premium"}'
```

```yaml
- trigger:
    nats:
      subject: "events.incoming"
  conditions:
    operator: and
    items:
      - field: "{@kv.feature_flags.new_pipeline:enabled}"
        operator: eq
        value: true
      - field: "{account_type}"
        operator: eq
        value: "{@kv.feature_flags.new_pipeline:target}"
  action:
    nats:
      subject: "pipeline.v2.process"
      payload: |
        {
          "event": {event_id},
          "account": "{account_type}",
          "routed_at": "{@timestamp()}"
        }
```

### Access control

Check user permissions before routing sensitive messages.

```bash
nats kv put user_permissions user-42 '{"role": "admin", "can_export": true}'
```

```yaml
- trigger:
    nats:
      subject: "api.export-request"
  conditions:
    operator: and
    items:
      - field: "{@kv.user_permissions.{user_id}:role}"
        operator: eq
        value: "admin"
      - field: "{@kv.user_permissions.{user_id}:can_export}"
        operator: eq
        value: true
  action:
    nats:
      subject: "exports.authorized"
      payload: |
        {
          "user_id": "{user_id}",
          "role": "{@kv.user_permissions.{user_id}:role}",
          "export_id": "{@uuid7()}"
        }
```

### Keys with dots

When the key name itself contains dots, the colon delimiter makes it unambiguous. The key is everything between the bucket name and the colon. Subject tokens can also be used as keys.

```bash
nats kv put logs 'api.auth.v2' '{"error_rate": 3.5, "p99_ms": 890}'
```

```yaml
- trigger:
    nats:
      subject: "monitoring.service-health"
  conditions:
    operator: and
    items:
      - field: "{@kv.logs.{service_name}:error_rate}"
        operator: gt
        value: 1.0
  action:
    nats:
      subject: "alerts.service-errors"
      payload: |
        {
          "service": "{service_name}",
          "error_rate": "{@kv.logs.{service_name}:error_rate}",
          "p99_ms": "{@kv.logs.{service_name}:p99_ms}"
        }
```

## Best practices

- Create buckets before starting Shunt — use an init container or startup script
- One bucket per data domain (devices, users, flags) to keep things organized
- Store JSON objects even if you only need one field today — adding fields later requires no rule changes
- Keep values small (kilobytes, not megabytes) — every value is held in memory when the cache is enabled
- Use `nats kv get bucket key` to verify data exists before debugging rules

## Anti-patterns

| Anti-pattern | Consequence |
|-------------|-------------|
| Using KV for per-message counters or session state | Values are eventually consistent; concurrent updates will conflict |
| Referencing a bucket not listed in `kv.buckets` config | Silent empty return — logged as a warning with available buckets |
| Storing large blobs (MB+) | All values cached in memory; risks OOM |
| Assuming a missing key will error | Returns empty value, rule silently uses `""` |
| Putting JSON path before the colon (`@kv.bucket.key.field`) | Treats `key.field` as the key name, not `key` with path `field` |

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Rule with KV condition never fires | Bucket not listed in `kv.buckets` config | Add bucket to config and restart |
| Action payload has empty KV fields | Key doesn't exist in the bucket | Verify with `nats kv get bucket key` |
| Startup fails with "bucket not found" | KV bucket not created | Run `nats kv add bucket_name` |
| Value not reflecting recent update | Watch goroutine may not be running | Enable `debug` logging and look for `"KV watcher initial sync complete"` |
| Dynamic key returns empty | Message field used in `{field}` is missing | Enable debug logging to see resolved field values |
| Non-JSON value returns empty with path | Raw string values don't support JSON path | Store values as JSON objects |
