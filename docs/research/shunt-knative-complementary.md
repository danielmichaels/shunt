# Research: Shunt + Knative Eventing — Complementary or Redundant?

## Context

You're considering replacing OpenFaaS with Knative Serving/Eventing as a way to run small functions that react to NATS messages without being full standalone apps. The question: does shunt still add value in a cluster where Knative Eventing can also subscribe to NATS, or does it become redundant?

## TL;DR

**They're complementary, not redundant.** Shunt is a smart routing/transformation layer that sits *between* NATS messages and their destinations. Knative is a function runtime with scale-to-zero. They solve different problems and work well together. However, for the *simplest* cases (raw NATS message → invoke a function, no transformation), Knative Eventing alone could suffice.

---

## How the Pieces Fit Together

### Without Shunt (Knative alone)

```
NATS Subject → [Knative JetStream Channel] → [Trigger w/ filter] → [Knative Service (your function)]
```

- Knative's `eventing-natss` extension provides JetStream-backed Channels
- Messages get wrapped in CloudEvents format and delivered as HTTP POST to your Knative Service
- Triggers can do **simple attribute-based filtering** (exact match on CloudEvents attributes like `type`, `source`)
- Your function scales to zero when idle, spins up on demand
- **No payload transformation, no conditional routing, no KV enrichment, no templating**

### With Shunt + Knative

```
NATS Subject → [Shunt] → (evaluates rules, transforms, enriches, routes) → publishes to new NATS subject(s)
                                                                              ↓
                                                              [Knative JetStream Channel] → [Trigger] → [Knative Service]
```

Or via shunt's HTTP gateway:

```
NATS Subject → [Shunt] → HTTP action → [Knative Service URL directly]
```

---

## What Each Brings to the Table

| Capability | Shunt | Knative Eventing |
|---|---|---|
| Subscribe to NATS JetStream | Yes (pull consumers, fine-grained control) | Yes (via eventing-natss, alpha quality) |
| Message transformation/templating | Yes — rich template engine with `{field}`, `{@kv.bucket.key}`, `{@uuid7()}`, etc. | No — passes messages through as CloudEvents |
| Conditional routing (gt, lt, regex, contains, AND/OR groups) | Yes — full rule engine | Minimal — exact-match on CloudEvents attributes only |
| Fan-out / split (forEach, filter arrays) | Yes — native forEach with per-element filtering | No — one message in, one delivery out |
| KV enrichment (lookup data, inject into payload) | Yes — built-in with local cache + KV Watch | No — your function code must do this |
| Hot-reload routing rules (no restart/redeploy) | Yes — rules in NATS KV, watched in real-time | No — requires CRD updates + reconciliation |
| Scale-to-zero functions | No — persistent workers | Yes — this is Knative's core strength |
| Function lifecycle management | No | Yes — deploy, version, traffic splitting |
| Debouncing | Yes — per-rule time windows | No |
| Cryptographic message verification | Yes — NKey signatures | No |

## The Three Scenarios

### 1. Shunt is redundant — use Knative alone
When your use case is: *"NATS message arrives on subject X, invoke function Y with the raw payload, done."*

- No transformation needed
- No conditional routing (or only simple type-based filtering)
- No enrichment from other data sources
- You just want scale-to-zero execution

In this case, a Knative JetStream Channel + Trigger + Service handles it cleanly.

### 2. Shunt + Knative complement each other (the sweet spot)
When: *"NATS messages arrive, but I need to inspect them, transform payloads, enrich with KV data, route to different functions based on content, and sometimes fan-out arrays into individual function calls."*

- Shunt acts as the **intelligent routing/transformation layer**
- Knative acts as the **function runtime** (scale-to-zero, HTTP-based)
- Shunt publishes transformed messages to specific NATS subjects that Knative Channels consume
- Or shunt calls Knative Services directly via HTTP actions (shunt already supports HTTP outbound with retry/backoff)

**This is the "shunt glues things" scenario you described** — and it's the strongest case.

### 3. Shunt handles everything, Knative not needed
When: *"I just need messages routed/transformed/published to other NATS subjects for downstream consumers that are always running."*

- No need for scale-to-zero
- Downstream consumers are persistent services
- Shunt does the routing, transformation, and publishing

---

## Replacing OpenFaaS with Knative — Where Shunt Fits

OpenFaaS and Knative Serving solve the same problem: run small functions with scale-to-zero. The difference is Knative is more Kubernetes-native and has the Eventing subsystem.

