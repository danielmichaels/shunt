# KV lifecycle demo: generate a rule from a template, then push / list / pull / delete it
# against a self-hosted `shunt dev` (embedded NATS, no external deps).
# Render: task docs:tape TAPE=tapes/kv-lifecycle.tape
# Prerequisites: shunt binary at ./bin/shunt (`task build:shunt`).

Output "docs/assets/casts/kv-lifecycle.webm"

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 720
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms
Set PlaybackSpeed 1.0

Hide
Type `export PATH="$(pwd)/bin:$PATH" SHUNT_NATS_URL="nats://localhost:14222"`
Enter
Type `mkdir -p /tmp/shunt-tape-empty && cd "$(mktemp -d)"`
Enter
Type `shunt dev --rules-dir /tmp/shunt-tape-empty --log-level error >/tmp/shunt-dev-tape.log 2>&1 & echo $! > /tmp/shunt-dev-tape.pid`
Enter
Type `for i in $(seq 1 50); do shunt kv list >/dev/null 2>&1 && break; sleep 0.1; done`
Enter
Type "clear"
Enter
Show

Type "shunt new -t nats-basic -o rule.yaml"
Sleep 400ms
Enter
Sleep 1500ms
Enter
Sleep 1s

Type "shunt kv push rule.yaml"
Sleep 400ms
Enter
Sleep 2s

Type "shunt kv list"
Sleep 400ms
Enter
Sleep 3s

Type "clear"
Sleep 200ms
Enter
Sleep 500ms

Type "shunt kv pull rule -f yaml | head -15"
Sleep 400ms
Enter
Sleep 4s

Type "shunt kv delete rule --force"
Sleep 400ms
Enter
Sleep 1500ms

Type "shunt kv list"
Sleep 400ms
Enter
Sleep 3s

Hide
Type `kill $(cat /tmp/shunt-dev-tape.pid) 2>/dev/null; rm -f /tmp/shunt-dev-tape.pid /tmp/shunt-dev-tape.log`
Enter
