# HTTP rule demo: a header-gated webhook rule. Same rule + same message,
# but `--header` flips the outcome — showing the full HTTP path/method/header
# trigger surface in one tape.
# Render: task docs:tape TAPE=tapes/http-rule.tape
# Prerequisites: shunt binary at ./bin/shunt (`task build:shunt`).
# Fixtures: tapes/fixtures/github-webhook.{yaml,json}.

Output "docs/assets/casts/http-rule.webm"

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 720
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms
Set PlaybackSpeed 1.0

Hide
Type `export PATH="$(pwd)/bin:$PATH" SHUNT_FIXTURES="$(pwd)/tapes/fixtures"`
Enter
Type `cd "$(mktemp -d)"`
Enter
Type `cp "$SHUNT_FIXTURES/github-webhook.yaml" rule.yaml`
Enter
Type `cp "$SHUNT_FIXTURES/github-webhook.json" msg.json`
Enter
Type "clear"
Enter
Show

Type "cat rule.yaml"
Sleep 400ms
Enter
Sleep 4s

Type "cat msg.json"
Sleep 400ms
Enter
Sleep 2s

Type "clear"
Sleep 200ms
Enter
Sleep 500ms

Type "shunt check --rule rule.yaml --message msg.json"
Sleep 400ms
Enter
Sleep 4s

Type `shunt check --rule rule.yaml --message msg.json --header "X-GitHub-Event: push"`
Sleep 400ms
Enter
Sleep 6s
