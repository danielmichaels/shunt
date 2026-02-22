# Shunt Local Testing Tutorial

A graduated, hands-on tutorial for running Shunt locally. Each lesson builds on the previous one, introducing one new concept at a time.

## Prerequisites

```bash
brew install nats-server nats-io/nats-tools/nats
```

## Initial Setup (do this once)

### Terminal 1 — Start NATS with JetStream
```bash
nats-server --jetstream
```
Leave this running for the entire tutorial.

### Terminal 2 — Build Shunt
```bash
cd /path/to/rule-router
task build:shunt
```

---

## Lesson 1: Your First Rule (basic conditions + templated payload)

**Concepts:** trigger subject, conditions (AND), nested field access, payload templates, `@timestamp()`, `@uuid7()`

**Rule file:** `rules/router/basic.yaml`

This rule watches `sensors.data` and routes high bedroom temperatures to `alerts.bedroom-temperature`.

### Setup streams + KV + push rule

Streams are needed for trigger subjects (pull consumers require them) and for output subjects that publish via JetStream (the default mode). Both streams below are required — SENSORS covers the trigger and ALERTS covers the JetStream output.

```bash
nats stream add SENSORS --subjects "sensors.>" --defaults
nats stream add ALERTS  --subjects "alerts.>"  --defaults
nats kv add rules

./bin/shunt kv push rules/router/basic.yaml --nats-url nats://localhost:4222
./bin/shunt kv list --nats-url nats://localhost:4222
```

### Start Shunt (Terminal 3)
```bash
SHUNT_LOG_LEVEL=debug \
  ./bin/shunt serve --nats-url nats://localhost:4222
```

### Subscribe to output (Terminal 4)
```bash
nats sub "alerts.>"
```

### Test it (Terminal 2)
```bash
# MATCH: reading > 30 AND location == "bedroom"
nats pub sensors.data '{"sensor": {"id": "temp001", "reading": 32.5, "location": "bedroom"}}'

# NO MATCH: reading too low
nats pub sensors.data '{"sensor": {"id": "temp002", "reading": 22.0, "location": "bedroom"}}'

# NO MATCH: wrong location
nats pub sensors.data '{"sensor": {"id": "temp003", "reading": 35.0, "location": "kitchen"}}'
```

**What to observe:**
- Only the first message appears in Terminal 4
- The payload contains templated fields (`sensor.id`, `sensor.reading`) resolved from the message
- `@timestamp()` and `@uuid7()` are auto-generated
- Shunt debug logs show rule evaluation per message

### Cleanup before next lesson
```bash
./bin/shunt kv delete router.basic --nats-url nats://localhost:4222
```

---

## Lesson 2: Nested Condition Groups (AND/OR logic)

**New concepts:** condition `groups`, nested AND/OR operators, complex boolean logic

**Rule file:** `rules/router/advanced.yaml`

This rule triggers on `sensors.environment` and demonstrates nested condition groups:
- Top-level AND: `status == "active"` AND (one of the groups below)
  - OR group: `temperature > 32` OR (nested AND: `humidity > 85` AND `pressure < 990`)

### Setup + push
```bash
nats stream add ENVIRONMENT --subjects "sensors.environment" --defaults

./bin/shunt kv push rules/router/advanced.yaml --nats-url nats://localhost:4222
```

### Test it
```bash
# MATCH: status active + temperature > 32
nats pub sensors.environment '{"status": "active", "temperature": 35, "humidity": 60, "pressure": 1010}'

# MATCH: status active + humidity > 85 AND pressure < 990 (nested AND group)
nats pub sensors.environment '{"status": "active", "temperature": 25, "humidity": 90, "pressure": 980}'

# NO MATCH: status inactive
nats pub sensors.environment '{"status": "inactive", "temperature": 35, "humidity": 90, "pressure": 980}'

# NO MATCH: active but none of the OR conditions met
nats pub sensors.environment '{"status": "active", "temperature": 25, "humidity": 70, "pressure": 1010}'
```

**What to observe:** The first two messages match because the OR group needs only one branch to be true. The nested AND group requires both humidity AND pressure thresholds.

### Cleanup
```bash
./bin/shunt kv delete router.advanced --nats-url nats://localhost:4222
```

---

## Lesson 3: Wildcard Subjects + Subject Tokens

**New concepts:** `*` (single-level wildcard), `>` (multi-level wildcard), `@subject.N` tokens, dynamic action subjects