Your current flow is likely:
```
NATS message → [something invokes OpenFaaS function] → function processes
```

With Knative alone:
```
NATS message → [Knative Channel+Trigger] → Knative Service (function)
```
This works for simple cases but you lose any routing intelligence.

With Shunt + Knative:
```
NATS message → [Shunt evaluates rules] → HTTP POST to Knative Service (or publish to routed NATS subject → Channel → Service)
```
This gives you the best of both: smart routing + scale-to-zero functions.

**Key insight**: Shunt's HTTP action support with retry/backoff means it can call Knative Services directly without needing Knative Eventing at all. You could skip the Channel/Trigger layer entirely and use shunt as the event source that invokes Knative Serving functions over HTTP.

---

## Practical Recommendation

```
                         Simple "just invoke"
NATS ──→ Knative Channel ──→ Trigger ──→ Knative Service

                         Complex routing/transformation
NATS ──→ Shunt ──→ HTTP action ──→ Knative Service (direct call)
              └──→ NATS publish ──→ other consumers
```

- Use **Knative Serving** as your OpenFaaS replacement (scale-to-zero functions)
- Use **Shunt** when you need routing logic, transformations, or enrichment before invoking functions
- Use **Knative Eventing** (Channels/Triggers) only when you want Knative-native NATS subscription for simple pass-through cases
- For complex cases, **shunt calling Knative Services via HTTP** is arguably simpler than configuring Knative Eventing's Channel+Trigger+Subscription CRDs

## Caveats

- `eventing-natss` JetStream support is **alpha quality** — shunt's JetStream consumer management is more mature and configurable
- Knative Eventing wraps everything in CloudEvents, which adds overhead if your functions just want raw JSON
- Cold start latency with scale-to-zero means shunt's HTTP actions to Knative Services will occasionally see higher latency on first invocation

---

## Concrete Example: Shunt + Knative Working Together

The scenario from our discussion:

> A NATS message arrives with a temperature reading. Shunt checks the threshold — it's over limit, so shunt sends a Telegram notification. No Knative needed. But now we also want to log those threshold breaches in Postgres. Shunt can't do that, so shunt POSTs to a Knative Service. Knative spins up, connects to PG, writes the row, scales back to zero.

```
                                    ┌──→ Telegram API (shunt HTTP action)
                                    │
NATS: sensors.temperature.> ──→ [Shunt] ──→ threshold exceeded?
                                    │
                                    └──→ Knative Service: log-to-postgres (HTTP action)
                                              ↓
                                         scale 0→1, connect PG, INSERT, scale 1→0
```

**Shunt rule** for this:
```yaml
# Rule 1: Telegram alert
- name: temp-alert-telegram
  trigger:
    nats:
      subject: "sensors.temperature.>"
  conditions:
    operator: and
    items:
      - field: "{temperature}"
        operator: gt
        value: 30
  action:
    http:
      url: "https://api.telegram.org/bot{@kv.secrets.telegram-token}/sendMessage"
      method: POST
      payload: |
        {
          "chat_id": "{@kv.config.alert-chat-id}",
          "text": "🔥 {device_id} temp {temperature}°C exceeds threshold"
        }

# Rule 2: Log to Postgres via Knative
- name: temp-alert-log-pg
  trigger:
    nats:
      subject: "sensors.temperature.>"
  conditions:
    operator: and
    items:
      - field: "{temperature}"
        operator: gt
        value: 30
  action:
    http:
      url: "http://log-to-postgres.default.svc.cluster.local"
      method: POST
      payload: |
        {
          "device_id": "{device_id}",
          "temperature": "{temperature}",
          "timestamp": "{@timestamp()}"
        }
      retry:
        maxAttempts: 3
        initialDelay: 2s
        maxDelay: 10s
```

Note: `http://log-to-postgres.default.svc.cluster.local` is the in-cluster URL for the Knative Service. Knative's activator intercepts the request and cold-starts the pod if it's scaled to zero.

---

## Getting Started with Knative Serving (on k3s)

### Step 1: Install k3s without Traefik

Knative needs its own networking layer, so disable k3s's built-in Traefik:

```bash
curl -sfL https://get.k3s.io | sh -s - --disable traefik
```

### Step 2: Install Knative Serving

```bash
# CRDs
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.21.1/serving-crds.yaml

# Core components
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.21.1/serving-core.yaml
```

### Step 3: Install Kourier (lightweight networking)

Kourier is purpose-built for Knative — just Envoy, no extra CRDs. Best for homelab:

