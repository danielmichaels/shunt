# Rule eval demo: a rule, an input message, and `shunt check` showing match + rendered action.
# Render: task docs:tape TAPE=tapes/rule-check.tape
# Prerequisites: shunt binary at ./bin/shunt (`task build:shunt`).
# Fixtures: tapes/fixtures/temp-alert.{yaml,json}.

Output "docs/assets/casts/rule-check.webm"

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms
Set PlaybackSpeed 1.0

Hide
Type `export PATH="$(pwd)/bin:$PATH" SHUNT_FIXTURES="$(pwd)/tapes/fixtures"`
Enter
Type `cd "$(mktemp -d)"`
Enter
Type `cp "$SHUNT_FIXTURES/temp-alert.yaml" rule.yaml`
Enter
Type `cp "$SHUNT_FIXTURES/temp-alert.json" msg.json`
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

Type "shunt check --rule rule.yaml --message msg.json --subject sensors.temperature.kitchen"
Sleep 400ms
Enter
Sleep 5s
