package cmd

type Globals struct {
	Debug   bool   `help:"Enable debug output" env:"SHUNT_DEBUG"`
	Version string `kong:"-"`
}
