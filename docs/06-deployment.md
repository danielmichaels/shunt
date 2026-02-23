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

Streams are required for two things:

1. **Trigger subjects** — shunt always creates JetStream pull consumers to receive messages, so every trigger subject must be covered by a stream.
2. **Output subjects using `mode: jetstream`** — JetStream publish requires a stream on the target subject.

Output subjects using `mode: core` publish via core NATS (fire-and-forget) and do **not** need a stream.

```bash
# Example: streams covering trigger and JetStream-output subjects
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
  -e SHUNT_NATS_URL=nats://nats:4222 \
  -e SHUNT_METRICS_ENABLED=true \
  -p 2112:2112 \
  ghcr.io/danielmichaels/shunt:latest
```

To enable the HTTP gateway subsystem:

```bash
docker run --rm \
  -e SHUNT_NATS_URL=nats://nats:4222 \
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

A reference config with all available options and documentation is provided at `config/shunt.yaml.example`. Copy and customize it as needed.

## Configuration in Containers

Shunt uses a layered configuration model:

1. **YAML config file** — full configuration, all options supported
2. **CLI flags** — a subset of commonly-used options that override the config file
3. **Environment variables** — each CLI flag has a corresponding `SHUNT_`-prefixed env var

No config file is required — shunt starts with sane defaults and gracefully handles a missing file. For simple deployments, CLI flags (via env vars) are sufficient. For advanced tuning, mount a YAML config file.

### Available Environment Variables

Only `serve` command CLI flags have env var equivalents. These override the corresponding config file values:

| CLI Flag | Env Var | Config Key | Example |
|---|---|---|---|
| `--nats-url` | `SHUNT_NATS_URL` | `nats.urls` | `nats://nats:4222` |
| `--log-level` | `SHUNT_LOG_LEVEL` | `logging.level` | `debug` |
| `--metrics-enabled` | `SHUNT_METRICS_ENABLED` | `metrics.enabled` | `true` |
| `--metrics-addr` | `SHUNT_METRICS_ADDR` | `metrics.address` | `:2112` |
| `--metrics-path` | `SHUNT_METRICS_PATH` | `metrics.path` | `/metrics` |
| `--gateway-enabled` | `SHUNT_GATEWAY_ENABLED` | `gateway.enabled` | `true` |
| `--kv-enabled` | `SHUNT_KV_ENABLED` | `kv.enabled` | `true` |
| `--worker-count` | `SHUNT_WORKER_COUNT` | `nats.consumers.workerCount` | `8` |
| `-c` / `--config` | `SHUNT_CONFIG` | — | `/etc/shunt/shunt.yaml` |

`SHUNT_NATS_URL` accepts a comma-separated string for multiple servers (e.g. `nats://s1:4222,nats://s2:4222`).

### Defaults vs Production Tuning

The built-in defaults are conservative. For production workloads, consider overriding:

| Setting | Default | Recommended | Env Var / Config |
|---|---|---|---|
| Worker count | `2` | `8` (2-4x CPU cores) | `SHUNT_WORKER_COUNT` |
| Fetch batch size | `1` | `64` (higher throughput) | config file: `nats.consumers.fetchBatchSize` |
| KV enrichment | `false` | `true` if using KV enrichment | `SHUNT_KV_ENABLED` |
| HTTP gateway | `false` | `true` for HTTP ingest | `SHUNT_GATEWAY_ENABLED` |
| Signature verification | `false` | `true` for signed messages | config file: `security.verification.enabled` |

### When You Need a Config File

A config file is required for settings that don't have a CLI flag, including:

- **`nats.credsFile`** — NATS JWT credentials path
- **`nats.consumers.fetchBatchSize`** — messages per pull request
- **`kv.buckets`** — list of KV bucket names for enrichment
- **`authManager.providers`** — OAuth/HTTP provider definitions
- **`security.verification.*`** — signature verification settings
- All other deeply-nested options (TLS, HTTP server/client tuning, etc.)

See [Configuration Reference](./07-configuration.md) for the complete list of all settings.

## Health Checks

When the HTTP gateway is enabled (`gateway.enabled: true`), Shunt exposes dedicated health endpoints on the gateway port:

- `GET /healthz` — returns `200 OK` when the server is running
- `GET /health` — same behavior, alternate path

```yaml
# Kubernetes probe example (gateway enabled)
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

If the gateway is **not** enabled, use the Prometheus metrics endpoint instead:

```yaml
# Kubernetes probe example (gateway disabled)
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

The metrics endpoint is available when `metrics.enabled` is `true` (the default).

## Kubernetes Deployment

