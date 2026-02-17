//file: cmd/rule-router/main.go

package main

import (
	"log"
	"strings"

	"github.com/danielmichaels/rule-engine/config"
	"github.com/danielmichaels/rule-engine/internal/app"
	"github.com/danielmichaels/rule-engine/internal/broker"
	"github.com/danielmichaels/rule-engine/internal/lifecycle"
	"github.com/danielmichaels/rule-engine/internal/logger"
	flag "github.com/spf13/pflag" // Use pflag aliased as flag
	"github.com/spf13/viper"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Parse configuration once - reused across reloads
	cfg, rulesPath := parseFlags()

	// Setup logger early for the lifecycle manager
	appLogger, err := logger.NewLogger(&cfg.Logging)
	if err != nil {
		return err
	}
	defer appLogger.Sync()

	// Create app factory function
	createApp := func() (lifecycle.Application, error) {
		baseApp, err := app.NewAppBuilder(cfg, rulesPath).
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

// parseFlags parses command line arguments and loads config via Viper
func parseFlags() (*config.Config, string) {
	// Define flags
	configPath := flag.String("config", "config/rule-router.yaml", "path to config file (YAML or JSON)")
	rulesPath := flag.String("rules", "rules", "path to rules directory")

	// Define override flags
	flag.String("nats-urls", "", "Comma-separated NATS server URLs to override config")
	flag.Bool("metrics-enabled", true, "Override enabling of metrics server")
	flag.String("metrics-addr", "", "Override metrics server address")
	flag.String("metrics-path", "", "Override metrics endpoint path")
	flag.String("log-level", "", "Override log level (debug, info, warn, error)")

	flag.Parse()

	// Bind flags to Viper
	v := viper.GetViper()
	v.BindPFlag("nats.urls", flag.Lookup("nats-urls"))
	v.BindPFlag("metrics.enabled", flag.Lookup("metrics-enabled"))
	v.BindPFlag("metrics.address", flag.Lookup("metrics-addr"))
	v.BindPFlag("metrics.path", flag.Lookup("metrics-path"))
	v.BindPFlag("logging.level", flag.Lookup("log-level"))

	// Special handling for comma-separated nats.urls string from flag/env
	if natsURLs := v.GetString("nats.urls"); natsURLs != "" {
		v.Set("nats.urls", strings.Split(natsURLs, ","))
	}

	// Load configuration using the new Viper-powered loader
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	return cfg, *rulesPath
}