**Rule file:** `rules/router/wildcard-examples.yaml`

### Setup
```bash
nats stream add DEVICES     --subjects "devices.>"     --defaults
nats stream add MONITORING  --subjects "monitoring.>"  --defaults
nats stream add EVENTS      --subjects "events.>"      --defaults
nats stream add BUILDING    --subjects "building.>"    --defaults
nats stream add COMPANY     --subjects "company.>"     --defaults
nats stream add OPS         --subjects "ops.>"         --defaults
nats stream add CRITICAL    --subjects "critical.>"    --defaults
nats stream add FACILITIES  --subjects "facilities.>"  --defaults
nats stream add ESCALATION  --subjects "escalation.>"  --defaults

./bin/shunt kv push rules/router/wildcard-examples.yaml --nats-url nats://localhost:4222
```

### Subscribe to outputs
```bash
nats sub ">"
# (subscribe to everything so you can see where messages get routed)
```

### Test single-level wildcard (`sensors.*`)
```bash
# Matches sensors.* → routes to alerts.{@subject.1}
nats pub sensors.temperature '{"value": 35, "location": "server-room"}'
# → routed to alerts.temperature (dynamic subject from @subject.1)

# Does NOT match (multi-level doesn't match single-level *)
nats pub sensors.room1.temperature '{"value": 35, "location": "room1"}'
```

### Test multi-level wildcard (`building.>`)
```bash
# Matches building.> with 3+ tokens
nats pub building.hvac.zone1.temperature '{"status": "alert", "message": "HVAC failure", "priority": "high"}'
# → routed to facilities.alerts.hvac
```

### Test dynamic routing from subject + message content
```bash
# events.* → critical.{@subject.1}.{region}
nats pub events.outage '{"severity": "critical", "region": "us-east", "description": "DB down", "affected_systems": ["api", "web"]}'
# → routed to critical.outage.us-east (combines subject token + message field)
```

**What to observe:**
- `{@subject.1}` extracts the second token (0-indexed) from the trigger subject
- `{@subject.count}` gives the total number of subject tokens
- Action subjects can mix `@subject.N` tokens with message field values

### Cleanup
```bash
./bin/shunt kv delete router.wildcard-examples --nats-url nats://localhost:4222
```

---

## Lesson 4: Array Operators

**New concepts:** `contains`, `not_contains`, `in`, `not_in` operators on arrays and strings

**Rule file:** `rules/router/array-operators.yaml`

### Setup
```bash
nats stream add USER         --subjects "user.>"         --defaults
nats stream add ANALYTICS    --subjects "analytics.>"    --defaults
nats stream add SEARCH       --subjects "search.>"       --defaults
nats stream add ORDERS       --subjects "orders.>"       --defaults
nats stream add FULFILLMENT  --subjects "fulfillment.>"  --defaults
nats stream add API          --subjects "api.>"          --defaults
nats stream add TASKS        --subjects "tasks.>"        --defaults
nats stream add SECURITY     --subjects "security.>"     --defaults
nats stream add FEATURES     --subjects "features.>"     --defaults
nats stream add IOT          --subjects "iot.>"          --defaults
nats stream add ADMIN        --subjects "admin.>"        --defaults
nats stream add CONTENT      --subjects "content.>"      --defaults
nats stream add MODERATION   --subjects "moderation.>"   --defaults
nats stream add ITEMS        --subjects "items.>"        --defaults
nats stream add CONFIG       --subjects "config.>"       --defaults

./bin/shunt kv push rules/router/array-operators.yaml --nats-url nats://localhost:4222
```

### Test array `contains`
```bash
# User with "premium" tag → analytics.premium-user-activity
nats pub user.profile-updated '{"user_id": "user-123", "tags": ["premium", "vip", "active"]}'

# User WITHOUT premium tag → no match
nats pub user.profile-updated '{"user_id": "user-456", "tags": ["basic", "active"]}'
```

### Test `in` operator (allowlist)
```bash
# Status is in allowed list → fulfillment.order-ready
nats pub orders.process '{"order_id": "ord-789", "status": "confirmed"}'

# Status NOT in allowed list → no match
nats pub orders.process '{"order_id": "ord-790", "status": "cancelled"}'
```

