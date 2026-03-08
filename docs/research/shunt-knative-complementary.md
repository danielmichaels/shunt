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
