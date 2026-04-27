# KV Lifecycle: A 5-Minute Tour

A visual walkthrough of managing rules in NATS KV — generate, push, list, pull, delete — using `shunt dev` so you don't need a separate NATS install.

For a deeper hands-on tutorial covering rule semantics, conditions, templating, and KV enrichment, see the [Local Testing Tutorial](09-local-testing-tutorial.md).

<video controls loop muted playsinline style="max-width: 100%; border-radius: 6px; margin: 1em 0;">
  <source src="/assets/casts/kv-lifecycle.webm" type="video/webm">
</video>

## Prerequisites

The `shunt` binary on your `PATH`. From the repo root:

```bash
task build:shunt
export PATH="$(pwd)/bin:$PATH"
```

## 1. Start an embedded NATS server

In one terminal:

```bash
shunt dev
```

`shunt dev` spins up an embedded NATS server on port `14222`, auto-provisions the `rules` KV bucket, and seeds a few demo rules so the engine has something to evaluate. It's intended for local exploration — not production.

In a second terminal, point the CLI at the dev server:

```bash
export SHUNT_NATS_URL="nats://localhost:14222"
```

## 2. Generate a rule from a template

```bash
shunt new -t nats-basic -o rule.yaml
```

Templates are starting points. Run `shunt new -t list` to see what's available, or `shunt new -i` for the interactive builder.

## 3. Push the rule into KV

```bash
shunt kv push rule.yaml
```

Shunt picks up the new rule immediately via KV Watch — no server restart needed. The KV key is derived from the file path: `rule.yaml` becomes the key `rule`.

## 4. List rules in KV

```bash
shunt kv list
```

You'll see `rule` alongside any rules `shunt dev` seeded on startup (e.g. `router.basic`, `http.webhooks`). The video uses `--rules-dir` pointed at an empty directory to keep the listing focused — by default you'll have a few extras.

## 5. Pull a rule back out

```bash
shunt kv pull rule -f yaml
```

Useful for inspecting what's actually stored in KV, or pulling rules into Git for version control. `-f json` is also supported.

## 6. Delete the rule

```bash
shunt kv delete rule --force
```

Shunt detects the deletion and tears down the rule's subscriptions automatically. `--force` skips the confirmation prompt.

## Next steps

- [Local Testing Tutorial](09-local-testing-tutorial.md) — graduated, hands-on lessons covering rule conditions, templating, wildcards, time-based logic, and the HTTP gateway.
- [Core Concepts](01-core-concepts.md) — the rule syntax in depth.
- [KV Enrichment](08-kv-enrichment.md) — using NATS KV to enrich messages at evaluation time.
- [Deployment](06-deployment.md) — running Shunt in production with externally managed NATS.