### Test `not_in` + `not_contains` (blocklist)
```bash
# Non-banned user, non-blocked action → security.allowed
nats pub security.event '{"user_id": "user-999", "action": "view_profile", "ip": "10.0.0.5"}'

# Banned user → no match
nats pub security.event '{"user_id": "banned-001", "action": "view_profile", "ip": "10.0.0.5"}'

# Blocked action → no match
nats pub security.event '{"user_id": "user-999", "action": "sudo", "ip": "10.0.0.5"}'
```

### Cleanup
```bash
./bin/shunt kv delete router.array-operators --nats-url nats://localhost:4222
```

---

## Lesson 5: Time-Based Conditions

**New concepts:** `@time.hour`, `@time.minute`, `@day.number`, `@day.name`, `@date.iso`, `@date.day`, `@date.month`

**Rule file:** `rules/router/time-based.yaml`

### Setup
```bash
nats stream add SYSTEM       --subjects "system.>"       --defaults
nats stream add PROCESSING   --subjects "processing.>"   --defaults
nats stream add DATA         --subjects "data.>"         --defaults
nats stream add SCHEDULER    --subjects "scheduler.>"    --defaults
nats stream add SCHEDULING   --subjects "scheduling.>"   --defaults
nats stream add REPORTS      --subjects "reports.>"      --defaults
nats stream add NOTIFICATIONS --subjects "notifications.>" --defaults

./bin/shunt kv push rules/router/time-based.yaml --nats-url nats://localhost:4222
```

### Test business hours rule
Time-based rules depend on your current clock. Check what time it is and pick the right test:

```bash
# If currently between 9 AM - 5 PM on a weekday:
nats pub sensors.temperature '{"temperature": 30}'
# → should route to alerts.business-hours

# Same message outside business hours → no match
```

### Test weekend escalation
```bash
# If it's a weekend (day number > 5):
nats pub system.errors '{"severity": "critical", "error_message": "Database connection lost"}'
# → routes to alerts.weekend-critical

# If it's a weekday, this won't match the weekend rule
```

**What to observe:** Time conditions evaluate against the server's local clock at message processing time. This makes rules like maintenance windows and business hours routing automatic.

### Cleanup
```bash
./bin/shunt kv delete router.time-based --nats-url nats://localhost:4222
```

---

## Lesson 6: KV Enrichment with JSON Path

**New concepts:** `@kv.bucket.key`, `@kv.bucket.{dynamic_key}:json.path`, KV data in conditions AND payloads

**Rule file:** `rules/router/kv-json-path.yaml`

This is the most powerful feature — looking up external data from NATS KV during rule evaluation.

### Setup KV data buckets
```bash
nats kv add customer_data
nats kv add device_config
nats kv add system_config
nats kv add feature_flags

# Seed customer data
nats kv put customer_data cust123 '{
  "tier": "premium",
  "profile": {
    "name": "Acme Corp",
    "contact": {"email": "admin@acme.com", "phone": "+1-555-0123"}
  },
  "shipping": {
    "preferences": {"method": "next_day", "carrier": "FedEx"},
    "addresses": [
      {"type": "primary", "city": "Seattle", "zip": "98101"},
      {"type": "secondary", "city": "Portland", "zip": "97201"}
    ]
  },
  "billing": {"plan": "enterprise", "credits": 1500}
}'

# Seed device config
nats kv put device_config sensor-42 '{
  "hardware": {"model": "TempSensor-Pro", "firmware": "2.1.4"},
  "thresholds": {"min": 10, "max": 35, "critical": 40},
  "location": {"building": "A", "floor": 3, "room": "server-room"}
}'
```

**Important:** You need to restart Shunt with a config file so it watches these additional KV buckets. Create a `kv-config.yaml`:

```yaml
kv:
  enabled: true
  buckets:
    - customer_data
    - device_config
    - system_config
    - feature_flags
```

### Push rules + restart
```bash
./bin/shunt kv push rules/router/kv-json-path.yaml --nats-url nats://localhost:4222

# Restart shunt with the KV config file
SHUNT_LOG_LEVEL=debug \
  ./bin/shunt serve --nats-url nats://localhost:4222 --config kv-config.yaml
```

### Test KV-enriched routing
```bash
# Premium customer order → enriched with customer data from KV
nats pub orders.premium '{"customer_id": "cust123", "order_value": 2500}'
# → routed to fulfillment.premium-customer
# → payload includes customer name, email, shipping method from KV lookup

# Non-existent customer → condition fails (KV lookup returns empty)
nats pub orders.premium '{"customer_id": "unknown", "order_value": 2500}'
```