### Env-Var-Only (No Config File)

The simplest deployment uses only env vars. This works for any setup that doesn't require KV enrichment, auth providers, or advanced tuning:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shunt
spec:
  replicas: 1
  selector:
    matchLabels:
      app: shunt
  template:
    metadata:
      labels:
        app: shunt
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "2112"
        prometheus.io/path: "/metrics"
    spec:
      containers:
        - name: shunt
          image: ghcr.io/danielmichaels/shunt:latest
          ports:
            - containerPort: 8080
              name: gateway
            - containerPort: 2112
              name: metrics
          env:
            - name: SHUNT_NATS_URL
              value: "nats://nats:4222"
            - name: SHUNT_GATEWAY_ENABLED
              value: "true"
            - name: SHUNT_METRICS_ENABLED
              value: "true"
            - name: SHUNT_WORKER_COUNT
              value: "8"
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
```

### With Credentials File

For NATS JWT authentication, mount a `.creds` file from a Secret and reference it via a config file (the `nats.credsFile` setting requires a config file — it has no CLI flag or env var):

```yaml
args: ["serve", "--config", "/etc/shunt/shunt.yaml"]
volumeMounts:
  - name: nats-creds
    mountPath: /etc/nats/creds
    readOnly: true
  - name: config
    mountPath: /etc/shunt
    readOnly: true
# ...
volumes:
  - name: nats-creds
    secret:
      secretName: shunt-nats-creds
  - name: config
    configMap:
      name: shunt-config
```

The ConfigMap for credentials:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: shunt-config
data:
  shunt.yaml: |
    nats:
      credsFile: /etc/nats/creds/shunt.creds
```

Create the Secret from your `.creds` file:

```bash
kubectl create secret generic shunt-nats-creds \
  --from-file=shunt.creds=/path/to/shunt.creds
```

### With Config File (KV Enrichment)

If you use KV enrichment, `kv.buckets` requires a config file since lists cannot be set via a single env var. Mount a ConfigMap and use env vars for the remaining overrides:

```yaml
env:
  - name: SHUNT_NATS_URL
    value: "nats://nats:4222"
  - name: SHUNT_GATEWAY_ENABLED
    value: "true"
  - name: SHUNT_WORKER_COUNT
    value: "8"
args: ["serve", "--config", "/etc/shunt/shunt.yaml"]
volumeMounts:
  - name: config
    mountPath: /etc/shunt
    readOnly: true
# ...
volumes:
  - name: config
    configMap:
      name: shunt-config
```

The ConfigMap only needs the fields that require YAML — env vars still override everything else:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: shunt-config
data:
  shunt.yaml: |
    kv:
      enabled: true
      buckets:
        - "device_status"
        - "feature_flags"
```

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

| Metric | Type | Labels | Description |
|---|---|---|---|
| `rule_matches_total` | Counter | `rule_name` | Rules matched per rule |
| `actions_total` | Counter | `status`, `rule_name` | Actions executed (success/error) per rule |
| `actions_by_type_total` | Counter | `type`, `rule_name` | Actions by type (nats/http) per rule |
| `action_publish_failures_total` | Counter | `rule_name` | NATS publish failures per rule |
| `messages_debounced_total` | Counter | `rule_name` | Debounced messages per rule |
| `messages_total{status="error"}` | Counter | `status` | Failed message processing |
| `nats_connection_status` | Gauge | | 1 = connected, 0 = disconnected |
| `nats_reconnects_total` | Counter | | NATS reconnection events |
| `rules_active` | Gauge | | Number of loaded rules |
| `message_processing_backlog` | Gauge | | Messages waiting to be processed |

The `rule_name` label is derived from the rule's optional `name` field. When omitted, the trigger subject (NATS) or path (HTTP) is used instead. Set explicit names when multiple rules share the same trigger to distinguish them in dashboards.

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
        run: |
          shunt lint -r sensors/
          shunt lint -r alerts/
          shunt lint -r webhooks/

      - name: Run rule tests
        run: |
          shunt test -r sensors/
          shunt test -r alerts/
          shunt test -r webhooks/

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

- **Separate buckets**: Push to `rules-staging` and `rules-production` buckets in the same cluster. Configure shunt with `rules.kvBucket` in the config file per environment.
- **Separate clusters**: Push to entirely different NATS clusters per environment using `--nats-url` targeting each cluster.

```bash
# Push to staging
shunt kv push sensors/ --nats-url $NATS_STAGING --bucket rules-staging

# Push to production (after staging validation)
shunt kv push sensors/ --nats-url $NATS_PRODUCTION --bucket rules-production
```