```bash
kubectl apply -f https://github.com/knative-extensions/net-kourier/releases/download/knative-v1.21.0/kourier.yaml

# Tell Knative to use Kourier
kubectl patch configmap/config-network \
  --namespace knative-serving \
  --type merge \
  --patch '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}'
```

### Step 4: DNS (sslip.io for homelab)

```bash
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.21.1/serving-default-domain.yaml
```

### Step 5: Verify

```bash
kubectl get pods -n knative-serving
# Should see: activator, autoscaler, controller, webhook, 3scale-kourier-control
```

That's it. You can now deploy Knative Services.

---

## Porting an OpenFaaS Function to Knative: Side-by-Side

### The Example: Read a Kubernetes Secret, Return Its Value

This is intentionally trivial — it isolates the differences in project structure, secret access, build workflow, and deployment.

---

### OpenFaaS Version

**Scaffold:**
```bash
faas-cli new --lang golang-middleware read-secret
```

**Project structure:**
```
read-secret/
├── handler.go
read-secret.yml   # (stack.yml)
```

**read-secret.yml** (stack file):
```yaml
version: 1.0
provider:
  name: openfaas
  gateway: http://127.0.0.1:8080

functions:
  read-secret:
    lang: golang-middleware
    handler: ./read-secret
    image: registry.example.com/read-secret:latest
    secrets:
      - my-api-key    # <-- declares which K8s secret to mount
```

**read-secret/handler.go:**
```go
package function

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

func Handle(w http.ResponseWriter, r *http.Request) {
	// OpenFaaS mounts secrets as files at /var/openfaas/secrets/<name>
	secret, err := os.ReadFile("/var/openfaas/secrets/my-api-key")
	if err != nil {
		http.Error(w, fmt.Sprintf("error reading secret: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Secret value: %s", strings.TrimSpace(string(secret)))
}
```

**Create the secret and deploy:**
```bash
# Create the K8s secret (OpenFaaS requires it in the openfaas-fn namespace)
kubectl create secret generic my-api-key \
  --from-literal=my-api-key="super-secret-value" \
  --namespace openfaas-fn

# Build, push, and deploy — all in one command
faas-cli up -f read-secret.yml
```

**What `faas-cli up` does under the hood:**
1. `faas-cli build` — uses the `golang-middleware` template to generate a Dockerfile, runs `docker build`
2. `faas-cli push` — runs `docker push` to your registry
3. `faas-cli deploy` — calls the OpenFaaS gateway API to create the function

---

### Knative Version

**Option A: Custom container (recommended — full control)**

**Project structure:**
```
read-secret/
├── main.go
├── Dockerfile
└── service.yaml
```

**main.go:**
```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func handler(w http.ResponseWriter, r *http.Request) {
	// Knative uses standard K8s secret injection — env var or volume mount
	// Here using an environment variable injected via secretKeyRef
	secret := os.Getenv("MY_API_KEY")
	if secret == "" {
		http.Error(w, "MY_API_KEY not set", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Secret value: %s", secret)
}

func main() {
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

**Dockerfile:**
```dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod main.go ./
RUN go build -o server .

FROM alpine:3.21
COPY --from=build /app/server /server
CMD ["/server"]
```

**service.yaml:**
```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: read-secret
  namespace: default
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/min-scale: "0"
    spec:
      containers:
        - image: registry.example.com/read-secret:latest
          ports:
            - containerPort: 8080
          env:
            - name: MY_API_KEY
              valueFrom:
                secretKeyRef:
                  name: my-api-key
                  key: my-api-key
```

**Create the secret and deploy:**
```bash
# Create the K8s secret (same namespace as the Knative Service)
kubectl create secret generic my-api-key \
  --from-literal=my-api-key="super-secret-value" \
  --namespace default

# Build and push — YOU do this, not the platform
docker build -t registry.example.com/read-secret:latest .
docker push registry.example.com/read-secret:latest

# Deploy
kubectl apply -f service.yaml
```

**Option B: Using `func` CLI (scaffolded, buildpack-based)**

```bash
func create -l go -t http read-secret
cd read-secret
```

Edit the generated `handle.go`:
```go
package function

import (
	"context"
	"fmt"
	"net/http"
	"os"
)