### Test dynamic KV threshold comparison
```bash
# Temperature exceeds KV-defined threshold (max=35 for sensor-42)
nats pub sensors.temperature '{"sensor_id": "sensor-42", "temperature": 38}'
# → routed to alerts.temperature-exceeded
# → payload includes device model, firmware, location from KV

# Below threshold → no match
nats pub sensors.temperature '{"sensor_id": "sensor-42", "temperature": 30}'
```

**What to observe:**
- `{@kv.customer_data.{customer_id}:tier}` — the colon (`:`) separates the KV key from the JSON path within the value
- `{customer_id}` in the KV reference is resolved from the message first, then used as the key
- KV data is cached locally — lookups are microsecond-fast, not network calls

### Cleanup
```bash
./bin/shunt kv delete router.kv-json-path --nats-url nats://localhost:4222
```

---

## Lesson 7: Hot-Reload Rules (no restart)

**Concepts:** KV Watch auto-reloads rules when pushed

### Demonstrate hot-reload
With Shunt still running from a previous lesson:

```bash
# Push the basic rule
./bin/shunt kv push rules/router/basic.yaml --nats-url nats://localhost:4222
# Watch the server logs — it detects the new rule and creates subscriptions

# Publish a matching message
nats pub sensors.data '{"sensor": {"id": "temp001", "reading": 32.5, "location": "bedroom"}}'
# → routes to alerts.bedroom-temperature

# Now delete the rule
./bin/shunt kv delete router.basic --nats-url nats://localhost:4222
# Watch the server logs — rule removed, subscriptions cleaned up

# Same message now goes nowhere
nats pub sensors.data '{"sensor": {"id": "temp001", "reading": 32.5, "location": "bedroom"}}'
# → no output (no matching rules)
```

**What to observe:** Rules are fully dynamic. Push, update, or delete rules at any time without restarting the server.

---

## Lesson 8: Per-Rule Publish Mode Override

**New concepts:** `mode: core` vs `mode: jetstream` on individual NATS actions, mixed delivery guarantees from one trigger

**Rule file:** `rules/router/mixed-mode.yaml`

By default, all actions publish using the global `nats.publish.mode` (default: `jetstream`). Individual rules can override this with `mode: core` (fire-and-forget) or `mode: jetstream` (durable). This lets a single consumed message trigger rules with different delivery guarantees.

### Setup
```bash
nats stream add NOTIFICATIONS --subjects "notifications.>" --defaults

./bin/shunt kv push rules/router/mixed-mode.yaml --nats-url nats://localhost:4222
```

Note: we only create a stream for the `notifications.>` subject (used by the jetstream rule). The `dashboard.>` subject does not need a stream because that rule uses `mode: core`.

### Subscribe to outputs (Terminal 4)
```bash
nats sub ">"
```

### Test it (Terminal 2)
```bash
# Publish a door access event
nats pub access.door.front '{"door_id": "front", "event": "opened", "user": "alice"}'
```

**What to observe:**
- Two outputs appear: one on `notifications.access.log` (jetstream) and one on `dashboard.door.status.front` (core)
- In the Shunt debug logs, you'll see `per-rule mode override active` for each action, showing the rule mode vs global mode
- The dashboard message is fire-and-forget — if no subscriber is listening, it's silently dropped
- The notification message is durable — JetStream acknowledges it and it persists in the NOTIFICATIONS stream

### Test core vs jetstream behavior
```bash
# Stop the subscriber in Terminal 4 (Ctrl+C), then publish again
nats pub access.door.front '{"door_id": "front", "event": "closed", "user": "alice"}'

# Now restart the subscriber
nats sub ">"

# The dashboard message is gone (core = no persistence)
# But check the notifications stream — the jetstream message persists:
nats stream view NOTIFICATIONS
```

### Cleanup
```bash
./bin/shunt kv delete router.mixed-mode --nats-url nats://localhost:4222
```

---

## Lesson 9: HTTP Gateway (bidirectional HTTP <> NATS)

**New concepts:** HTTP triggers (inbound webhooks), HTTP actions (outbound calls), `@header.*`, `@path.*`, `@method`, retry config

**Rule file:** `rules/http/webhooks.yaml`

### Enable the gateway
The HTTP gateway is an optional subsystem. Enable it:

```bash
./bin/shunt kv push rules/http/webhooks.yaml --nats-url nats://localhost:4222

SHUNT_LOG_LEVEL=debug \
  ./bin/shunt serve --nats-url nats://localhost:4222 --gateway-enabled
```

