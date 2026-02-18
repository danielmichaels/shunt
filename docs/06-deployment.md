# Deployment Guide

## NATS Prerequisites

Shunt requires a NATS server with JetStream enabled. Before starting shunt, create the required infrastructure.

### KV Buckets

The `rules` KV bucket must exist — shunt watches it for rule definitions:

```bash
nats kv add rules
```

If your rules reference additional KV buckets for data enrichment (configured via `kv.buckets`), create those too:

```bash
nats kv add device_status
nats kv add feature_flags
```

### JetStream Streams

Create streams for every subject your rules trigger on or publish to:

```bash
nats stream add EVENTS --subjects "events.>"
nats stream add ALERTS --subjects "alerts.>"
```

As an alternative to imperative `nats` CLI commands, consider [declarative JetStream configuration](https://docs.nats.io/running-a-nats-service/configuration/resource_management) managed alongside your NATS server config.

## Container Image

Container images are published to GHCR on every push to `main` and on semver tags.

```bash
docker pull ghcr.io/danielmichaels/shunt:latest
```

### Running

```bash
docker run --rm \
  -e SHUNT_NATS_URLS=nats://nats:4222 \
  -e SHUNT_METRICS_ENABLED=true \
  -p 2112:2112 \
  ghcr.io/danielmichaels/shunt:latest
```

To enable the HTTP gateway subsystem:

```bash
docker run --rm \
  -e SHUNT_NATS_URLS=nats://nats:4222 \
  -e SHUNT_GATEWAY_ENABLED=true \
  -e SHUNT_METRICS_ENABLED=true \
  -p 8080:8080 \
  -p 2112:2112 \
  ghcr.io/danielmichaels/shunt:latest
```

Optionally mount a config file instead of using env vars:

```bash
docker run --rm \
  -v ./my-config.yaml:/config.yaml \
  ghcr.io/danielmichaels/shunt:latest \
  --config /config.yaml
```

## Configuration in Containers

All configuration keys can be set via environment variables with the `SHUNT_` prefix. Nested keys use `_` as a separator.

| Config Key | Env Var | Example |
|---|---|---|
| `nats.urls` | `SHUNT_NATS_URLS` | `nats://nats:4222` |
| `nats.credsFile` | `SHUNT_NATS_CREDSFILE` | `/etc/nats/creds/user.creds` |
| `nats.consumers.workerCount` | `SHUNT_NATS_CONSUMERS_WORKERCOUNT` | `4` |
| `logging.level` | `SHUNT_LOGGING_LEVEL` | `debug` |
| `metrics.enabled` | `SHUNT_METRICS_ENABLED` | `true` |
| `metrics.address` | `SHUNT_METRICS_ADDRESS` | `:2112` |
| `gateway.enabled` | `SHUNT_GATEWAY_ENABLED` | `true` |
| `rules.kvBucket` | `SHUNT_RULES_KVBUCKET` | `rules` |

`SHUNT_NATS_URLS` accepts a comma-separated string for multiple servers (e.g. `nats://s1:4222,nats://s2:4222`).

List values like `kv.buckets` cannot be set via a single env var — use a config file for those.

See [Configuration Reference](./07-configuration.md) for the complete list of all settings.

## Health Checks

Shunt does not expose a dedicated `/healthz` endpoint. Use the Prometheus metrics endpoint for liveness probes:

```yaml
# Kubernetes probe example
livenessProbe:
  httpGet:
    path: /metrics
    port: 2112
  initialDelaySeconds: 5
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /metrics
    port: 2112
  initialDelaySeconds: 5
  periodSeconds: 10
```

The metrics endpoint is available when `metrics.enabled` is `true` (the default via `--metrics-enabled` flag).

## Prometheus Metrics

Add scrape annotations to your pod:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "2112"
    prometheus.io/path: "/metrics"
```

Key metrics to alert on:

| Metric | Type | Description |
|---|---|---|
| `messages_total{status="error"}` | Counter | Failed message processing |
| `action_publish_failures_total` | Counter | NATS publish failures |
| `nats_connection_status` | Gauge | 1 = connected, 0 = disconnected |
| `nats_reconnects_total` | Counter | NATS reconnection events |
| `rules_active` | Gauge | Number of loaded rules |
| `message_processing_backlog` | Gauge | Messages waiting to be processed |

## Init Container Pattern

Use an init container to idempotently create NATS streams and KV buckets before shunt starts. This prevents startup failures when infrastructure doesn't exist yet.

```yaml
initContainers:
  - name: nats-setup
    image: natsio/nats-box:latest
    command:
      - /bin/sh
      - -c
      - |
        set -e
        nats kv add rules --server=$NATS_URL 2>/dev/null || true
        nats stream add EVENTS --subjects="events.>" --server=$NATS_URL 2>/dev/null || true
        nats stream add ALERTS --subjects="alerts.>" --server=$NATS_URL 2>/dev/null || true
    env:
      - name: NATS_URL
        value: nats://nats:4222
```

The `2>/dev/null || true` pattern makes the commands idempotent — they succeed whether the resource already exists or is newly created.

## Rule Management

Rules are standalone YAML files, decoupled from the shunt server binary. The server watches the NATS KV `rules` bucket and hot-reloads on any change — you never restart shunt to deploy new routing logic. This makes rules ideal for a GitOps workflow: author rules in version control, validate in CI, and push to NATS KV on merge.

### Separate Rules Repository

Rules don't need to live in the same repository as your shunt deployment. A dedicated rules repo keeps routing logic independent and lets domain teams own their rules without access to infrastructure code.

```
my-rules-repo/
├── sensors/
│   ├── tank.yaml
│   └── temperature.yaml
├── alerts/
│   └── critical.yaml
├── webhooks/
│   └── github.yaml
└── README.md
```

### KV Key Derivation

When `shunt kv push` uploads a file, the file path determines the KV key:

1. Strip the `.yaml` or `.yml` extension
2. Replace path separators (`/`, `\`) with `.`
3. Strip the bucket name prefix (default: `rules.`)

Examples:

| File Path | KV Key |
|---|---|
| `routing.yaml` | `routing` |
| `sensors/tank.yaml` | `sensors.tank` |
| `alerts/critical.yaml` | `alerts.critical` |

This means your directory structure maps directly to a dotted namespace in the KV store.

### Directory Push Behavior

`shunt kv push <dir>` pushes all `*.yaml` and `*.yml` files in the given directory but **does not recurse into subdirectories**. To push a nested structure, push each directory separately:

```bash
shunt kv push sensors/   --nats-url $NATS_URL
shunt kv push alerts/    --nats-url $NATS_URL
shunt kv push webhooks/  --nats-url $NATS_URL
```

Or push individual files directly:

```bash
shunt kv push sensors/tank.yaml --nats-url $NATS_URL
```

### CI/CD Pipeline

A typical pipeline validates rules before pushing them to NATS KV. Run `lint` and `test` on every PR, and `kv push` only on merge to your main branch.

```yaml
# Example GitHub Actions workflow
name: rules
on:
  push:
    branches: [main]
  pull_request:

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install shunt
        run: go install github.com/danielmichaels/shunt/cmd/shunt@latest

      - name: Lint rules
        run: shunt lint sensors/ alerts/ webhooks/

      - name: Run rule tests
        run: shunt test sensors/ alerts/ webhooks/

  push:
    needs: validate
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install shunt
        run: go install github.com/danielmichaels/shunt/cmd/shunt@latest

      - name: Push rules to NATS KV
        run: |
          shunt kv push sensors/   --nats-url ${{ secrets.NATS_URL }} --creds ${{ secrets.NATS_CREDS_PATH }}
          shunt kv push alerts/    --nats-url ${{ secrets.NATS_URL }} --creds ${{ secrets.NATS_CREDS_PATH }}
          shunt kv push webhooks/  --nats-url ${{ secrets.NATS_URL }} --creds ${{ secrets.NATS_CREDS_PATH }}
```

### Multi-Environment Strategy

Use separate KV buckets or NATS clusters per environment:

- **Separate buckets**: Push to `rules-staging` and `rules-production` buckets in the same cluster. Configure shunt with `rules.kvBucket` (or `SHUNT_RULES_KVBUCKET`) per environment.
- **Separate clusters**: Push to entirely different NATS clusters per environment using `--nats-url` targeting each cluster.

```bash
# Push to staging
shunt kv push sensors/ --nats-url $NATS_STAGING --bucket rules-staging

# Push to production (after staging validation)
shunt kv push sensors/ --nats-url $NATS_PRODUCTION --bucket rules-production
```
