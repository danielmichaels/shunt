# Changelog

All notable changes to this project will be documented in this file.

## [0.1.0] - 2026-03-24

### 🐛 Bug Fixes

- Use explicit git-cliff ranges to handle rc tags in changelog (#25)([4c4f8bf](https://github.com/danielmichaels/shunt/commit/4c4f8bf3ef5d64a53e297dff23c7a14a83d09e03))

### 📦 Other

- Add git cliff ci (#26)([be6eb7c](https://github.com/danielmichaels/shunt/commit/be6eb7c43dd561243171ba4e75f3ba7b2fa38962))
## [0.0.3] - 2026-03-22

### 🐛 Bug Fixes

- Logging to debug to reduce noise at start up (#23)([a68fed5](https://github.com/danielmichaels/shunt/commit/a68fed59dfcf7c4a27070eab1a29ffd03e75cef1))
- *(broker)* Fix outbound HTTP action processing and consumer creation (#22)([78d2abd](https://github.com/danielmichaels/shunt/commit/78d2abd44c432f9ebc61cb200d2f30e246c3e504))
- *(broker)* Wire OutboundClient for KV-loaded HTTP action rules (#18)([955a950](https://github.com/danielmichaels/shunt/commit/955a950baed2d3840cc52107247634ff1bc8acf6))

### 📦 Other

- Add info when kv updates (#24)([5ae4678](https://github.com/danielmichaels/shunt/commit/5ae4678f9fda6a541e190a6d4054482001801db2))
- Rm .idea (#20)([65ba21e](https://github.com/danielmichaels/shunt/commit/65ba21efb7a27f19d1f94e7f4e17e168483353cf))
- Update tags to include sortable timestamp on build([7c06d30](https://github.com/danielmichaels/shunt/commit/7c06d3057f656918e68a0b7d4bd717fea9cc1951))

### 🚀 Features

- Add version info (#21)([80bab29](https://github.com/danielmichaels/shunt/commit/80bab29288ea2ed84ec3a3ee3a6755c590c47f08))
- *(broker)* Validate stream existence on KV rule push([ce5177e](https://github.com/danielmichaels/shunt/commit/ce5177e67374c0cbc5ff18105c1e4de8f7a35ac0))
## [0.0.2] - 2026-03-09

### ⚙️ Miscellaneous

- Update deps, go version, and README([824c07e](https://github.com/danielmichaels/shunt/commit/824c07ee485ae091d328a544d7a36c8905fb41bc))

### 🐛 Bug Fixes

- *(app)* Remove dead GetAllRules outbound subscription setup([0a04ad3](https://github.com/danielmichaels/shunt/commit/0a04ad36f767e0cc6e83baafa174f0d6432d5842))
- *(broker)* Eliminate data race in Discover() summary log([79f1ebd](https://github.com/danielmichaels/shunt/commit/79f1ebdeb5a22c13cebbe1ac1860f42508d74970))
- *(gateway)* Replace static handler registration with catch-all route([851b464](https://github.com/danielmichaels/shunt/commit/851b464b506359b7ef6daa81abd37222e44d9e9d))
- *(broker)* Track watcher goroutine in RuleKVManager; push HTTP rules from KV([95bf519](https://github.com/danielmichaels/shunt/commit/95bf519de3d39fa1f1091336c07b237b7a9b5605))
- *(rule,broker)* Add ErrMalformedPayload; fix isTerminalError JSON mismatch([58e2f43](https://github.com/danielmichaels/shunt/commit/58e2f43404eadef50a0be8b9e1e76e4e58e6ac9f))
- *(rule)* Correct debouncer TOCTOU race with LoadOrStore+CAS([d20166c](https://github.com/danielmichaels/shunt/commit/d20166c7e6ac640f5f316e8726ae863123c55029))
- Add better tracing([1d12b9b](https://github.com/danielmichaels/shunt/commit/1d12b9b2ae92b0ea0f64af78c4828207705dc073))
- Nats conn guage([1874229](https://github.com/danielmichaels/shunt/commit/18742297a6a9cc1c5f62eb51493ce49802d8d04f))
- Kong args for KV commands([2119ea5](https://github.com/danielmichaels/shunt/commit/2119ea5512a706e3fbcfa113c5a024ad88210c00))
- Address critical production-readiness issues from codebase review([39a99be](https://github.com/danielmichaels/shunt/commit/39a99be21e09ec4863ab70fb02a9ab9f8c74986b))
- Derive KV keys from full file path, not just basename([8ce358c](https://github.com/danielmichaels/shunt/commit/8ce358c31eefab6af2bf6ec2381327674cd9c856))

### 📚 Documentation

- Add GitOps rule management workflow to deployment guide([e9e65f1](https://github.com/danielmichaels/shunt/commit/e9e65f159c8aa300e3b99ec1403fb26e689497b0))

### 📦 Other

- Add Apache License 2.0

Added Apache License 2.0 to the project.([9e7f398](https://github.com/danielmichaels/shunt/commit/9e7f398d77d60b971a38ec551c6f5b9d73a9b7ef))
- Update README.md([13ef284](https://github.com/danielmichaels/shunt/commit/13ef284c7fa9313d7df711dace3346bfd5b76b9b))
- Add stream refresh capability to handle dynamically created streams (#15)([ba60ec6](https://github.com/danielmichaels/shunt/commit/ba60ec67ea646cffb092c5eebe854a901485e51c))
- Add names for tracking events in VM/Grafana([cf01aac](https://github.com/danielmichaels/shunt/commit/cf01aac2d31f4c07036ab63574c1768783093be3))
- Fix goreleaser (#10)([b8e867d](https://github.com/danielmichaels/shunt/commit/b8e867d253df8945ec8e5454bdd94988c8dcd3ac))
- Allow global and trigger level core/jetstream modes (#6)([57ca3ef](https://github.com/danielmichaels/shunt/commit/57ca3ef5fe694a0b0395d045a5888b66d506f968))
- Add PR CI([d496e31](https://github.com/danielmichaels/shunt/commit/d496e31a294241d4dd5823cf75cc2ff14d24bfcc))
- Migrate to Kong from Cobra/Viper([5a59531](https://github.com/danielmichaels/shunt/commit/5a59531272fd74cc51f3cc4bcfed24d524a3160c))
- Fix missing config options([8c44b66](https://github.com/danielmichaels/shunt/commit/8c44b66674f31e19d99336eca977b20b13287b6a))
- Clean up the docs and config options([7dfa0fa](https://github.com/danielmichaels/shunt/commit/7dfa0fadcfd791392572018896c5d9da8e4fa4f4))
- Update deployment docs([858d970](https://github.com/danielmichaels/shunt/commit/858d970239a5b1b814e8b592b799458af77a78bb))
- Clean up file comments, add local testing tutorial([d329f11](https://github.com/danielmichaels/shunt/commit/d329f11a48c6788bf2c040df82e7e755ff3fff58))
- Clean up file comments, dev task env, and add format helper

Remove stale // file: comments from cmd files, add logging env vars to
dev task, strip redundant emoji comments from basic.yaml, and add shared
format.go output helper for CLI commands.([446c0af](https://github.com/danielmichaels/shunt/commit/446c0af1260327a4c0c7476f4a6015e7f6f78dcb))
- Add TTY-aware spinner to KV commands and improve missing stream warnings

Add briandowns/spinner to the 4 KV CLI commands (push, pull, list, delete)
so users get visual feedback during NATS connection. Spinner is suppressed
in non-TTY environments. Also improve rule_kv_manager logging to emit a
clear warning with actionable hint when a rule references a subject with
no backing JetStream stream, instead of dumping the full error chain.([b034895](https://github.com/danielmichaels/shunt/commit/b034895f4ac5ef4c56d9eecca8bf8cc5c8377ec8))
- Remove license update go.mod([0571b6b](https://github.com/danielmichaels/shunt/commit/0571b6b70a483fde6a404c7a09035a03fb1b4cb3))

### 🚀 Features

- Add a debouncer to trigger([3c4dbb9](https://github.com/danielmichaels/shunt/commit/3c4dbb9372be74acf00dc0e0147f3afaa9d34ef8))
- Add goreleaser support (no homebrew)([e0dfd93](https://github.com/danielmichaels/shunt/commit/e0dfd93bd4607c828faf3b7e21f8a19c1f62a5cc))
- Rebrand rule-engine to shunt with single unified binary([e8e158b](https://github.com/danielmichaels/shunt/commit/e8e158b4d95be8c1ba113007ac69a2ac659569d2))
- Consolidate 4 binaries into 2 (rule-router + rule-cli)([739bd54](https://github.com/danielmichaels/shunt/commit/739bd540b5f143d216d0b94730ae9646d018049a))
- Replace file-based rule loading with NATS KV Watch([93cc8cc](https://github.com/danielmichaels/shunt/commit/93cc8cc59d944b68f97405c0565f3d15f1bc9213))

### 🚜 Refactor

- *(rule)* Replace httpPathIndex with atomic.Value for KV HTTP routing([344f3ca](https://github.com/danielmichaels/shunt/commit/344f3cad9230fc1dff4e791649193e9c707cd8d4))
- Slog middlewares and response writers([6613a20](https://github.com/danielmichaels/shunt/commit/6613a200a79516d4fa34301e7c0bef1604d48706))
- Replace zap logger with stdlib slog([9d16e6b](https://github.com/danielmichaels/shunt/commit/9d16e6b91880faf2de5b6ff57d6b9d18b113f361))
- Unify CLI flags and env var handling under SHUNT_ prefix([200c219](https://github.com/danielmichaels/shunt/commit/200c21971cc15bfac33fc2a9fc318b10fec18e53))

### 🧪 Testing

- *(broker)* Add embedded-NATS integration tests for KV HTTP routing([5d9f068](https://github.com/danielmichaels/shunt/commit/5d9f068a3b6cfe7084a03c6b9c326ab2a0b79e50))