### Subscribe to see inbound webhook results
```bash
nats sub ">"
```

### Test inbound webhook (HTTP → NATS)
```bash
# Simulate a GitHub PR webhook
curl -X POST http://localhost:8080/webhooks/github/pr \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: pull_request" \
  -d '{
    "action": "opened",
    "number": 42,
    "repository": {"name": "shunt"},
    "user": {"login": "octocat"},
    "pull_request": {
      "title": "Add new feature",
      "html_url": "https://github.com/example/shunt/pull/42"
    }
  }'
# → published to NATS subject: github.pr.shunt.opened

# Generic webhook
curl -X POST http://localhost:8080/webhooks/generic \
  -H "Content-Type: application/json" \
  -d '{"data": {"event": "test", "value": 123}}'
# → published to NATS subject: webhooks.received.POST
```

**What to observe:**
- HTTP headers are accessible via `{@header.X-GitHub-Event}`
- Path tokens are accessible via `{@path.N}`
- The gateway bridges external HTTP services into your NATS routing fabric

---

## Quick Reference: All CLI Commands

| Command | Purpose |
|---------|---------|
| `./bin/shunt serve` | Start the routing server |
| `./bin/shunt kv push <file\|dir>` | Push rules to NATS KV |
| `./bin/shunt kv list` | List all rules in KV |
| `./bin/shunt kv pull <key>` | Pull a specific rule |
| `./bin/shunt kv delete <key>` | Delete a rule |
| `./bin/shunt lint -r <dir>` | Validate rules offline |
| `./bin/shunt new -t <template>` | Generate a rule from template |
| `./bin/shunt new -i` | Interactive rule builder |
| `nats pub <subject> '<json>'` | Publish a test message |
| `nats sub "<pattern>"` | Subscribe to see routed output |
| `nats kv put <bucket> <key> '<json>'` | Seed KV data |
| `nats kv get <bucket> <key>` | Inspect KV data |
| `nats stream ls` | List all streams |

## Quick Reference: Rule Operators

| Operator | Usage |
|----------|-------|
| `eq`, `neq` | Equality / inequality |
| `gt`, `gte`, `lt`, `lte` | Numeric comparison |
| `contains`, `not_contains` | Array membership or string substring |
| `in`, `not_in` | Value in/not-in a list |
| `exists` | Field exists in message |
| `any`, `all`, `none` | Array element conditions (with nested items) |

## Quick Reference: Template Variables

| Variable | Example | Description |
|----------|---------|-------------|
| `{field.path}` | `{sensor.reading}` | Nested message field |
| `{@subject}` | `sensors.temperature` | Full trigger subject |
| `{@subject.N}` | `{@subject.1}` → `temperature` | Subject token (0-indexed) |
| `{@subject.count}` | `2` | Number of subject tokens |
| `{@timestamp()}` | ISO timestamp | Current time |
| `{@uuid7()}` / `{@uuid4()}` | UUID | Generated ID |
| `{@time.hour}` | `14` | Current hour (0-23) |
| `{@day.name}` | `monday` | Day of week (lowercase) |
| `{@day.number}` | `1` | Day number (1=Mon, 7=Sun) |
| `{@date.iso}` | `2026-02-18` | ISO date |
| `{@kv.bucket.key}` | `{@kv.customers.cust123}` | KV lookup (whole value) |
| `{@kv.bucket.key:path}` | `{@kv.customers.{id}:tier}` | KV lookup with JSON path |
| `{@header.Name}` | `{@header.X-API-Key}` | HTTP header (gateway) |
| `{@path.N}` | `{@path.1}` | HTTP path token (gateway) |
| `{@method}` | `POST` | HTTP method (gateway) |

## Troubleshooting

**"stream not found" errors:** Streams are required for trigger subjects (pull consumers need them) and for output subjects that use `mode: jetstream` (the default). Create a stream whose subject filter covers the subject in question. Output subjects using `mode: core` do not need a stream.

**Rule not firing:** Check `./bin/shunt lint -r <dir>` for validation errors. Ensure the KV bucket name matches (`rules` by default).

**KV enrichment returning empty:** Ensure the KV data bucket is listed in config (`kv.buckets`) and Shunt was restarted after adding it.

**Port mismatch:** Default config uses port `4442`. Override with `--nats-url nats://localhost:4222` or create a `config/shunt.yaml` with the correct URL.
