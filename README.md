# Shunt

A high-performance, rule-based message router for NATS JetStream with an integrated HTTP gateway and automated token management.

Rules are stored in NATS KV and hot-reloaded via KV Watch — no restarts required.

## Features

*   **High Performance**: Microsecond rule evaluation, asynchronous processing, thousands of messages per second.
*   **KV-Based Rules**: Rules stored in NATS KV, hot-reloaded via Watch. Manage with `shunt kv push/pull/list/delete`.
*   **Array Processing**: Batch message processing with array operators (`any`, `all`, `none`) and `forEach` iteration.
*   **Primitive Message Support**: Handle strings, numbers, arrays, and objects at the root.
*   **HTTP Gateway** (optional subsystem): Bidirectional HTTP-to-NATS bridge with inbound webhook ingestion and outbound API calls.
*   **Auth Manager** (optional subsystem): OAuth2 and custom-HTTP token management, stored in NATS KV.
*   **NATS JetStream Native**: Pull consumers for durable, scalable message processing. Per-rule publish mode override (`core` or `jetstream`) for mixed delivery guarantees.
*   **Debounce**: Per-rule suppression of rapid re-fires within a configurable time window.
*   **Rule Engine**: Dynamic conditions, payload/header/subject templating, KV data enrichment with local cache, time-based logic.
*   **Cryptographic Security**: NKey signature verification for message integrity.
*   **Production Ready**: Structured logging, Prometheus metrics, graceful shutdown, full NATS auth support.

## Architecture

Single binary with subcommands:

*   **`shunt serve`** — Start the routing server. Runs NATS-to-NATS message routing with optional subsystems:
    *   **Gateway** (`gateway.enabled: true`): Bidirectional HTTP-to-NATS bridge for webhooks and outbound API calls.
    *   **Auth Manager** (`authManager.enabled: true`): Manages OAuth2/custom-HTTP tokens in NATS KV.
*   **`shunt kv push`** / `pull` / `list` / `delete` — Manage rules in NATS KV.
*   **`shunt lint`** / `test` / `check` — Validate rules offline.
*   **`shunt new`** / `scaffold` — Generate rule templates.

Rules can be managed in a separate Git repository and deployed via CI/CD — see [Rule Management](./docs/06-deployment.md#rule-management) in the deployment guide.

## Quick Start

### Prerequisites

*   Go 1.23+ (for building from source)
*   A running NATS Server with JetStream enabled

### 1. Build

```bash
go build -o shunt ./cmd/shunt
```

### 2. Set Up NATS

```bash
# KV bucket for rule storage (required)
nats kv add rules

# Streams for your message subjects
nats stream add EVENTS --subjects "events.>"
nats stream add ALERTS --subjects "alerts.>"
```

### 3. Push Rules

Write a rule file and push it to NATS KV:

```yaml
# routing.yaml
- trigger:
    nats:
      subject: "events.device.status"
  conditions:
    operator: and
    items:
      - field: "{severity}"
        operator: gte
        value: 5
  action:
    nats:
      subject: "alerts.critical.{device_id}"
      passthrough: true
```

```bash
./shunt kv push routing.yaml --nats-url nats://localhost:4222
```

### 4. Run

```bash
./shunt serve --nats-url nats://localhost:4222
```

Or with env vars:

```bash
SHUNT_NATS_URL=nats://localhost:4222 SHUNT_METRICS_ENABLED=true ./shunt serve
```

## Container Image

```bash
docker pull ghcr.io/danielmichaels/shunt:latest

docker run --rm \
  -e SHUNT_NATS_URL=nats://nats:4222 \
  -e SHUNT_METRICS_ENABLED=true \
  -p 2112:2112 \
  ghcr.io/danielmichaels/shunt:latest
```

Docker Compose:

```yaml
services:
  shunt:
    image: ghcr.io/danielmichaels/shunt:latest
    environment:
      SHUNT_NATS_URL: nats://nats:4222
      SHUNT_METRICS_ENABLED: "true"
      SHUNT_GATEWAY_ENABLED: "true"
    ports:
      - "8080:8080"
      - "2112:2112"
    depends_on:
      - nats
  nats:
    image: nats:latest
    command: ["--jetstream"]
    ports:
      - "4222:4222"
```

## Documentation

*   **[01 - Core Concepts](./docs/01-core-concepts.md)**: Triggers, Conditions, Actions, and Environment Variables.
*   **[02 - System Variables & Functions](./docs/02-system-variables.md)**: Full reference for all `@` variables and functions.
*   **[03 - Array Processing](./docs/03-array-processing.md)**: Guide to `forEach` and array operators.
*   **[04 - Primitive & Array Root Messages](./docs/04-primitive-messages.md)**: Non-object JSON payloads.
*   **[05 - Security](./docs/05-security.md)**: Cryptographic Signature Verification.
*   **[06 - Deployment](./docs/06-deployment.md)**: Container deployment, health checks, init containers.
*   **[07 - Configuration](./docs/07-configuration.md)**: Complete configuration reference.

## Monitoring

Prometheus metrics endpoint on `:2112/metrics` (when `metrics.enabled` is `true`).

Key metrics:

| Metric | Description |
|---|---|
| `messages_total` | Messages processed by status |
| `rule_matches_total` | Rule match count |
| `messages_debounced_total` | Messages suppressed by per-rule debounce |
| `actions_total` | Actions executed by status |
| `action_publish_failures_total` | NATS publish failures |
| `nats_connection_status` | 1 = connected, 0 = disconnected |
| `foreach_iterations_total` | Array elements processed in forEach |
| `http_inbound_requests_total` | Inbound HTTP requests (gateway) |
| `http_outbound_requests_total` | Outbound HTTP requests (gateway) |

## License

This project is licensed under the MIT License - see the [LICENSE](./LICENSE) file for details.