func Handle(ctx context.Context, res http.ResponseWriter, req *http.Request) {
	secret := os.Getenv("MY_API_KEY")
	if secret == "" {
		http.Error(res, "MY_API_KEY not set", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, "Secret value: %s", secret)
}
```

Edit `func.yaml` to add the secret env var:
```yaml
envs:
  - name: MY_API_KEY
    value: '{{ secret:my-api-key:my-api-key }}'
```

```bash
func deploy --registry registry.example.com/myuser
```

---

## The Build & Push Model: The Big Difference

This is one of the most practical differences when migrating from OpenFaaS to Knative.

### OpenFaaS: Platform Builds For You

```
Developer writes handler.go
        ↓
faas-cli build      ← generates Dockerfile from template, runs docker build
        ↓
faas-cli push       ← pushes image to registry
        ↓
faas-cli deploy     ← tells OpenFaaS gateway to pull and run the image
```

Or just `faas-cli up` which does all three.

**What you DON'T have to think about:**
- Writing a Dockerfile (the template generates one)
- The HTTP server boilerplate (the template's watchdog handles it)
- Health checks (built into the watchdog)
- Graceful shutdown (built into the watchdog)

**What `faas-cli build` actually does:**
1. Copies your `handler.go` into a generated build context
2. Uses the `golang-middleware` template which includes the OpenFaaS watchdog (a process that wraps your handler with HTTP server, health checks, timeouts, etc.)
3. Runs `docker build` using the template's Dockerfile

### Knative: You Own the Container

```
Developer writes main.go + Dockerfile
        ↓
docker build        ← you build it yourself
        ↓
docker push         ← you push it yourself
        ↓
kubectl apply       ← standard K8s deployment (or kn service create)
```

**What you DO have to think about:**
- Writing a Dockerfile (or using `func` CLI with buildpacks to avoid this)
- Including an HTTP server in your code (`net/http`, chi, echo, etc.)
- Health check endpoints (Knative probes your container)
- Graceful shutdown handling

**What you get in return:**
- Full control over the container — use any base image, any framework, any dependencies
- No vendor-specific watchdog process wrapping your code
- Standard OCI images that work anywhere, not just on this platform
- Any CI/CD pipeline (GitHub Actions, Drone, etc.) can build and push

### Practical Impact for Your Homelab

| Aspect | OpenFaaS | Knative (custom container) | Knative (`func` CLI) |
|---|---|---|---|
| **Write a Dockerfile?** | No (template generates it) | Yes | No (buildpacks) |
| **Write HTTP server?** | No (watchdog provides it) | Yes | No (scaffolded) |
| **Build command** | `faas-cli build` | `docker build` | `func build` |
| **Push command** | `faas-cli push` | `docker push` | `func build --push` |
| **Deploy command** | `faas-cli deploy` | `kubectl apply` | `func deploy` |
| **One-liner** | `faas-cli up` | N/A (2 steps min) | `func deploy` (builds+pushes+deploys) |
| **Registry required?** | Yes | Yes | Yes |
| **CI/CD integration** | `faas-cli` in pipeline | Standard Docker build | `func` in pipeline |
| **Lock-in** | OpenFaaS templates + watchdog | None — standard OCI | Knative `func` conventions |

### The `func` CLI Narrows the Gap

If you don't want to write Dockerfiles, `func create` + `func deploy` gives you a similar DX to `faas-cli new` + `faas-cli up`. The difference is it uses Cloud Native Buildpacks instead of OpenFaaS templates, and produces standard Knative Services.

### Recommendation for Migration

For your homelab, the pragmatic path:

1. **Start with custom containers** for your first few functions — you already know Go, writing a `main.go` with `net/http` is trivial, and you get full control
2. **Use a Makefile or Taskfile** to wrap the `docker build && docker push && kubectl apply` into a single command — you'll barely notice the difference from `faas-cli up`
3. **Try `func` CLI** for quick throwaway functions where you don't care about the Dockerfile
4. **Use a local registry** (like the k3s built-in registry or a simple Docker registry) to avoid pushing to the internet for every build

Example `Taskfile.yaml` (or Makefile) to match the `faas-cli up` DX:
```yaml
# Taskfile.yaml
version: "3"

vars:
  REGISTRY: registry.example.com

tasks:
  up:
    desc: "Build, push, and deploy (like faas-cli up)"
    cmds:
      - docker build -t {{.REGISTRY}}/{{.NAME}}:latest .
      - docker push {{.REGISTRY}}/{{.NAME}}:latest .
      - kubectl apply -f service.yaml
    vars:
      NAME:
        sh: basename $(pwd)
```

Then `task up` is your new `faas-cli up`.
