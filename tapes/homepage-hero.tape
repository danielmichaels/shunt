# Homepage hero: 25-second tour of the shunt CLI surface.
# Render: task docs:tape TAPE=tapes/homepage-hero.tape
# Prerequisites: shunt binary at ./bin/shunt (`task build:shunt`).

Output "docs/assets/casts/homepage-hero.webm"

Set Shell "bash"
Set FontSize 14
Set Width 1100
Set Height 600
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms
Set PlaybackSpeed 1.0

Hide
Type `export PATH="$(pwd)/bin:$PATH"`
Enter
Type `cd "$(mktemp -d)"`
Enter
Type "clear"
Enter
Show

Type "shunt --version"
Sleep 400ms
Enter
Sleep 1s

Type "shunt new --list"
Sleep 400ms
Enter
Sleep 2s

Type "shunt new --show nats-basic | sed -n '10,28p'"
Sleep 400ms
Enter
Sleep 4s

Type "shunt --help"
Sleep 400ms
Enter
Sleep 4s
