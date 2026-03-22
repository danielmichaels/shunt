package cmd

import (
	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/app"
	"github.com/danielmichaels/shunt/internal/broker"
	"github.com/danielmichaels/shunt/internal/buildinfo"
	"github.com/danielmichaels/shunt/internal/lifecycle"
	"github.com/danielmichaels/shunt/internal/logger"
)

type ServeCmd struct {
	Config         string   `short:"c" default:"shunt.yaml" help:"Path to config file"`
	NATSURLs       []string `name:"nats-url" help:"NATS server URLs (repeatable)" sep:","`
	LogLevel       string   `name:"log-level" help:"Log level (debug, info, warn, error)"`
	MetricsEnabled *bool    `name:"metrics-enabled" help:"Enable metrics server" negatable:""`
	MetricsAddr    string   `name:"metrics-addr" help:"Metrics server address"`
	MetricsPath    string   `name:"metrics-path" help:"Metrics endpoint path"`
	GatewayEnabled *bool    `name:"gateway-enabled" help:"Enable HTTP gateway" negatable:""`
	KVEnabled      *bool    `name:"kv-enabled" help:"Enable KV store" negatable:""`
	WorkerCount    *int     `name:"worker-count" help:"Number of consumer workers"`
}

func (s *ServeCmd) toOverrides() config.ServeOverrides {
	return config.ServeOverrides{
		NATSURLs:       s.NATSURLs,
		LogLevel:       s.LogLevel,
		MetricsEnabled: s.MetricsEnabled,
		MetricsAddr:    s.MetricsAddr,
		MetricsPath:    s.MetricsPath,
		GatewayEnabled: s.GatewayEnabled,
		KVEnabled:      s.KVEnabled,
		WorkerCount:    s.WorkerCount,
	}
}

func (s *ServeCmd) Run(globals *Globals) error {
	cfg, err := config.Load(s.Config)
	if err != nil {
		return err
	}
	cfg.ApplyOverrides(s.toOverrides())

	appLogger, err := logger.NewLogger(&cfg.Logging)
	if err != nil {
		return err
	}

	bi := buildinfo.Get(globals.Version)
	appLogger.Info("starting shunt",
		"version", bi.Version,
		"commit", bi.Commit,
		"buildTime", bi.Time,
		"modified", bi.Modified)

	createApp := func() (lifecycle.Application, error) {
		baseApp, err := app.NewAppBuilder(cfg).
			WithLogger().
			WithMetrics().
			WithNATSBroker().
			WithKVRuleProcessor().
			Build()
		if err != nil {
			return nil, err
		}

		routerApp := app.NewKVRouterApp(baseApp, cfg)

		kvManager := broker.NewRuleKVManager(
			cfg.Rules.KVBucket,
			cfg.KV.AutoProvision,
			baseApp.Processor,
			baseApp.Broker,
			baseApp.RulesLoader,
			baseApp.Logger,
		)
		routerApp.SetRuleKVManager(kvManager)

		return routerApp, nil
	}

	return lifecycle.Run(createApp, appLogger)
}
