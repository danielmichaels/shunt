# Shunt

A high-performance, rule-based message router for NATS JetStream with an integrated HTTP gateway and automated token management.

Rules are stored in NATS KV and hot-reloaded via KV Watch — no restarts required.

<video autoplay loop muted playsinline style="max-width: 100%; border-radius: 6px; margin: 1em 0;">
  <source src="/assets/casts/homepage-hero.webm" type="video/webm">
</video>

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

## What's here

<div class="grid cards" markdown>

-   :material-school: **[Core Concepts](01-core-concepts.md)**

    The shared rule syntax — triggers, conditions, and actions.

-   :material-variable: **[System Variables](02-system-variables.md)**

    Built-in variables available in templating and conditions.

-   :material-format-list-numbered: **[Array Processing](03-array-processing.md)**

    Operators (`any`, `all`, `none`) and `forEach` iteration.

-   :material-package-variant: **[Primitive Messages](04-primitive-messages.md)**

    Strings, numbers, arrays, and objects at the root.

-   :material-shield-key: **[Security](05-security.md)**

    NKey signature verification and message integrity.

-   :material-rocket-launch: **[Deployment](06-deployment.md)**

    Production deployment patterns and rule management via Git.

-   :material-cog: **[Configuration](07-configuration.md)**

    All configuration options.

-   :material-database: **[KV Enrichment](08-kv-enrichment.md)**

    Enrich messages with data from NATS KV with local caching.

-   :material-test-tube: **[Local Testing Tutorial](09-local-testing-tutorial.md)**

    Step-by-step walkthrough for local development.

</div>

## Source

Shunt is open source under the [Apache 2 License](https://github.com/danielmichaels/shunt/blob/main/LICENSE) — see the [GitHub repository](https://github.com/danielmichaels/shunt).

## Inspiration

This should be considered a re-imagining of [rule-router] for my use. [rule-router] is a mature, well architected project please go star it!

[rule-router]: https://github.com/skeeeon/rule-router
